package feedback

import (
	"context"
	"testing"
	"time"
)

// mockStore implements Store for unit tests.
type mockStore struct {
	upsertFn         func(ctx context.Context, userID, messageID uint, rating Rating, reason ReasonCode) (Feedback, error)
	deleteFn         func(ctx context.Context, userID, messageID uint) error
	listByMessageIDs func(ctx context.Context, userID uint, messageIDs []uint) (map[uint]Feedback, error)
}

func (m *mockStore) Upsert(ctx context.Context, userID, messageID uint, rating Rating, reason ReasonCode) (Feedback, error) {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, userID, messageID, rating, reason)
	}
	return Feedback{Rating: rating, Reason: reason, UpdatedAt: time.Now()}, nil
}

func (m *mockStore) Delete(ctx context.Context, userID, messageID uint) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, userID, messageID)
	}
	return nil
}

func (m *mockStore) ListByMessageIDs(ctx context.Context, userID uint, messageIDs []uint) (map[uint]Feedback, error) {
	if m.listByMessageIDs != nil {
		return m.listByMessageIDs(ctx, userID, messageIDs)
	}
	return map[uint]Feedback{}, nil
}

func TestServiceUpsert_Validation(t *testing.T) {
	svc := NewService(&mockStore{})
	ctx := context.Background()

	tests := []struct {
		name      string
		userID    uint
		messageID uint
		rating    Rating
		reason    ReasonCode
		wantErr   error
	}{
		{"zero userID", 0, 1, RatingUp, ReasonHelpful, ErrInvalidUser},
		{"zero messageID", 1, 0, RatingUp, ReasonHelpful, ErrInvalidMessage},
		{"invalid rating", 1, 1, "bad", ReasonHelpful, ErrInvalidRating},
		{"invalid reason", 1, 1, RatingUp, "bad", ErrInvalidReason},
		{"mismatch up+down_reason", 1, 1, RatingUp, ReasonNotHelpful, ErrReasonMismatch},
		{"mismatch down+up_reason", 1, 1, RatingDown, ReasonHelpful, ErrReasonMismatch},
		{"valid up", 1, 1, RatingUp, ReasonHelpful, nil},
		{"valid down", 1, 1, RatingDown, ReasonNotHelpful, nil},
		{"valid other up", 1, 1, RatingUp, ReasonOther, nil},
		{"valid other down", 1, 1, RatingDown, ReasonOther, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Upsert(ctx, tt.userID, tt.messageID, tt.rating, tt.reason)
			if err != tt.wantErr {
				t.Errorf("Upsert() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceDelete_Validation(t *testing.T) {
	svc := NewService(&mockStore{})
	ctx := context.Background()

	if err := svc.Delete(ctx, 0, 1); err != ErrInvalidUser {
		t.Errorf("Delete(0, 1) = %v, want %v", err, ErrInvalidUser)
	}
	if err := svc.Delete(ctx, 1, 0); err != ErrInvalidMessage {
		t.Errorf("Delete(1, 0) = %v, want %v", err, ErrInvalidMessage)
	}
	if err := svc.Delete(ctx, 1, 1); err != nil {
		t.Errorf("Delete(1, 1) = %v, want nil", err)
	}
}

func TestServiceListByMessageIDs_EmptyIDs(t *testing.T) {
	svc := NewService(&mockStore{})
	ctx := context.Background()

	result, err := svc.ListByMessageIDs(ctx, 1, nil)
	if err != nil {
		t.Fatalf("ListByMessageIDs() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("ListByMessageIDs() returned %d items, want 0", len(result))
	}
}

func TestServiceListByMessageIDs_ZeroUser(t *testing.T) {
	svc := NewService(&mockStore{})
	ctx := context.Background()

	_, err := svc.ListByMessageIDs(ctx, 0, []uint{1})
	if err != ErrInvalidUser {
		t.Errorf("ListByMessageIDs(0, ...) = %v, want %v", err, ErrInvalidUser)
	}
}
