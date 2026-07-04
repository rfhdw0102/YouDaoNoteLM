# Generation Memory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add best-effort Redis-backed recent memory to the generation Agent, scoped by user, notebook, and generation type.

**Architecture:** Add a small `GenerationMemoryStore` interface in `internal/service`, backed by a Redis list implementation in `pkg/cache`. `generationService.Generate` reads memory before agent execution, appends it to the prompt context as non-authoritative continuity context, then writes a compact entry after successful generation.

**Tech Stack:** Go, `github.com/redis/go-redis/v9`, existing `pkg/cache`, existing generation service tests with fake models/stores.

---

## File Structure

- Create `internal/service/generation_memory.go`: service-level memory structs, optional store interface, context rendering, memory entry construction helpers.
- Create `internal/service/generation_memory_test.go`: unit tests for memory context rendering and entry summarization.
- Create `internal/service/generation_memory_service_test.go`: service tests using fake model and fake memory store.
- Create `pkg/cache/generation_memory.go`: Redis-backed memory list with key format, JSON entries, list trimming, and TTL.
- Create `pkg/cache/generation_memory_test.go`: pure unit tests for key construction and nil-client behavior.
- Modify `internal/service/generation_service.go`: add optional memory dependency, constructor overload, memory read/write in `Generate`, and meta fields.
- Modify `internal/service/generation_user_llm_config.go`: pass the optional memory store through user-LLM wrapper.
- Modify `internal/app/app.go`: create Redis memory cache when Redis is available and inject it into generation service.

---

### Task 1: Memory Types And Context Rendering

**Files:**
- Create: `internal/service/generation_memory.go`
- Test: `internal/service/generation_memory_test.go`

- [ ] **Step 1: Write failing tests for memory context rendering**

Add `internal/service/generation_memory_test.go`:

```go
package service

import (
	"strings"
	"testing"
	"time"
)

func TestBuildGenerationMemoryContextRendersRecentEntries(t *testing.T) {
	entries := []GenerationMemoryEntry{
		{
			Prompt:        "做成适合复习的 PPT",
			InputSummary:  "细胞呼吸的定义和阶段",
			OutputSummary: "上次输出了背景、过程和易错点",
			CreatedAt:     time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC),
		},
	}

	got := buildGenerationMemoryContext(entries)

	for _, want := range []string{
		"历史生成记忆",
		"偏好和连续性上下文",
		"做成适合复习的 PPT",
		"细胞呼吸的定义和阶段",
		"上次输出了背景、过程和易错点",
		"2026-06-15T10:30:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("memory context missing %q: %s", want, got)
		}
	}
}

func TestBuildGenerationMemoryContextSkipsBlankEntries(t *testing.T) {
	got := buildGenerationMemoryContext([]GenerationMemoryEntry{
		{Prompt: "   ", InputSummary: "\n", OutputSummary: "\t"},
	})
	if strings.TrimSpace(got) != "" {
		t.Fatalf("memory context = %q, want empty", got)
	}
}

func TestBuildGenerationMemoryEntrySummarizesInputAndOutput(t *testing.T) {
	req := &GenerationRequest{
		Prompt:   "整理为学习笔记",
		Markdown: "# 很长的标题\n\n" + strings.Repeat("内容", 120),
		Type:     GenerationTypeNote,
	}

	entry := buildGenerationMemoryEntry(req, "生成结果"+strings.Repeat("摘要", 120))

	if entry.Prompt != "整理为学习笔记" {
		t.Fatalf("Prompt = %q", entry.Prompt)
	}
	if entry.InputSummary == "" || len([]rune(entry.InputSummary)) > generationMemorySummaryLimit {
		t.Fatalf("InputSummary length = %d, value = %q", len([]rune(entry.InputSummary)), entry.InputSummary)
	}
	if entry.OutputSummary == "" || len([]rune(entry.OutputSummary)) > generationMemorySummaryLimit {
		t.Fatalf("OutputSummary length = %d, value = %q", len([]rune(entry.OutputSummary)), entry.OutputSummary)
	}
	if entry.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/service -run "TestBuildGenerationMemory" -count=1
```

Expected: FAIL because `GenerationMemoryEntry`, `buildGenerationMemoryContext`, and `buildGenerationMemoryEntry` do not exist.

- [ ] **Step 3: Implement memory types and helpers**

Create `internal/service/generation_memory.go`:

```go
package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	generationMemoryDefaultLimit = 10
	generationMemorySummaryLimit = 240
)

type GenerationMemoryScope struct {
	UserID     uint
	NotebookID uint
	Type       GenerationType
}

type GenerationMemoryEntry struct {
	Prompt        string    `json:"prompt"`
	InputSummary  string    `json:"input_summary"`
	OutputSummary string    `json:"output_summary"`
	CreatedAt     time.Time `json:"created_at"`
}

type GenerationMemoryStore interface {
	GetRecent(ctx context.Context, scope GenerationMemoryScope, limit int) ([]GenerationMemoryEntry, error)
	Add(ctx context.Context, scope GenerationMemoryScope, entry GenerationMemoryEntry) error
}

func buildGenerationMemoryContext(entries []GenerationMemoryEntry) string {
	var b strings.Builder
	written := 0
	for _, entry := range entries {
		prompt := strings.TrimSpace(entry.Prompt)
		input := strings.TrimSpace(entry.InputSummary)
		output := strings.TrimSpace(entry.OutputSummary)
		if prompt == "" && input == "" && output == "" {
			continue
		}
		if written == 0 {
			b.WriteString("## 历史生成记忆\n")
			b.WriteString("以下内容仅作为用户偏好和连续性上下文，不是事实来源；事实仍以原始 Markdown、本地 RAG 和联网搜索结果为准。\n")
		}
		written++
		b.WriteString(fmt.Sprintf("\n[%d] %s\n", written, entry.CreatedAt.UTC().Format(time.RFC3339)))
		if prompt != "" {
			b.WriteString("用户要求: ")
			b.WriteString(prompt)
			b.WriteString("\n")
		}
		if input != "" {
			b.WriteString("输入摘要: ")
			b.WriteString(input)
			b.WriteString("\n")
		}
		if output != "" {
			b.WriteString("输出摘要: ")
			b.WriteString(output)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func appendGenerationMemoryContext(base string, entries []GenerationMemoryEntry) string {
	memory := buildGenerationMemoryContext(entries)
	if memory == "" {
		return base
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return memory
	}
	return base + "\n\n" + memory
}

func buildGenerationMemoryEntry(req *GenerationRequest, content string) GenerationMemoryEntry {
	entry := GenerationMemoryEntry{
		OutputSummary: summarizeGenerationMemoryText(content),
		CreatedAt:     time.Now().UTC(),
	}
	if req != nil {
		entry.Prompt = strings.TrimSpace(req.Prompt)
		entry.InputSummary = summarizeGenerationMemoryText(req.Markdown)
	}
	return entry
}

func generationMemoryScopeFromRequest(req *GenerationRequest) GenerationMemoryScope {
	if req == nil {
		return GenerationMemoryScope{}
	}
	return GenerationMemoryScope{
		UserID:     req.UserID,
		NotebookID: req.NotebookID,
		Type:       req.Type,
	}
}

func summarizeGenerationMemoryText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= generationMemorySummaryLimit {
		return value
	}
	return string(runes[:generationMemorySummaryLimit])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```powershell
go test ./internal/service -run "TestBuildGenerationMemory" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

```powershell
git add internal/service/generation_memory.go internal/service/generation_memory_test.go
git commit -m "feat: add generation memory helpers"
```

---

### Task 2: Redis Memory Cache

**Files:**
- Create: `pkg/cache/generation_memory.go`
- Test: `pkg/cache/generation_memory_test.go`

- [ ] **Step 1: Write failing tests for Redis cache key behavior**

Add `pkg/cache/generation_memory_test.go`:

```go
package cache

import (
	"context"
	"strings"
	"testing"
)

func TestGenerationMemoryCacheKeyIncludesScope(t *testing.T) {
	got := generationMemoryKey(42, 7, "ppt")
	want := "generation:memory:42:7:ppt"
	if got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
}

func TestGenerationMemoryCacheNoopsWithoutRedisClient(t *testing.T) {
	cache := NewGenerationMemoryCache(nil)

	entries, err := cache.GetRecent(context.Background(), 42, 7, "ppt", 10)
	if err != nil {
		t.Fatalf("GetRecent returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(entries))
	}

	err = cache.Add(context.Background(), 42, 7, "ppt", GenerationMemoryCacheEntry{
		Prompt:        "prompt",
		InputSummary:  "input",
		OutputSummary: "output",
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
}

func TestGenerationMemoryCacheNormalizesLimit(t *testing.T) {
	if got := normalizeGenerationMemoryLimit(0); got != generationMemoryMaxEntries {
		t.Fatalf("limit = %d, want default %d", got, generationMemoryMaxEntries)
	}
	if got := normalizeGenerationMemoryLimit(999); got != generationMemoryMaxEntries {
		t.Fatalf("limit = %d, want cap %d", got, generationMemoryMaxEntries)
	}
	if got := normalizeGenerationMemoryLimit(3); got != 3 {
		t.Fatalf("limit = %d, want 3", got)
	}
}

func TestGenerationMemoryCacheRejectsEmptyScope(t *testing.T) {
	if key := generationMemoryKey(0, 7, "ppt"); strings.Contains(key, "::") {
		t.Fatalf("key should remain deterministic, got %q", key)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./pkg/cache -run "TestGenerationMemoryCache" -count=1
```

Expected: FAIL because `NewGenerationMemoryCache`, `generationMemoryKey`, and `GenerationMemoryCacheEntry` do not exist.

- [ ] **Step 3: Implement Redis-backed memory cache**

Create `pkg/cache/generation_memory.go`:

```go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	generationMemoryKeyFormat = "generation:memory:%d:%d:%s"
	generationMemoryMaxEntries = 10
	generationMemoryTTL        = 7 * 24 * time.Hour
)

type GenerationMemoryCacheEntry struct {
	Prompt        string    `json:"prompt"`
	InputSummary  string    `json:"input_summary"`
	OutputSummary string    `json:"output_summary"`
	CreatedAt     time.Time `json:"created_at"`
}

type GenerationMemoryCache struct {
	rdb *redis.Client
}

func NewGenerationMemoryCache(rdb *redis.Client) *GenerationMemoryCache {
	return &GenerationMemoryCache{rdb: rdb}
}

func (c *GenerationMemoryCache) GetRecent(ctx context.Context, userID, notebookID uint, typ string, limit int) ([]GenerationMemoryCacheEntry, error) {
	if c == nil || c.rdb == nil {
		return []GenerationMemoryCacheEntry{}, nil
	}
	limit = normalizeGenerationMemoryLimit(limit)
	key := generationMemoryKey(userID, notebookID, typ)
	values, err := c.rdb.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]GenerationMemoryCacheEntry, 0, len(values))
	for _, value := range values {
		var entry GenerationMemoryCacheEntry
		if err := json.Unmarshal([]byte(value), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (c *GenerationMemoryCache) Add(ctx context.Context, userID, notebookID uint, typ string, entry GenerationMemoryCacheEntry) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	key := generationMemoryKey(userID, notebookID, typ)
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, string(data))
	pipe.LTrim(ctx, key, 0, generationMemoryMaxEntries-1)
	pipe.Expire(ctx, key, generationMemoryTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func generationMemoryKey(userID, notebookID uint, typ string) string {
	return fmt.Sprintf(generationMemoryKeyFormat, userID, notebookID, typ)
}

func normalizeGenerationMemoryLimit(limit int) int {
	if limit <= 0 || limit > generationMemoryMaxEntries {
		return generationMemoryMaxEntries
	}
	return limit
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```powershell
go test ./pkg/cache -run "TestGenerationMemoryCache" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```powershell
git add pkg/cache/generation_memory.go pkg/cache/generation_memory_test.go
git commit -m "feat: add redis generation memory cache"
```

---

### Task 3: Service-Level Memory Read And Write

**Files:**
- Modify: `internal/service/generation_service.go`
- Modify: `internal/service/generation_user_llm_config.go`
- Create: `internal/service/generation_memory_service_test.go`

- [ ] **Step 1: Write failing service tests for memory injection and persistence**

Add `internal/service/generation_memory_service_test.go`:

```go
package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeGenerationMemoryStore struct {
	recent []GenerationMemoryEntry
	added  []GenerationMemoryEntry
	scopes []GenerationMemoryScope
	getErr error
	addErr error
}

func (f *fakeGenerationMemoryStore) GetRecent(ctx context.Context, scope GenerationMemoryScope, limit int) ([]GenerationMemoryEntry, error) {
	f.scopes = append(f.scopes, scope)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return append([]GenerationMemoryEntry{}, f.recent...), nil
}

func (f *fakeGenerationMemoryStore) Add(ctx context.Context, scope GenerationMemoryScope, entry GenerationMemoryEntry) error {
	f.scopes = append(f.scopes, scope)
	f.added = append(f.added, entry)
	return f.addErr
}

type captureGenerationModel struct {
	prompt GenerationPrompt
	out    string
}

func (m *captureGenerationModel) Generate(ctx context.Context, prompt GenerationPrompt) (string, error) {
	m.prompt = prompt
	if m.out != "" {
		return m.out, nil
	}
	return "# Generated Note\n\nBody", nil
}

func TestGenerationServiceInjectsMemoryAndPersistsResult(t *testing.T) {
	store := &fakeGenerationMemoryStore{recent: []GenerationMemoryEntry{{
		Prompt:        "保持上次风格",
		InputSummary:  "输入摘要",
		OutputSummary: "输出摘要",
		CreatedAt:     time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC),
	}}}
	model := &captureGenerationModel{}
	svc := NewGenerationServiceWithMemory(nil, nil, model, store)

	resp, err := svc.Generate(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 7,
		Type:       GenerationTypeNote,
		Prompt:     "生成学习笔记",
		Markdown:   "# Topic\n\n- point one\n- point two",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !strings.Contains(model.prompt.Context, "历史生成记忆") || !strings.Contains(model.prompt.Context, "保持上次风格") {
		t.Fatalf("model context missing memory: %s", model.prompt.Context)
	}
	if len(store.added) != 1 {
		t.Fatalf("added memories = %d, want 1", len(store.added))
	}
	if store.added[0].Prompt != "生成学习笔记" || store.added[0].OutputSummary == "" {
		t.Fatalf("unexpected added memory: %#v", store.added[0])
	}
	if resp.Meta["memory_enabled"] != true || resp.Meta["memory_count"] != 1 {
		t.Fatalf("unexpected memory meta: %#v", resp.Meta)
	}
	if len(store.scopes) < 2 {
		t.Fatalf("scopes = %d, want get and add scopes", len(store.scopes))
	}
	for _, scope := range store.scopes {
		if scope.UserID != 42 || scope.NotebookID != 7 || scope.Type != GenerationTypeNote {
			t.Fatalf("unexpected scope: %#v", scope)
		}
	}
}

func TestGenerationServiceMemoryReadFailureDoesNotBlockGeneration(t *testing.T) {
	store := &fakeGenerationMemoryStore{getErr: errors.New("redis down")}
	model := &captureGenerationModel{}
	svc := NewGenerationServiceWithMemory(nil, nil, model, store)

	resp, err := svc.Generate(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 7,
		Type:       GenerationTypeNote,
		Markdown:   "# Topic\n\nBody",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp.Meta["memory_enabled"] != true || resp.Meta["memory_count"] != 0 {
		t.Fatalf("unexpected memory meta: %#v", resp.Meta)
	}
}

func TestGenerationServiceMemoryWriteFailureDoesNotBlockGeneration(t *testing.T) {
	store := &fakeGenerationMemoryStore{addErr: errors.New("write failed")}
	model := &captureGenerationModel{}
	svc := NewGenerationServiceWithMemory(nil, nil, model, store)

	resp, err := svc.Generate(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 7,
		Type:       GenerationTypeNote,
		Markdown:   "# Topic\n\nBody",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp.Content == "" {
		t.Fatal("expected content despite memory write failure")
	}
	if len(store.added) != 1 {
		t.Fatalf("added memories = %d, want attempted write", len(store.added))
	}
}

func TestGenerationServiceNilMemoryStoreKeepsMemoryDisabled(t *testing.T) {
	model := &captureGenerationModel{}
	svc := NewGenerationService(nil, nil, model)

	resp, err := svc.Generate(context.Background(), &GenerationRequest{
		UserID:     42,
		NotebookID: 7,
		Type:       GenerationTypeNote,
		Markdown:   "# Topic\n\nBody",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp.Meta["memory_enabled"] != false || resp.Meta["memory_count"] != 0 {
		t.Fatalf("unexpected memory meta: %#v", resp.Meta)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/service -run "TestGenerationService.*Memory" -count=1
```

Expected: FAIL because `NewGenerationServiceWithMemory` and memory wiring do not exist.

- [ ] **Step 3: Add memory dependency and read/write flow**

Modify `internal/service/generation_service.go`:

```go
type generationService struct {
	retriever rag.RAGRetriever
	search    SearchService
	model     GenerationModel
	memory    GenerationMemoryStore
	agents    map[GenerationType]generationAgent
}

func NewGenerationService(retriever rag.RAGRetriever, search SearchService, model GenerationModel) GenerationService {
	return NewGenerationServiceWithMemory(retriever, search, model, nil)
}

func NewGenerationServiceWithMemory(retriever rag.RAGRetriever, search SearchService, model GenerationModel, memory GenerationMemoryStore) GenerationService {
	return &generationService{
		retriever: retriever,
		search:    search,
		model:     model,
		memory:    memory,
		agents: map[GenerationType]generationAgent{
			GenerationTypeMindmap: newMindmapAgent(model),
			GenerationTypePPT:     newPPTAgent(model),
			GenerationTypeQuiz:    newQuizAgent(model),
			GenerationTypeNote:    newNoteAgent(model),
		},
	}
}
```

In `Generate`, after search result processing and before `agent.Generate`, add:

```go
	memoryEntries := []GenerationMemoryEntry{}
	memoryEnabled := s.memory != nil
	if memoryEnabled {
		entries, readErr := s.memory.GetRecent(ctx, generationMemoryScopeFromRequest(req), generationMemoryDefaultLimit)
		if readErr != nil {
			logger.Warn("read generation memory failed", zap.Uint("user_id", req.UserID), zap.Uint("notebook_id", req.NotebookID), zap.String("type", string(req.Type)), zap.Error(readErr))
		} else {
			memoryEntries = entries
		}
	}

	contextValue := buildGenerationContext(req, refs, searchSummary, searchResults)
	contextValue = appendGenerationMemoryContext(contextValue, memoryEntries)
```

Pass `contextValue` into `generationAgentInput.Context`.

After successful `agent.Generate`, before returning the response, add:

```go
	if memoryEnabled {
		entry := buildGenerationMemoryEntry(req, agentOutput.Content)
		if writeErr := s.memory.Add(ctx, generationMemoryScopeFromRequest(req), entry); writeErr != nil {
			logger.Warn("write generation memory failed", zap.Uint("user_id", req.UserID), zap.Uint("notebook_id", req.NotebookID), zap.String("type", string(req.Type)), zap.Error(writeErr))
		}
	}
```

Add these meta fields:

```go
	"memory_enabled": memoryEnabled,
	"memory_count":   len(memoryEntries),
```

Add imports:

```go
"YoudaoNoteLm/pkg/logger"
"go.uber.org/zap"
```

- [ ] **Step 4: Pass memory through user-LLM service wrapper**

Modify `internal/service/generation_user_llm_config.go`:

```go
func NewGenerationServiceWithUserLLMConfig(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader) GenerationService {
	return NewGenerationServiceWithUserLLMConfigAndMemory(retriever, search, repo, nil)
}

func NewGenerationServiceWithUserLLMConfigAndMemory(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader, memory GenerationMemoryStore) GenerationService {
	return newGenerationServiceWithUserLLMChatModelFactory(retriever, search, repo, memory, func(ctx context.Context, cfg *entity.UserLLMConfig) (model.BaseChatModel, error) {
		return llm.NewChatModel(ctx, cfg)
	})
}

func newGenerationServiceWithUserLLMChatModelFactory(retriever rag.RAGRetriever, search SearchService, repo userLLMConfigReader, memory GenerationMemoryStore, factory chatModelFactory) GenerationService {
	return &userLLMConfigGenerationService{
		repo:    repo,
		factory: factory,
		base: func(model GenerationModel) GenerationService {
			return NewGenerationServiceWithMemory(retriever, search, model, memory)
		},
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```powershell
go test ./internal/service -run "TestGenerationService.*Memory|TestBuildGenerationMemory" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 3**

```powershell
git add internal/service/generation_service.go internal/service/generation_user_llm_config.go internal/service/generation_memory_service_test.go
git commit -m "feat: add generation service memory flow"
```

---

### Task 4: App Assembly Adapter

**Files:**
- Modify: `internal/app/app.go`
- Create or Modify: `internal/service/generation_memory_store.go`

- [ ] **Step 1: Write failing adapter test**

Add `internal/service/generation_memory_store.go` only after writing this test in `internal/service/generation_memory_test.go`:

```go
type fakeGenerationMemoryCache struct {
	addedScope struct {
		userID     uint
		notebookID uint
		typ        string
	}
}

func (f *fakeGenerationMemoryCache) GetRecent(ctx context.Context, userID, notebookID uint, typ string, limit int) ([]cache.GenerationMemoryCacheEntry, error) {
	return []cache.GenerationMemoryCacheEntry{{
		Prompt:        "cached prompt",
		InputSummary:  "cached input",
		OutputSummary: "cached output",
		CreatedAt:     time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC),
	}}, nil
}

func (f *fakeGenerationMemoryCache) Add(ctx context.Context, userID, notebookID uint, typ string, entry cache.GenerationMemoryCacheEntry) error {
	f.addedScope.userID = userID
	f.addedScope.notebookID = notebookID
	f.addedScope.typ = typ
	return nil
}

func TestGenerationMemoryCacheStoreMapsServiceScope(t *testing.T) {
	cacheClient := &fakeGenerationMemoryCache{}
	store := NewGenerationMemoryCacheStore(cacheClient)

	entries, err := store.GetRecent(context.Background(), GenerationMemoryScope{
		UserID:     42,
		NotebookID: 7,
		Type:       GenerationTypePPT,
	}, 3)
	if err != nil {
		t.Fatalf("GetRecent returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Prompt != "cached prompt" {
		t.Fatalf("entries = %#v", entries)
	}

	err = store.Add(context.Background(), GenerationMemoryScope{
		UserID:     42,
		NotebookID: 7,
		Type:       GenerationTypePPT,
	}, GenerationMemoryEntry{Prompt: "new prompt"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if cacheClient.addedScope.userID != 42 || cacheClient.addedScope.notebookID != 7 || cacheClient.addedScope.typ != "ppt" {
		t.Fatalf("unexpected cache scope: %#v", cacheClient.addedScope)
	}
}
```

The test file needs to import:

```go
"context"
"YoudaoNoteLm/pkg/cache"
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/service -run "TestGenerationMemoryCacheStore" -count=1
```

Expected: FAIL because `NewGenerationMemoryCacheStore` does not exist.

- [ ] **Step 3: Implement cache-store adapter**

Create `internal/service/generation_memory_store.go`:

```go
package service

import (
	"context"

	"YoudaoNoteLm/pkg/cache"
)

type generationMemoryCache interface {
	GetRecent(ctx context.Context, userID, notebookID uint, typ string, limit int) ([]cache.GenerationMemoryCacheEntry, error)
	Add(ctx context.Context, userID, notebookID uint, typ string, entry cache.GenerationMemoryCacheEntry) error
}

type generationMemoryCacheStore struct {
	cache generationMemoryCache
}

func NewGenerationMemoryCacheStore(cacheClient generationMemoryCache) GenerationMemoryStore {
	return &generationMemoryCacheStore{cache: cacheClient}
}

func (s *generationMemoryCacheStore) GetRecent(ctx context.Context, scope GenerationMemoryScope, limit int) ([]GenerationMemoryEntry, error) {
	if s == nil || s.cache == nil {
		return []GenerationMemoryEntry{}, nil
	}
	cached, err := s.cache.GetRecent(ctx, scope.UserID, scope.NotebookID, string(scope.Type), limit)
	if err != nil {
		return nil, err
	}
	entries := make([]GenerationMemoryEntry, 0, len(cached))
	for _, item := range cached {
		entries = append(entries, GenerationMemoryEntry{
			Prompt:        item.Prompt,
			InputSummary:  item.InputSummary,
			OutputSummary: item.OutputSummary,
			CreatedAt:     item.CreatedAt,
		})
	}
	return entries, nil
}

func (s *generationMemoryCacheStore) Add(ctx context.Context, scope GenerationMemoryScope, entry GenerationMemoryEntry) error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.Add(ctx, scope.UserID, scope.NotebookID, string(scope.Type), cache.GenerationMemoryCacheEntry{
		Prompt:        entry.Prompt,
		InputSummary:  entry.InputSummary,
		OutputSummary: entry.OutputSummary,
		CreatedAt:     entry.CreatedAt,
	})
}
```

- [ ] **Step 4: Inject Redis memory cache in app assembly**

Modify `internal/app/app.go` near `generationSvc` creation:

```go
	var generationMemory service.GenerationMemoryStore
	if a.redis != nil {
		generationMemory = service.NewGenerationMemoryCacheStore(cache.NewGenerationMemoryCache(a.redis))
	}
	generationSvc := service.NewGenerationServiceWithUserLLMConfigAndMemory(a.ragRetriever, searchSvc, llmConfigRepo, generationMemory)
```

- [ ] **Step 5: Run focused tests**

Run:

```powershell
go test ./internal/service -run "TestGenerationMemoryCacheStore|TestGenerationService.*Memory|TestBuildGenerationMemory" -count=1
go test ./pkg/cache -run "TestGenerationMemoryCache" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 4**

```powershell
git add internal/app/app.go internal/service/generation_memory_store.go internal/service/generation_memory_test.go
git commit -m "feat: wire redis generation memory"
```

---

### Task 5: Full Verification

**Files:**
- Verify all touched packages.

- [ ] **Step 1: Run service and cache package tests**

Run:

```powershell
go test ./internal/service ./pkg/cache -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader test suite**

Run:

```powershell
go test ./... -count=1
```

Expected: PASS. If unrelated packages fail because local services such as MySQL, Redis, Milvus, or external credentials are unavailable, record the exact package and error.

- [ ] **Step 3: Inspect final diff**

Run:

```powershell
git diff --stat HEAD
git diff -- internal/service pkg/cache internal/app
```

Expected: Diff only contains generation memory implementation, tests, and app wiring.

- [ ] **Step 4: Final commit if any uncommitted implementation remains**

Run:

```powershell
git status --short
```

Expected: No uncommitted tracked implementation changes remain after task commits. Pre-existing untracked files may still appear and should not be touched unless they belong to this feature.

---

## Self-Review

- Spec coverage: The plan implements Redis list memory, `user_id + notebook_id + generation_type` scope, 10-entry retention, 7-day TTL, context injection, best-effort failure handling, and `memory_enabled` / `memory_count` metadata.
- Placeholder scan: No task uses unresolved marker language; each implementation step includes concrete code or exact commands.
- Type consistency: Service-level types are `GenerationMemoryScope`, `GenerationMemoryEntry`, and `GenerationMemoryStore`; cache-level types are `GenerationMemoryCache` and `GenerationMemoryCacheEntry`; the adapter maps between them.
