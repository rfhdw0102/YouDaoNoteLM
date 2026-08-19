package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"YoudaoNoteLm/internal/memory"
)

type chatMemoryReader struct {
	snapshot memory.Snapshot
	err      error
}

func (r chatMemoryReader) LoadSnapshot(context.Context, uint) (memory.Snapshot, error) {
	return r.snapshot, r.err
}

func TestChatLongTermMemoryPromptDegradesOnReadFailure(t *testing.T) {
	service := &chatAgentService{longTermMemory: chatMemoryReader{err: errors.New("database unavailable")}}
	if prompt := service.longTermMemoryPrompt(context.Background(), 1); prompt != "" {
		t.Fatalf("expected empty prompt after read failure, got %q", prompt)
	}
}

func TestChatLongTermMemoryPromptRendersSnapshot(t *testing.T) {
	service := &chatAgentService{longTermMemory: chatMemoryReader{snapshot: memory.Snapshot{Preferences: []memory.Preference{{
		Type: memory.TypeLanguage, Content: "中文",
	}}}}}
	if prompt := service.longTermMemoryPrompt(context.Background(), 1); !strings.Contains(prompt, `"label":"默认语言","value":"中文"`) {
		t.Fatalf("expected rendered prompt, got %q", prompt)
	}
}
