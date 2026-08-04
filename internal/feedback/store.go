package feedback

import "context"

// Store is the feedback module's persistence port. It intentionally has no
// knowledge of HTTP, agents, cache, or external services. All write operations
// perform ownership verification and row locking within a single transaction.
type Store interface {
	// Upsert creates or replaces the current user's feedback for a message.
	// It verifies that the message is an undeleted assistant message belonging
	// to an undeleted conversation owned by the user, all within a single
	// transaction with FOR UPDATE locking.
	Upsert(ctx context.Context, userID, messageID uint, rating Rating, reason ReasonCode) (Feedback, error)

	// Delete removes the current user's feedback for a message.
	// It performs the same ownership verification as Upsert.
	// Deleting feedback for a valid target that has no existing feedback
	// returns success (idempotent).
	Delete(ctx context.Context, userID, messageID uint) error

	// ListByMessageIDs returns a map of message_id -> Feedback for the given
	// user, only including messages that are active assistant messages in
	// active conversations owned by the user. An empty messageIDs list
	// returns an empty map without querying.
	ListByMessageIDs(ctx context.Context, userID uint, messageIDs []uint) (map[uint]Feedback, error)
}
