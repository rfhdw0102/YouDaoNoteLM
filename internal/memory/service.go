package memory

import (
	"context"
	"sort"
)

// Reader is the only dependency Chat, Generation, and a future
// ContextManager need for long-term memory reads.
type Reader interface {
	LoadSnapshot(ctx context.Context, userID uint) (Snapshot, error)
}

// Service owns V1 validation, CRUD, and structured snapshot construction.
type Service interface {
	Reader
	List(ctx context.Context, userID uint) ([]Preference, error)
	Upsert(ctx context.Context, userID uint, typ Type, content string) (Preference, error)
	Delete(ctx context.Context, userID uint, typ Type) error
}

type service struct {
	store Store
}

func NewService(store Store) Service {
	return &service{store: store}
}

func (s *service) List(ctx context.Context, userID uint) ([]Preference, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	items, err := s.store.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return preferencesFromItems(items), nil
}

func (s *service) Upsert(ctx context.Context, userID uint, typ Type, content string) (Preference, error) {
	if err := validateUserID(userID); err != nil {
		return Preference{}, err
	}
	if err := validateType(typ); err != nil {
		return Preference{}, err
	}
	normalized, err := validatePreferenceContent(typ, content)
	if err != nil {
		return Preference{}, err
	}
	item, err := s.store.Upsert(ctx, UserMemory{
		UserID:     userID,
		MemoryType: string(typ),
		Content:    normalized,
	})
	if err != nil {
		return Preference{}, err
	}
	return preferenceFromItem(item), nil
}

func (s *service) Delete(ctx context.Context, userID uint, typ Type) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if err := validateType(typ); err != nil {
		return err
	}
	return s.store.DeleteByUserIDAndType(ctx, userID, typ)
}

func (s *service) LoadSnapshot(ctx context.Context, userID uint) (Snapshot, error) {
	preferences, err := s.List(ctx, userID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Preferences: preferences}, nil
}

func preferencesFromItems(items []UserMemory) []Preference {
	preferences := make([]Preference, 0, len(items))
	for _, item := range items {
		typ := Type(item.MemoryType)
		if !typ.IsValid() {
			continue
		}
		preferences = append(preferences, preferenceFromItem(item))
	}
	sort.SliceStable(preferences, func(i, j int) bool {
		return typeOrder(preferences[i].Type) < typeOrder(preferences[j].Type)
	})
	return preferences
}

func preferenceFromItem(item UserMemory) Preference {
	return Preference{
		Type:      Type(item.MemoryType),
		Content:   item.Content,
		UpdatedAt: item.UpdatedAt,
	}
}

func typeOrder(typ Type) int {
	for i, candidate := range orderedTypes {
		if candidate == typ {
			return i
		}
	}
	return len(orderedTypes)
}
