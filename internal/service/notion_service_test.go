package service

import (
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/rag"
	"YoudaoNoteLm/internal/repository"
	notion "YoudaoNoteLm/internal/service/external/notion"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/utils"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testEncryptionKey = "0123456789abcdef0123456789abcdef"

func TestStartOAuthStoresUserBoundStateAndBuildsURL(t *testing.T) {
	d := newNotionServiceTestDeps()
	svc := d.service(t)

	authorizeURL, err := svc.StartOAuth(context.Background(), 7)
	require.NoError(t, err)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	require.Equal(t, "https", parsed.Scheme)
	require.Equal(t, "api.notion.com", parsed.Host)
	q := parsed.Query()
	require.Equal(t, "user", q.Get("owner"))
	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, "client-test", q.Get("client_id"))
	require.Equal(t, "https://app.example.test/callback", q.Get("redirect_uri"))
	require.NotEmpty(t, q.Get("state"))
	require.Len(t, q.Get("state"), 43) // base64url-encoded 32 random bytes
	require.Equal(t, uint(7), d.states.states[q.Get("state")].userID)
	require.Equal(t, 10*time.Minute, d.states.states[q.Get("state")].ttl)
}

func TestHandleOAuthCallbackConsumesStateAndEncryptsToken(t *testing.T) {
	d := newNotionServiceTestDeps()
	d.states.states["state-once"] = storedState{userID: 7}
	d.client.exchange = notion.OAuthToken{AccessToken: "test-token-not-real", WorkspaceID: "workspace-1", WorkspaceName: "工作区", BotID: "bot-1"}
	svc := d.service(t)

	result, err := svc.HandleOAuthCallback(context.Background(), "test-code-not-real", "state-once")
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, OAuthCallbackReasonSuccess, result.Reason)
	require.NotNil(t, d.bindings.binding)
	require.NotEqual(t, "test-token-not-real", d.bindings.binding.AccessTokenEncrypted)
	plain, err := utils.Decrypt(d.bindings.binding.AccessTokenEncrypted, []byte(testEncryptionKey))
	require.NoError(t, err)
	require.Equal(t, "test-token-not-real", plain)
	require.Equal(t, "active", d.bindings.binding.Status)
	require.Empty(t, d.states.states)
}

func TestHandleOAuthCallbackRejectsReplayedState(t *testing.T) {
	d := newNotionServiceTestDeps()
	d.states.states["state-once"] = storedState{userID: 7}
	d.client.exchange = notion.OAuthToken{AccessToken: "placeholder-token", WorkspaceID: "workspace-1"}
	svc := d.service(t)

	_, err := svc.HandleOAuthCallback(context.Background(), "placeholder-code", "state-once")
	require.NoError(t, err)
	result, err := svc.HandleOAuthCallback(context.Background(), "placeholder-code", "state-once")
	require.Error(t, err)
	require.False(t, result.Success)
	require.Equal(t, OAuthCallbackReasonInvalidState, result.Reason)
	require.Equal(t, 1, d.client.exchangeCalls)
}

func TestListPagesRequiresActiveBindingAndDecryptsToken(t *testing.T) {
	d := newNotionServiceTestDeps()
	ciphertext, err := utils.Encrypt("placeholder-access-token", []byte(testEncryptionKey))
	require.NoError(t, err)
	d.bindings.binding = &entity.NotionBinding{UserID: 7, Status: "active", AccessTokenEncrypted: ciphertext}
	d.client.search = notion.PageSearchResult{Pages: []notion.Page{{ID: "page-1", Title: "页面", URL: "https://www.notion.so/page-1", HasChildren: true}}, NextCursor: "next", HasMore: true}
	svc := d.service(t)

	pages, err := svc.ListPages(context.Background(), 7, "query", "cursor", 0)
	require.NoError(t, err)
	require.Equal(t, "placeholder-access-token", d.client.searchToken)
	require.Equal(t, 50, d.client.searchPageSize)
	require.Equal(t, []PageItem{{ID: "page-1", Title: "页面", URL: "https://www.notion.so/page-1", HasChildren: true}}, pages.Items)
	require.Equal(t, "next", pages.NextCursor)
	require.True(t, pages.HasMore)
}

func TestImportPagesBatchRejectsForeignNotebook(t *testing.T) {
	d := newNotionServiceTestDeps()
	d.notebooks.notebook = &entity.Notebook{UserID: 9}
	svc := d.service(t)

	_, _, err := svc.ImportPagesBatch(context.Background(), 7, 3, []string{"page-1"})
	require.ErrorIs(t, err, bizerrors.ErrForbidden)
	require.Empty(t, d.sources.sources)
}

func TestImportPagesBatchDeduplicatesAndCreatesPendingSources(t *testing.T) {
	d := newNotionServiceTestDeps()
	d.notebooks.notebook = &entity.Notebook{UserID: 7}
	d.bindActive(t, 7)
	d.client.waitForContext = true
	svc := d.service(t)

	taskID, sourceIDs, err := svc.ImportPagesBatch(context.Background(), 7, 3, []string{"page-1", "page-1", "page-2"})
	require.NoError(t, err)
	require.NotEmpty(t, taskID)
	require.Len(t, sourceIDs, 2)
	require.NotNil(t, d.tasks.tasks[taskID])
	require.Equal(t, "notion", d.tasks.tasks[taskID].TaskType)
	require.Contains(t, []string{"pending", "running"}, d.tasks.tasks[taskID].Status)
	for _, id := range sourceIDs {
		source := d.sources.get(id)
		require.Equal(t, "notion", source.Type)
		require.Equal(t, "pending", source.Status)
		require.Contains(t, []string{"page-1", "page-2"}, source.ExternalID)
	}
	require.NoError(t, svc.CancelImportTask(context.Background(), 7, taskID))
}

func TestProcessPageMarksReadyAfterIngestion(t *testing.T) {
	d := newNotionServiceTestDeps()
	d.bindActive(t, 7)
	d.sources.add(&entity.Source{BaseEntity: entity.BaseEntity{ID: 11}, UserID: 7, NotebookID: 3, Name: "旧名", Type: "notion", ExternalID: "page-1", Status: "pending"})
	d.client.page = notion.Page{ID: "page-1", Title: "页面标题", URL: "https://www.notion.so/page-1"}
	d.client.blocks = notion.BlockChildrenResult{Blocks: []notion.Block{paragraphBlock("block-1", "正文")}}
	svc := d.service(t)

	err := svc.processPage(context.Background(), "placeholder-access-token", 11, "page-1")
	require.NoError(t, err)
	source := d.sources.get(11)
	require.Equal(t, "ready", source.Status)
	require.Equal(t, "页面标题", source.Name)
	require.Equal(t, "https://www.notion.so/page-1", source.OriginalURL)
	require.Contains(t, source.MarkdownContent, "正文")
	require.Equal(t, StructureMeta{Title: "页面标题", SourceType: "notion"}, d.structurer.meta)
	require.Equal(t, []uint{11}, d.ingestion.ids)
}

func TestProcessPageMarksFailedOnNotionAuthExpiry(t *testing.T) {
	d := newNotionServiceTestDeps()
	d.sources.add(&entity.Source{BaseEntity: entity.BaseEntity{ID: 11}, UserID: 7, NotebookID: 3, Type: "notion", ExternalID: "page-1", Status: "pending"})
	d.client.getPageErr = notion.ErrAuthExpired
	svc := d.service(t)

	err := svc.processPage(context.Background(), "placeholder-access-token", 11, "page-1")
	require.ErrorIs(t, err, bizerrors.ErrNotionAuthExpired)
	source := d.sources.get(11)
	require.Equal(t, "failed", source.Status)
	require.Equal(t, bizerrors.ErrNotionAuthExpired.Message, source.ErrorMessage)
}

func TestCancelImportTaskMarksPendingSourcesCancelled(t *testing.T) {
	d := newNotionServiceTestDeps()
	d.notebooks.notebook = &entity.Notebook{UserID: 7}
	d.bindActive(t, 7)
	d.client.waitForContext = true
	d.client.started = make(chan struct{}, 2)
	svc := d.service(t)

	taskID, sourceIDs, err := svc.ImportPagesBatch(context.Background(), 7, 3, []string{"page-1", "page-2", "page-3"})
	require.NoError(t, err)
	<-d.client.started
	<-d.client.started
	require.NoError(t, svc.CancelImportTask(context.Background(), 7, taskID))
	require.Eventually(t, func() bool { return d.sources.get(sourceIDs[2]).Status == "cancelled" }, time.Second, 10*time.Millisecond)
	require.Equal(t, "cancelled", d.tasks.tasks[taskID].Status)
}

type notionServiceTestDeps struct {
	client     *fakeNotionClient
	bindings   *fakeNotionBindingRepository
	notebooks  *fakeNotebookRepository
	sources    *fakeSourceRepository
	users      *fakeUserRepository
	states     *fakeOAuthStateStore
	tasks      *fakeImportTaskStore
	ingestion  *fakeIngestionService
	structurer *fakeMarkdownStructurer
}

func newNotionServiceTestDeps() *notionServiceTestDeps {
	return &notionServiceTestDeps{
		client: &fakeNotionClient{}, bindings: &fakeNotionBindingRepository{}, notebooks: &fakeNotebookRepository{},
		sources: newFakeSourceRepository(), users: &fakeUserRepository{user: &entity.User{BaseEntity: entity.BaseEntity{ID: 7}, Status: 1}},
		states: &fakeOAuthStateStore{states: map[string]storedState{}}, tasks: &fakeImportTaskStore{tasks: map[string]*cache.ImportTask{}},
		ingestion: &fakeIngestionService{}, structurer: &fakeMarkdownStructurer{},
	}
}

func (d *notionServiceTestDeps) service(t *testing.T) *notionService {
	t.Helper()
	svc := NewNotionService(config.NotionConfig{ClientID: "client-test", ClientSecret: "secret-placeholder", RedirectURI: "https://app.example.test/callback", FrontendRedirectURL: "https://web.example.test/settings"}, d.client, d.bindings, d.notebooks, d.sources, d.users, d.states, d.tasks, d.ingestion, d.structurer, nil, nil, testEncryptionKey)
	return svc.(*notionService)
}

func (d *notionServiceTestDeps) bindActive(t *testing.T, userID uint) {
	t.Helper()
	ciphertext, err := utils.Encrypt("placeholder-access-token", []byte(testEncryptionKey))
	require.NoError(t, err)
	d.bindings.binding = &entity.NotionBinding{UserID: userID, Status: "active", AccessTokenEncrypted: ciphertext}
}

type storedState struct {
	userID uint
	ttl    time.Duration
}
type fakeOAuthStateStore struct {
	mu     sync.Mutex
	states map[string]storedState
}

func (f *fakeOAuthStateStore) Save(_ context.Context, state string, userID uint, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[state] = storedState{userID, ttl}
	return nil
}
func (f *fakeOAuthStateStore) Consume(_ context.Context, state string) (uint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.states[state]
	if !ok {
		return 0, errors.New("state missing")
	}
	delete(f.states, state)
	return value.userID, nil
}

type fakeNotionClient struct {
	exchange       notion.OAuthToken
	exchangeErr    error
	exchangeCalls  int
	search         notion.PageSearchResult
	searchErr      error
	searchToken    string
	searchPageSize int
	page           notion.Page
	getPageErr     error
	blocks         notion.BlockChildrenResult
	blocksErr      error
	waitForContext bool
	started        chan struct{}
}

func (f *fakeNotionClient) ExchangeCode(context.Context, string) (notion.OAuthToken, error) {
	f.exchangeCalls++
	return f.exchange, f.exchangeErr
}
func (f *fakeNotionClient) SearchPages(_ context.Context, token, _, _ string, pageSize int) (notion.PageSearchResult, error) {
	f.searchToken, f.searchPageSize = token, pageSize
	return f.search, f.searchErr
}
func (f *fakeNotionClient) GetPage(ctx context.Context, _ string, _ string) (notion.Page, error) {
	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.waitForContext {
		<-ctx.Done()
		return notion.Page{}, ctx.Err()
	}
	return f.page, f.getPageErr
}
func (f *fakeNotionClient) ListBlockChildren(context.Context, string, string, string, int) (notion.BlockChildrenResult, error) {
	return f.blocks, f.blocksErr
}
func (f *fakeNotionClient) RevokeToken(context.Context, string) error { return nil }

type fakeNotionBindingRepository struct {
	binding *entity.NotionBinding
	deleted bool
}

func (f *fakeNotionBindingRepository) FindByUserID(uint) (*entity.NotionBinding, error) {
	return f.binding, nil
}
func (f *fakeNotionBindingRepository) Upsert(binding *entity.NotionBinding) error {
	f.binding = binding
	return nil
}
func (f *fakeNotionBindingRepository) Delete(uint) error {
	f.deleted = true
	f.binding = nil
	return nil
}

type fakeNotebookRepository struct{ notebook *entity.Notebook }

func (f *fakeNotebookRepository) FindByID(uint) (*entity.Notebook, error)     { return f.notebook, nil }
func (*fakeNotebookRepository) Create(*entity.Notebook) error                 { return nil }
func (*fakeNotebookRepository) ExistsByName(uint, string) (bool, error)       { return false, nil }
func (*fakeNotebookRepository) ListByUserID(uint) ([]*entity.Notebook, error) { return nil, nil }
func (*fakeNotebookRepository) Update(*entity.Notebook) error                 { return nil }
func (*fakeNotebookRepository) Delete(uint) error                             { return nil }
func (*fakeNotebookRepository) CountByUserID(uint) (int64, error)             { return 0, nil }

type fakeSourceRepository struct {
	mu      sync.Mutex
	sources map[uint]*entity.Source
	nextID  uint
}

func newFakeSourceRepository() *fakeSourceRepository {
	return &fakeSourceRepository{sources: map[uint]*entity.Source{}, nextID: 1}
}
func (f *fakeSourceRepository) add(source *entity.Source) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources[source.ID] = source
	if source.ID >= f.nextID {
		f.nextID = source.ID + 1
	}
}
func (f *fakeSourceRepository) get(id uint) *entity.Source {
	f.mu.Lock()
	defer f.mu.Unlock()
	value := *f.sources[id]
	return &value
}
func (f *fakeSourceRepository) FindByID(id uint) (*entity.Source, error) { return f.get(id), nil }
func (f *fakeSourceRepository) FindByIDs(ids []uint) ([]*entity.Source, error) {
	result := make([]*entity.Source, 0, len(ids))
	for _, id := range ids {
		result = append(result, f.get(id))
	}
	return result, nil
}
func (f *fakeSourceRepository) Create(source *entity.Source) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	source.ID = f.nextID
	f.nextID++
	f.sources[source.ID] = source
	return nil
}
func (f *fakeSourceRepository) Update(source *entity.Source) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources[source.ID] = source
	return nil
}
func (f *fakeSourceRepository) UpdateContent(id uint, markdown, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources[id].MarkdownContent, f.sources[id].Status = markdown, status
	return nil
}
func (f *fakeSourceRepository) UpdateSummary(uint, string) error { return nil }
func (f *fakeSourceRepository) UpdateStatus(id uint, status, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources[id].Status, f.sources[id].ErrorMessage = status, message
	return nil
}
func (*fakeSourceRepository) SetVectorized(uint) error      { return nil }
func (*fakeSourceRepository) Delete(uint) error             { return nil }
func (*fakeSourceRepository) BatchDelete([]uint) error      { return nil }
func (*fakeSourceRepository) DeleteByNotebookID(uint) error { return nil }
func (*fakeSourceRepository) ListByNotebook(uint, uint, string, int, int) ([]*entity.Source, int64, error) {
	return nil, 0, nil
}
func (*fakeSourceRepository) DeleteFailedByNotebook(uint, uint) (int64, error) { return 0, nil }
func (*fakeSourceRepository) ResetVectorizedByUserID(uint) error               { return nil }
func (*fakeSourceRepository) FindUnvectorizedByUserID(uint) ([]*entity.Source, error) {
	return nil, nil
}
func (*fakeSourceRepository) FindSummaryByID(uint) (string, error)                 { return "", nil }
func (*fakeSourceRepository) FindReadyByNotebookID(uint) ([]*entity.Source, error) { return nil, nil }

type fakeUserRepository struct{ user *entity.User }

func (f *fakeUserRepository) FindByID(uint) (*entity.User, error)        { return f.user, nil }
func (*fakeUserRepository) FindByUsername(string) (*entity.User, error)  { return nil, nil }
func (*fakeUserRepository) FindByEmail(string) (*entity.User, error)     { return nil, nil }
func (*fakeUserRepository) Create(*entity.User) error                    { return nil }
func (*fakeUserRepository) Update(*entity.User) error                    { return nil }
func (*fakeUserRepository) Delete(uint) error                            { return nil }
func (*fakeUserRepository) HardDelete(uint) error                        { return nil }
func (*fakeUserRepository) List(int, int) ([]*entity.User, int64, error) { return nil, 0, nil }
func (*fakeUserRepository) ExistsByUsername(string) (bool, error)        { return false, nil }
func (*fakeUserRepository) ExistsByEmail(string) (bool, error)           { return false, nil }
func (*fakeUserRepository) UpdateLoginAttempts(uint, int) error          { return nil }
func (*fakeUserRepository) LockUser(uint, time.Time) error               { return nil }
func (*fakeUserRepository) ResetLoginAttempts(uint) error                { return nil }

type fakeImportTaskStore struct {
	mu      sync.Mutex
	tasks   map[string]*cache.ImportTask
	cancels map[string]context.CancelFunc
}

func (f *fakeImportTaskStore) Save(_ context.Context, task *cache.ImportTask) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks[task.TaskID] = task
	return nil
}
func (f *fakeImportTaskStore) GetForUser(_ context.Context, userID uint, id string) (*cache.ImportTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	task := f.tasks[id]
	if task == nil || task.UserID != userID {
		return nil, bizerrors.ErrNotFound
	}
	return task, nil
}
func (f *fakeImportTaskStore) CancelForUser(ctx context.Context, userID uint, id string) (*cache.ImportTask, error) {
	task, err := f.GetForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	task.Status = "cancelled"
	cancel := f.cancels[id]
	f.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return task, nil
}
func (f *fakeImportTaskStore) RegisterCancel(id string, cancel context.CancelFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancels == nil {
		f.cancels = map[string]context.CancelFunc{}
	}
	f.cancels[id] = cancel
}
func (f *fakeImportTaskStore) ClearCancel(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cancels, id)
}
func (f *fakeImportTaskStore) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.tasks, id)
	return nil
}

type fakeIngestionService struct{ ids []uint }

func (f *fakeIngestionService) IngestSingle(_ context.Context, id uint) error {
	f.ids = append(f.ids, id)
	return nil
}
func (*fakeIngestionService) Ingest(context.Context, []uint) error           { return nil }
func (*fakeIngestionService) DeleteSource(context.Context, uint, uint) error { return nil }
func (*fakeIngestionService) DropUserCollection(context.Context, uint) error { return nil }

var _ rag.IngestionService = (*fakeIngestionService)(nil)

type fakeMarkdownStructurer struct{ meta StructureMeta }

func (f *fakeMarkdownStructurer) Structure(_ context.Context, _ uint, content string, meta StructureMeta) (StructureResult, error) {
	f.meta = meta
	return StructureResult{Content: content}, nil
}

func paragraphBlock(id, content string) notion.Block {
	raw, _ := json.Marshal(map[string]any{"rich_text": []any{map[string]any{"plain_text": content}}})
	return notion.Block{ID: id, Type: "paragraph", Data: raw}
}

var _ repository.NotionBindingRepository = (*fakeNotionBindingRepository)(nil)
var _ repository.NotebookRepository = (*fakeNotebookRepository)(nil)
var _ repository.SourceRepository = (*fakeSourceRepository)(nil)
var _ repository.UserRepository = (*fakeUserRepository)(nil)
