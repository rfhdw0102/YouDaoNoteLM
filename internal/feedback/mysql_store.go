package feedback

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mysqlStore struct {
	db *gorm.DB
}

// NewMySQLStore creates a feedback store backed by MySQL.
func NewMySQLStore(db *gorm.DB) Store {
	return &mysqlStore{db: db}
}

// NewMySQLAdminStore creates a store that implements both Store and AdminReader.
func NewMySQLAdminStore(db *gorm.DB) (Store, AdminReader) {
	s := &mysqlStore{db: db}
	return s, s
}

// Upsert creates or replaces the current user's feedback for a message.
// Ownership verification, row locking, and upsert all happen within a single
// short transaction. No LLM, HTTP, Redis, or external calls are made inside.
func (s *mysqlStore) Upsert(ctx context.Context, userID, messageID uint, rating Rating, reason ReasonCode) (Feedback, error) {
	var result Feedback

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Verify message is an undeleted assistant message in an undeleted
		//    conversation owned by the user. Lock the row with FOR UPDATE.
		var msg struct {
			ID uint
		}
		err := tx.Table("messages").
			Select("messages.id").
			Joins("JOIN conversations ON conversations.id = messages.conversation_id").
			Where(`messages.id = ?
				AND messages.role = 'assistant'
				AND messages.deleted_at IS NULL
				AND conversations.user_id = ?
				AND conversations.deleted_at IS NULL`, messageID, userID).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Take(&msg).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAnswerNotFound
		}
		if err != nil {
			return err
		}

		// 2. Atomic upsert on the unique business key (user_id, message_id).
		item := AnswerFeedback{
			UserID:     userID,
			MessageID:  messageID,
			Rating:     string(rating),
			ReasonCode: string(reason),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "message_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"rating", "reason_code", "updated_at"}),
		}).Create(&item).Error; err != nil {
			return err
		}

		// 3. Read back the current row to return the server-side state.
		var row AnswerFeedback
		if err := tx.Where("user_id = ? AND message_id = ?", userID, messageID).
			First(&row).Error; err != nil {
			return err
		}

		result = Feedback{
			Rating:    Rating(row.Rating),
			Reason:    ReasonCode(row.ReasonCode),
			UpdatedAt: row.UpdatedAt,
		}
		return nil
	})

	if err != nil {
		return Feedback{}, err
	}
	return result, nil
}

// Delete removes the current user's feedback for a message.
// Ownership verification happens within a single transaction.
// Deleting feedback for a valid target with no existing feedback is idempotent.
func (s *mysqlStore) Delete(ctx context.Context, userID, messageID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Same ownership verification and locking as Upsert.
		var msg struct {
			ID uint
		}
		err := tx.Table("messages").
			Select("messages.id").
			Joins("JOIN conversations ON conversations.id = messages.conversation_id").
			Where(`messages.id = ?
				AND messages.role = 'assistant'
				AND messages.deleted_at IS NULL
				AND conversations.user_id = ?
				AND conversations.deleted_at IS NULL`, messageID, userID).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Take(&msg).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAnswerNotFound
		}
		if err != nil {
			return err
		}

		// 2. Delete by business key. Zero rows affected is success (idempotent).
		result := tx.Where("user_id = ? AND message_id = ?", userID, messageID).
			Delete(&AnswerFeedback{})
		return result.Error
	})
}

// ListByMessageIDs returns a map of message_id -> Feedback for the given user,
// only for messages that are active assistant messages in active conversations
// owned by the user. An empty messageIDs list returns an empty map without querying.
func (s *mysqlStore) ListByMessageIDs(ctx context.Context, userID uint, messageIDs []uint) (map[uint]Feedback, error) {
	if len(messageIDs) == 0 {
		return map[uint]Feedback{}, nil
	}

	var rows []AnswerFeedback
	err := s.db.WithContext(ctx).
		Joins("JOIN messages ON messages.id = answer_feedbacks.message_id").
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where(`answer_feedbacks.user_id = ?
			AND answer_feedbacks.message_id IN (?)
			AND messages.role = 'assistant'
			AND messages.deleted_at IS NULL
			AND conversations.deleted_at IS NULL`, userID, messageIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]Feedback, len(rows))
	for _, row := range rows {
		result[row.MessageID] = Feedback{
			Rating:    Rating(row.Rating),
			Reason:    ReasonCode(row.ReasonCode),
			UpdatedAt: row.UpdatedAt,
		}
	}
	return result, nil
}

// --- AdminReader implementation ---

// baseAdminQuery builds a query that joins answer_feedbacks with active
// messages and conversations, applying the common time and optional
// rating/reason filters. No user IDs, message IDs, or content are exposed.
func (s *mysqlStore) baseAdminQuery(filter AdminFilter) *gorm.DB {
	q := s.db.Table("answer_feedbacks af").
		Joins("JOIN messages m ON m.id = af.message_id").
		Joins("JOIN conversations c ON c.id = m.conversation_id").
		Where("m.role = 'assistant'").
		Where("m.deleted_at IS NULL").
		Where("c.deleted_at IS NULL").
		Where("af.created_at >= ?", filter.From).
		Where("af.created_at < ?", filter.To)

	if filter.Rating != nil {
		q = q.Where("af.rating = ?", string(*filter.Rating))
	}
	if filter.Reason != nil {
		q = q.Where("af.reason_code = ?", string(*filter.Reason))
	}
	return q
}

// Overview returns aggregate statistics for the admin dashboard.
func (s *mysqlStore) Overview(ctx context.Context, filter AdminFilter) (FeedbackOverview, error) {
	q := s.baseAdminQuery(filter).WithContext(ctx)

	// Total count
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return FeedbackOverview{}, err
	}

	// Up count
	var upCount int64
	if err := q.Where("af.rating = ?", string(RatingUp)).Count(&upCount).Error; err != nil {
		return FeedbackOverview{}, err
	}

	downCount := total - upCount
	var positiveRatio float64
	if total > 0 {
		positiveRatio = float64(upCount) / float64(total)
	}

	// Reason distribution
	type reasonRow struct {
		ReasonCode string
		Count      int64
	}
	var reasonRows []reasonRow
	if err := s.baseAdminQuery(filter).WithContext(ctx).
		Select("af.reason_code, COUNT(*) as count").
		Group("af.reason_code").
		Find(&reasonRows).Error; err != nil {
		return FeedbackOverview{}, err
	}

	reasonCounts := make(map[ReasonCode]int64, len(reasonRows))
	for _, r := range reasonRows {
		reasonCounts[ReasonCode(r.ReasonCode)] = r.Count
	}

	return FeedbackOverview{
		TotalCount:    total,
		UpCount:       upCount,
		DownCount:     downCount,
		PositiveRatio: positiveRatio,
		ReasonCounts:  reasonCounts,
	}, nil
}

// ListForAdmin returns a paginated list of desensitized feedback items.
func (s *mysqlStore) ListForAdmin(ctx context.Context, filter AdminFilter, page, size int) ([]AdminFeedbackItem, int64, error) {
	q := s.baseAdminQuery(filter).WithContext(ctx)

	// Count
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Paginated query — only select safe fields
	var rows []struct {
		CreatedAt time.Time
		Rating    string
		ReasonCode string
	}
	offset := (page - 1) * size
	if err := q.Select("af.created_at, af.rating, af.reason_code").
		Order("af.created_at DESC").
		Offset(offset).
		Limit(size).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]AdminFeedbackItem, len(rows))
	for i, r := range rows {
		items[i] = AdminFeedbackItem{
			CreatedAt: r.CreatedAt,
			Rating:    Rating(r.Rating),
			Reason:    ReasonCode(r.ReasonCode),
		}
	}
	return items, total, nil
}

// WriteCSV writes filtered feedback as UTF-8 CSV with BOM. Returns the number
// of data rows written. Exceeding 10,000 rows returns an error.
func (s *mysqlStore) WriteCSV(ctx context.Context, filter AdminFilter, w io.Writer) (int, error) {
	const maxRows = 10000

	// Check count first
	q := s.baseAdminQuery(filter).WithContext(ctx)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, err
	}
	if total > maxRows {
		return 0, fmt.Errorf("导出行数 %d 超过上限 %d，请缩小时间范围", total, maxRows)
	}

	// Fetch all rows
	var rows []struct {
		CreatedAt time.Time
		Rating    string
		ReasonCode string
	}
	if err := q.Select("af.created_at, af.rating, af.reason_code").
		Order("af.created_at DESC").
		Find(&rows).Error; err != nil {
		return 0, err
	}

	// Write UTF-8 BOM
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return 0, err
	}

	cw := csv.NewWriter(w)
	// Header
	if err := cw.Write([]string{"提交时间", "评价", "原因"}); err != nil {
		return 0, err
	}

	ratingLabels := map[string]string{
		string(RatingUp):   "点赞",
		string(RatingDown): "点踩",
	}

	for _, r := range rows {
		label := r.Rating
		if l, ok := ratingLabels[r.Rating]; ok {
			label = l
		}
		if err := cw.Write([]string{
			r.CreatedAt.UTC().Format(time.RFC3339),
			label,
			r.ReasonCode,
		}); err != nil {
			return 0, err
		}
	}
	cw.Flush()
	return len(rows), cw.Error()
}
