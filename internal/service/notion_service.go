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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	notionOAuthAuthorizeURL = "https://api.notion.com/v1/oauth/authorize"
	notionOAuthStateTTL     = 10 * time.Minute
	notionPageSizeDefault   = 50
	notionPageSizeMax       = 100
	notionBatchSizeMax      = 50
)

type notionService struct {
	config        config.NotionConfig
	client        notion.Client
	bindingRepo   repository.NotionBindingRepository
	notebookRepo  repository.NotebookRepository
	sourceRepo    repository.SourceRepository
	userRepo      repository.UserRepository
	stateStore    OAuthStateStore
	taskStore     ImportTaskStore
	ingestionSvc  rag.IngestionService
	structurer    MarkdownStructurer
	configSvc     ConfigService
	summaryCache  *cache.SourceSummaryCache
	encryptionKey []byte
}

func NewNotionService(
	notionConfig config.NotionConfig,
	client notion.Client,
	bindingRepo repository.NotionBindingRepository,
	notebookRepo repository.NotebookRepository,
	sourceRepo repository.SourceRepository,
	userRepo repository.UserRepository,
	stateStore OAuthStateStore,
	taskStore ImportTaskStore,
	ingestionSvc rag.IngestionService,
	structurer MarkdownStructurer,
	configSvc ConfigService,
	summaryCache *cache.SourceSummaryCache,
	encryptionKey string,
) NotionService {
	return &notionService{
		config: notionConfig, client: client, bindingRepo: bindingRepo, notebookRepo: notebookRepo,
		sourceRepo: sourceRepo, userRepo: userRepo, stateStore: stateStore, taskStore: taskStore,
		ingestionSvc: ingestionSvc, structurer: structurer, configSvc: configSvc, summaryCache: summaryCache,
		encryptionKey: []byte(encryptionKey),
	}
}

func (s *notionService) StartOAuth(ctx context.Context, userID uint) (string, error) {
	if !s.config.Configured() {
		return "", bizerrors.ErrNotionNotConfigured
	}
	if s.stateStore == nil {
		return "", bizerrors.ErrNotionNotConfigured
	}
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("generate notion oauth state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	if err := s.stateStore.Save(ctx, state, userID, notionOAuthStateTTL); err != nil {
		return "", fmt.Errorf("save notion oauth state: %w", err)
	}
	values := url.Values{}
	values.Set("owner", "user")
	values.Set("response_type", "code")
	values.Set("client_id", s.config.ClientID)
	values.Set("redirect_uri", s.config.RedirectURI)
	values.Set("state", state)
	return notionOAuthAuthorizeURL + "?" + values.Encode(), nil
}

func (s *notionService) HandleOAuthCallback(ctx context.Context, code, state string) (OAuthCallbackResult, error) {
	if !s.config.Configured() {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonExchangeFail}, bizerrors.ErrNotionNotConfigured
	}
	if strings.TrimSpace(state) == "" || s.stateStore == nil {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonInvalidState}, bizerrors.ErrInvalidParam
	}
	userID, err := s.stateStore.Consume(ctx, state)
	if err != nil || userID == 0 {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonInvalidState}, bizerrors.ErrInvalidParam
	}
	// Consume state before inspecting the code so a denied authorization cannot
	// leave a reusable state value in Redis.
	if strings.TrimSpace(code) == "" {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonExchangeFail}, bizerrors.ErrInvalidParam
	}
	user, err := s.userRepo.FindByID(userID)
	if err != nil || user == nil {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonUserDisabled}, bizerrors.ErrUserNotFound
	}
	if user.Status != 1 {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonUserDisabled}, bizerrors.ErrUserDisabled
	}
	token, err := s.client.ExchangeCode(ctx, code)
	if err != nil || strings.TrimSpace(token.AccessToken) == "" {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonExchangeFail}, fmt.Errorf("exchange notion oauth code: %w", err)
	}
	if len(s.encryptionKey) != 32 {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonSaveFail}, fmt.Errorf("invalid notion encryption key length")
	}
	encryptedToken, err := utils.Encrypt(token.AccessToken, s.encryptionKey)
	if err != nil {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonSaveFail}, fmt.Errorf("encrypt notion token: %w", err)
	}
	if err := s.bindingRepo.Delete(userID); err != nil {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonSaveFail}, fmt.Errorf("delete previous notion binding: %w", err)
	}
	binding := &entity.NotionBinding{
		UserID: userID, WorkspaceID: token.WorkspaceID, WorkspaceName: token.WorkspaceName,
		WorkspaceIcon: token.WorkspaceIcon, BotID: token.BotID, AccessTokenEncrypted: encryptedToken, Status: "active",
	}
	if err := s.bindingRepo.Upsert(binding); err != nil {
		return OAuthCallbackResult{Reason: OAuthCallbackReasonSaveFail}, fmt.Errorf("save notion binding: %w", err)
	}
	return OAuthCallbackResult{Success: true, Reason: OAuthCallbackReasonSuccess}, nil
}

func (s *notionService) GetBinding(_ context.Context, userID uint) (*NotionBindingView, error) {
	binding, err := s.bindingRepo.FindByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("find notion binding: %w", err)
	}
	if binding == nil {
		return nil, nil
	}
	return &NotionBindingView{WorkspaceID: binding.WorkspaceID, WorkspaceName: binding.WorkspaceName, WorkspaceIcon: binding.WorkspaceIcon, BotID: binding.BotID, Status: binding.Status}, nil
}

func (s *notionService) Unbind(ctx context.Context, userID uint) error {
	binding, err := s.bindingRepo.FindByUserID(userID)
	if err != nil {
		return fmt.Errorf("find notion binding: %w", err)
	}
	if binding == nil {
		return nil
	}
	accessToken, decryptErr := s.decryptBinding(binding)
	if err := s.bindingRepo.Delete(userID); err != nil {
		return fmt.Errorf("delete notion binding: %w", err)
	}
	if decryptErr == nil && s.client != nil {
		remoteCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		_ = s.client.RevokeToken(remoteCtx, accessToken) // local credential deletion must not be blocked by remote revoke.
	}
	return nil
}

func (s *notionService) ListPages(ctx context.Context, userID uint, query, cursor string, pageSize int) (PageList, error) {
	if !s.config.Configured() {
		return PageList{}, bizerrors.ErrNotionNotConfigured
	}
	accessToken, err := s.activeAccessToken(userID)
	if err != nil {
		return PageList{}, err
	}
	if pageSize <= 0 {
		pageSize = notionPageSizeDefault
	}
	if pageSize > notionPageSizeMax {
		return PageList{}, bizerrors.ErrInvalidParam
	}
	result, err := s.client.SearchPages(ctx, accessToken, query, cursor, pageSize)
	if err != nil {
		return PageList{}, mapNotionError(err)
	}
	items := make([]PageItem, 0, len(result.Pages))
	for _, page := range result.Pages {
		title := strings.TrimSpace(page.Title)
		if title == "" {
			title = "未命名 Notion 页面"
		}
		items = append(items, PageItem{ID: page.ID, Title: title, URL: page.URL, LastEditedTime: page.LastEditedTime, HasChildren: page.HasChildren})
	}
	return PageList{Items: items, NextCursor: result.NextCursor, HasMore: result.HasMore}, nil
}

func (s *notionService) activeAccessToken(userID uint) (string, error) {
	binding, err := s.bindingRepo.FindByUserID(userID)
	if err != nil {
		return "", fmt.Errorf("find notion binding: %w", err)
	}
	if binding == nil || binding.Status != "active" {
		return "", bizerrors.ErrNotionNotConnected
	}
	return s.decryptBinding(binding)
}

func (s *notionService) decryptBinding(binding *entity.NotionBinding) (string, error) {
	if len(s.encryptionKey) != 32 {
		return "", bizerrors.ErrNotionAuthExpired
	}
	token, err := utils.Decrypt(binding.AccessTokenEncrypted, s.encryptionKey)
	if err != nil || strings.TrimSpace(token) == "" {
		return "", bizerrors.ErrNotionAuthExpired
	}
	return token, nil
}

func mapNotionError(err error) error {
	if errors.Is(err, notion.ErrAuthExpired) || errors.Is(err, notion.ErrAccessDenied) {
		return bizerrors.ErrNotionAuthExpired
	}
	if errors.Is(err, notion.ErrRateLimited) {
		return bizerrors.ErrNotionRateLimited
	}
	if errors.Is(err, context.Canceled) {
		return bizerrors.ErrNotionImportCancelled
	}
	return bizerrors.ErrNotionContentFailed
}
