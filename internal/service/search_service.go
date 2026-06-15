package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/cache"
	"YoudaoNoteLm/pkg/config"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type searchService struct {
	client         external.WebSearchClient
	userConfigRepo repository.UserConfigRepository
	cache          *cache.Cache
	bochaConfig    config.BochaConfig
}

type cachedSearchResponse struct {
	Response *SearchResponse `json:"response"`
}

// NewSearchService 创建统一搜索服务。
func NewSearchService(
	client external.WebSearchClient,
	userConfigRepo repository.UserConfigRepository,
	cacheClient *cache.Cache,
	bochaConfig config.BochaConfig,
) SearchService {
	return &searchService{
		client:         client,
		userConfigRepo: userConfigRepo,
		cache:          cacheClient,
		bochaConfig:    bochaConfig,
	}
}

func (s *searchService) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	resolved, query, err := s.prepareRequest(req)
	if err != nil {
		return nil, err
	}

	queryHash := hashQuery(query)
	cacheKey := s.buildCacheKey(resolved, query)
	if resp, ok := s.getCachedResponse(ctx, cacheKey); ok {
		resp.Cached = true
		if resp.Meta == nil {
			resp.Meta = map[string]any{}
		}
		resp.Meta["query_hash"] = queryHash
		resp.Meta["degraded"] = false
		return resp, nil
	}

	userCfg, providerCfg, quota, err := s.resolveProviderConfig(resolved.UserID)
	if err != nil {
		return nil, err
	}

	providerResp, err := s.client.Search(ctx, providerCfg, &external.SearchProviderRequest{
		Query:          query,
		Freshness:      resolved.Freshness,
		Count:          resolved.Count,
		NeedSummary:    resolved.NeedSummary,
		NeedContent:    resolved.NeedContent,
		Language:       resolved.Language,
		AllowedDomains: resolved.AllowedDomains,
		BlockedDomains: resolved.BlockedDomains,
		TraceID:        resolved.TraceID,
	})
	if err != nil {
		if degraded := s.tryDegrade(resolved, err, queryHash); degraded != nil {
			return degraded, nil
		}
		return nil, err
	}
	if len(providerResp.Results) == 0 {
		return nil, bizerrors.ErrSearchProviderEmptyResult
	}

	results := NormalizeSearchResults(providerResp.Results, resolved.NeedContent, resolved.AllowedDomains, resolved.BlockedDomains)
	if len(results) == 0 {
		return nil, bizerrors.ErrSearchNormalizedEmptyResult
	}

	summary := ""
	if resolved.NeedSummary {
		summary = firstNonEmpty(providerResp.Summary, BuildSearchSummary(results))
	}

	response := &SearchResponse{
		Query:    query,
		Provider: providerResp.Provider,
		Results:  results,
		Summary:  summary,
		Total:    len(results),
		Cached:   false,
		Quota:    quota,
		Meta: map[string]any{
			"query_hash":       queryHash,
			"provider_total":   providerResp.Total,
			"scene":            resolved.Scene,
			"degraded":         false,
			"normalized_query": query,
		},
	}

	if err := s.consumeQuota(userCfg, quota); err != nil {
		logger.Warn("consume search quota failed", zap.Uint("user_id", resolved.UserID), zap.Error(err))
	}
	if err := s.setCachedResponse(ctx, cacheKey, response); err != nil {
		logger.Warn("cache search response failed", zap.String("cache_key", cacheKey), zap.Error(err))
	}

	logger.Info("search completed",
		zap.Uint("user_id", resolved.UserID),
		zap.String("scene", string(resolved.Scene)),
		zap.String("provider", response.Provider),
		zap.String("query_hash", queryHash),
		zap.Int("result_count", len(results)),
	)

	return response, nil
}

func (s *searchService) SearchAndSummarize(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	cloned := cloneSearchRequest(req)
	cloned.NeedSummary = true
	return s.Search(ctx, cloned)
}

func (s *searchService) SearchForImport(ctx context.Context, req *SearchImportRequest) (*SearchImportResponse, error) {
	if req == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "搜索导入请求不能为空")
	}
	cloned := cloneSearchRequest(&req.SearchRequest)
	if cloned.Scene == "" {
		cloned.Scene = SearchSceneImport
	}
	resp, err := s.Search(ctx, cloned)
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(resp.Results))
	for _, result := range resp.Results {
		urls = append(urls, result.URL)
	}

	return &SearchImportResponse{
		Query:    resp.Query,
		Provider: resp.Provider,
		Results:  resp.Results,
		URLs:     urls,
		Total:    resp.Total,
		Cached:   resp.Cached,
		Quota:    resp.Quota,
		Meta:     resp.Meta,
	}, nil
}

func (s *searchService) prepareRequest(req *SearchRequest) (*SearchRequest, string, error) {
	cloned := cloneSearchRequest(req)
	if cloned.Count <= 0 {
		cloned.Count = max(1, s.bochaConfig.DefaultCount)
	}
	maxCount := s.bochaConfig.MaxCount
	if maxCount <= 0 {
		maxCount = 10
	}
	if cloned.Count > maxCount {
		cloned.Count = maxCount
	}
	if strings.TrimSpace(cloned.Freshness) == "" {
		cloned.Freshness = "noLimit"
	}

	query, err := BuildSearchQuery(cloned)
	if err != nil {
		return nil, "", err
	}
	return cloned, query, nil
}

func (s *searchService) resolveProviderConfig(userID uint) (*entity.UserConfig, external.SearchProviderConfig, *SearchQuota, error) {
	providerCfg := external.SearchProviderConfig{
		BaseURL: strings.TrimSpace(s.bochaConfig.BaseURL),
		APIKey:  strings.TrimSpace(s.bochaConfig.APIKey),
		Timeout: time.Duration(max(1, s.bochaConfig.TimeoutSeconds)) * time.Second,
	}

	var quota *SearchQuota
	var userCfg *entity.UserConfig
	if userID == 0 || s.userConfigRepo == nil {
		if providerCfg.APIKey == "" || providerCfg.BaseURL == "" {
			return nil, external.SearchProviderConfig{}, nil, bizerrors.ErrSearchProviderNotConfigured
		}
		return nil, providerCfg, nil, nil
	}

	cfg, err := s.userConfigRepo.FindByUserAndType(userID, "search")
	if err != nil {
		return nil, external.SearchProviderConfig{}, nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "读取用户搜索配置失败", err)
	}
	userCfg = cfg
	if userCfg != nil {
		if !userCfg.Enabled {
			return nil, external.SearchProviderConfig{}, nil, bizerrors.New(bizerrors.CodeSearchProviderNotConfigured, "当前用户未启用联网搜索")
		}
		if provider := strings.TrimSpace(strings.ToLower(userCfg.Provider)); provider != "" && provider != "bocha" {
			return nil, external.SearchProviderConfig{}, nil, bizerrors.New(bizerrors.CodeSearchProviderNotConfigured, "当前仅支持 bocha 搜索 provider")
		}
		if value := strings.TrimSpace(userCfg.APIURL); value != "" {
			providerCfg.BaseURL = value
		}
		if value := strings.TrimSpace(userCfg.APIKey); value != "" {
			providerCfg.APIKey = value
		}
		if err := s.resetQuotaIfNeeded(userCfg); err != nil {
			return nil, external.SearchProviderConfig{}, nil, err
		}
		quota = buildSearchQuota(userCfg)
		if userCfg.DailyQuota != nil && userCfg.QuotaUsed >= *userCfg.DailyQuota {
			return nil, external.SearchProviderConfig{}, quota, bizerrors.ErrSearchQuotaExhausted
		}
	}

	if providerCfg.APIKey == "" || providerCfg.BaseURL == "" {
		return nil, external.SearchProviderConfig{}, quota, bizerrors.ErrSearchProviderNotConfigured
	}
	return userCfg, providerCfg, quota, nil
}

func (s *searchService) resetQuotaIfNeeded(cfg *entity.UserConfig) error {
	if cfg == nil || cfg.QuotaResetAt == nil || s.userConfigRepo == nil {
		return nil
	}
	now := time.Now()
	if cfg.QuotaResetAt.After(now) {
		return nil
	}
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	cfg.QuotaUsed = 0
	cfg.QuotaResetAt = &next
	return s.userConfigRepo.Update(cfg)
}

func (s *searchService) consumeQuota(cfg *entity.UserConfig, quota *SearchQuota) error {
	if cfg == nil || cfg.DailyQuota == nil || s.userConfigRepo == nil {
		return nil
	}
	cfg.QuotaUsed++
	if cfg.QuotaResetAt == nil {
		next := time.Now().Add(24 * time.Hour)
		cfg.QuotaResetAt = &next
	}
	if err := s.userConfigRepo.Update(cfg); err != nil {
		return err
	}
	if quota != nil {
		quota.Used = cfg.QuotaUsed
		if quota.DailyQuota != nil {
			remaining := *quota.DailyQuota - quota.Used
			if remaining < 0 {
				remaining = 0
			}
			quota.Remaining = &remaining
		}
		quota.ResetAt = cfg.QuotaResetAt
	}
	return nil
}

func buildSearchQuota(cfg *entity.UserConfig) *SearchQuota {
	if cfg == nil {
		return nil
	}
	quota := &SearchQuota{
		DailyQuota: cfg.DailyQuota,
		Used:       cfg.QuotaUsed,
		ResetAt:    cfg.QuotaResetAt,
	}
	if cfg.DailyQuota != nil {
		remaining := *cfg.DailyQuota - cfg.QuotaUsed
		if remaining < 0 {
			remaining = 0
		}
		quota.Remaining = &remaining
	}
	return quota
}

func (s *searchService) tryDegrade(req *SearchRequest, err error, queryHash string) *SearchResponse {
	if req == nil || !req.AllowDegrade {
		return nil
	}
	if req.Scene != SearchSceneGeneration && req.Scene != SearchSceneChat {
		return nil
	}

	logger.Warn("search degraded to local-only",
		zap.Uint("user_id", req.UserID),
		zap.String("scene", string(req.Scene)),
		zap.String("query_hash", queryHash),
		zap.Error(err),
	)

	return &SearchResponse{
		Query:    req.Query,
		Provider: "bocha",
		Results:  []SearchResult{},
		Total:    0,
		Cached:   false,
		Meta: map[string]any{
			"degraded":   true,
			"query_hash": queryHash,
			"reason":     err.Error(),
		},
	}
}

func (s *searchService) buildCacheKey(req *SearchRequest, query string) string {
	return fmt.Sprintf(
		"search:bocha:%s:%s:%s:%d:%t:%t",
		req.Scene,
		hashQuery(query),
		req.Freshness,
		req.Count,
		req.NeedSummary,
		req.NeedContent,
	)
}

func (s *searchService) getCachedResponse(ctx context.Context, key string) (*SearchResponse, bool) {
	if s.cache == nil || strings.TrimSpace(key) == "" {
		return nil, false
	}

	var payload cachedSearchResponse
	if err := s.cache.Get(ctx, key, &payload); err != nil || payload.Response == nil {
		return nil, false
	}
	return payload.Response, true
}

func (s *searchService) setCachedResponse(ctx context.Context, key string, resp *SearchResponse) error {
	if s.cache == nil || resp == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	ttlSeconds := s.bochaConfig.CacheTTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return s.cache.Set(ctx, key, &cachedSearchResponse{Response: resp}, time.Duration(ttlSeconds)*time.Second)
}

func cloneSearchRequest(req *SearchRequest) *SearchRequest {
	if req == nil {
		return &SearchRequest{}
	}
	cloned := *req
	cloned.AllowedDomains = append([]string(nil), req.AllowedDomains...)
	cloned.BlockedDomains = append([]string(nil), req.BlockedDomains...)
	return &cloned
}

func hashQuery(query string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(query)))
	return hex.EncodeToString(sum[:])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
