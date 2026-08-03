package memory

import "context"

// Store is the memory module's persistence port. It intentionally has no
// knowledge of HTTP, prompt rendering, agents, cache, or vector storage.
type Store interface {
	ListByUserID(ctx context.Context, userID uint) ([]UserMemory, error)
	Upsert(ctx context.Context, item UserMemory) (UserMemory, error)
	DeleteByUserIDAndType(ctx context.Context, userID uint, typ Type) error
}
