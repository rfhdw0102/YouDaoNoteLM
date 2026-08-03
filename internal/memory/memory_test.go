package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	items []UserMemory
	err   error
}

func (s *fakeStore) ListByUserID(_ context.Context, userID uint) ([]UserMemory, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make([]UserMemory, 0)
	for _, item := range s.items {
		if item.UserID == userID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *fakeStore) Upsert(_ context.Context, item UserMemory) (UserMemory, error) {
	if s.err != nil {
		return UserMemory{}, s.err
	}
	for i := range s.items {
		if s.items[i].UserID == item.UserID && s.items[i].MemoryType == item.MemoryType {
			s.items[i].Content = item.Content
			s.items[i].UpdatedAt = time.Now()
			return s.items[i], nil
		}
	}
	item.UpdatedAt = time.Now()
	s.items = append(s.items, item)
	return item, nil
}

func (s *fakeStore) DeleteByUserIDAndType(_ context.Context, userID uint, typ Type) error {
	if s.err != nil {
		return s.err
	}
	kept := s.items[:0]
	for _, item := range s.items {
		if item.UserID != userID || item.MemoryType != string(typ) {
			kept = append(kept, item)
		}
	}
	s.items = kept
	return nil
}

func TestServiceUpsertNormalizesAndOverwrites(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)

	first, err := svc.Upsert(context.Background(), 1, TypeOutputFormat, "  使用\n Markdown   表格  ")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if first.Content != "使用 Markdown 表格" {
		t.Fatalf("unexpected normalized content: %q", first.Content)
	}
	second, err := svc.Upsert(context.Background(), 1, TypeOutputFormat, "使用 JSON")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if second.Content != "使用 JSON" || len(store.items) != 1 {
		t.Fatalf("upsert must replace a slot, got %#v", store.items)
	}
}

func TestServiceRejectsInvalidInput(t *testing.T) {
	svc := NewService(&fakeStore{})
	for _, tc := range []struct {
		name string
		typ  Type
		body string
		err  error
	}{
		{name: "unknown type", typ: Type("unknown"), body: "x", err: ErrInvalidType},
		{name: "empty", typ: TypeLanguage, body: " \t\n", err: ErrEmptyContent},
		{name: "unsupported language", typ: TypeLanguage, body: "日本語", err: ErrInvalidLanguage},
		{name: "too long", typ: TypeOutputFormat, body: strings.Repeat("长", MaxContentRunes+1), err: ErrContentTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Upsert(context.Background(), 1, tc.typ, tc.body)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
		})
	}
}

func TestServiceHandlesTenBoundaryInputs(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	for _, tc := range []struct {
		name        string
		userID      uint
		typ         Type
		content     string
		wantErr     error
		wantContent string
	}{
		{name: "zero user", userID: 0, typ: TypeLanguage, content: "English", wantErr: ErrInvalidUser},
		{name: "unknown type", userID: 1, typ: Type("profile"), content: "English", wantErr: ErrInvalidType},
		{name: "empty string", userID: 1, typ: TypeLanguage, content: "", wantErr: ErrEmptyContent},
		{name: "whitespace only", userID: 1, typ: TypeLanguage, content: " \t\n", wantErr: ErrEmptyContent},
		{name: "Chinese language", userID: 1, typ: TypeLanguage, content: "中文", wantContent: "中文"},
		{name: "English language", userID: 1, typ: TypeLanguage, content: "English", wantContent: "English"},
		{name: "language normalized", userID: 1, typ: TypeLanguage, content: "\u00a0English\u00a0", wantContent: "English"},
		{name: "unsupported language", userID: 1, typ: TypeLanguage, content: "日本語", wantErr: ErrInvalidLanguage},
		{name: "max ascii", userID: 1, typ: TypeCustomInstruction, content: strings.Repeat("a", MaxContentRunes), wantContent: strings.Repeat("a", MaxContentRunes)},
		{name: "too long unicode", userID: 1, typ: TypeCustomInstruction, content: strings.Repeat("英", MaxContentRunes+1), wantErr: ErrContentTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preference, err := svc.Upsert(context.Background(), tc.userID, tc.typ, tc.content)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr == nil && preference.Content != tc.wantContent {
				t.Fatalf("expected content %q, got %q", tc.wantContent, preference.Content)
			}
		})
	}
}

func TestLoadSnapshotFiltersOtherUsersAndOrdersPreferences(t *testing.T) {
	store := &fakeStore{items: []UserMemory{
		{UserID: 1, MemoryType: string(TypeOutputFormat), Content: "表格"},
		{UserID: 2, MemoryType: string(TypeLanguage), Content: "English"},
		{UserID: 1, MemoryType: string(TypeLanguage), Content: "中文"},
	}}
	snapshot, err := NewService(store).LoadSnapshot(context.Background(), 1)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(snapshot.Preferences) != 2 || snapshot.Preferences[0].Type != TypeLanguage || snapshot.Preferences[1].Type != TypeOutputFormat {
		t.Fatalf("unexpected snapshot: %#v", snapshot.Preferences)
	}
}

func TestSnapshotRenderPromptUsesStableSafeFormat(t *testing.T) {
	prompt := (Snapshot{Preferences: []Preference{
		{Type: TypeOutputFormat, Content: "表格"},
		{Type: TypeLanguage, Content: "中文"},
	}}).RenderPrompt()
	if !strings.Contains(prompt, longTermMemoryHeading) || !strings.Contains(prompt, `"label":"默认语言","value":"中文"`) || !strings.Contains(prompt, `"label":"输出格式","value":"表格"`) {
		t.Fatalf("missing rendered preferences: %q", prompt)
	}
	if strings.Index(prompt, "默认语言") > strings.Index(prompt, "输出格式") {
		t.Fatalf("preferences were not rendered in stable order: %q", prompt)
	}
	if !strings.Contains(prompt, "当前请求明确") {
		t.Fatalf("missing priority guard: %q", prompt)
	}
	if !strings.Contains(prompt, "应用已校验的默认回答语言：中文") || !strings.Contains(prompt, "不得从提问文本的语言推断回答语言") {
		t.Fatalf("missing default language rule: %q", prompt)
	}
}
