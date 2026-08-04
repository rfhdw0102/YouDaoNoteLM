package feedback

import (
	"context"
	"io"
)

// Reader is the read-only dependency that ConversationService and other
// consumers use to fetch feedback for messages they already own.
type Reader interface {
	ListByMessageIDs(ctx context.Context, userID uint, messageIDs []uint) (map[uint]Feedback, error)
}

// AdminReader provides read-only access to aggregated feedback data for the
// admin dashboard. It never exposes user IDs, message IDs, or content.
type AdminReader interface {
	Overview(ctx context.Context, filter AdminFilter) (FeedbackOverview, error)
	ListForAdmin(ctx context.Context, filter AdminFilter, page, size int) ([]AdminFeedbackItem, int64, error)
	WriteCSV(ctx context.Context, filter AdminFilter, w io.Writer) (rows int, err error)
}

// Service owns validation, CRUD, and the full feedback lifecycle.
type Service interface {
	Reader
	Upsert(ctx context.Context, userID, messageID uint, rating Rating, reason ReasonCode) (Feedback, error)
	Delete(ctx context.Context, userID, messageID uint) error
}

type service struct {
	store Store
}

// NewService creates a feedback service backed by the given store.
func NewService(store Store) Service {
	return &service{store: store}
}

func (s *service) Upsert(ctx context.Context, userID, messageID uint, rating Rating, reason ReasonCode) (Feedback, error) {
	if userID == 0 {
		return Feedback{}, ErrInvalidUser
	}
	if messageID == 0 {
		return Feedback{}, ErrInvalidMessage
	}
	if err := ValidateRatingReasonCombination(rating, reason); err != nil {
		return Feedback{}, err
	}
	return s.store.Upsert(ctx, userID, messageID, rating, reason)
}

func (s *service) Delete(ctx context.Context, userID, messageID uint) error {
	if userID == 0 {
		return ErrInvalidUser
	}
	if messageID == 0 {
		return ErrInvalidMessage
	}
	return s.store.Delete(ctx, userID, messageID)
}

func (s *service) ListByMessageIDs(ctx context.Context, userID uint, messageIDs []uint) (map[uint]Feedback, error) {
	if userID == 0 {
		return nil, ErrInvalidUser
	}
	if len(messageIDs) == 0 {
		return map[uint]Feedback{}, nil
	}
	return s.store.ListByMessageIDs(ctx, userID, messageIDs)
}
