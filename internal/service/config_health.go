package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/service/external"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Healthy   bool   `json:"healthy"`          // 是否健康
	Message   string `json:"message"`          // 结果描述
	LatencyMs int64  `json:"latency_ms"`       // 检查耗时（毫秒）
	Detail    string `json:"detail,omitempty"` // 详细信息
}

// ConfigHealthChecker 配置健康检查器
type ConfigHealthChecker struct {
	registry *external.Registry
}

// NewConfigHealthChecker 创建配置健康检查器
func NewConfigHealthChecker() *ConfigHealthChecker {
	return &ConfigHealthChecker{
		registry: external.GetGlobalRegistry(),
	}
}

// TestConfig 测试配置连通性
// configType: "llm", "search", "asr", "embedding"
func (h *ConfigHealthChecker) TestConfig(configType string, config *entity.UserConfig) *HealthCheckResult {
	start := time.Now()

	var result *HealthCheckResult

	switch configType {
	case "llm":
		result = h.testLLM(config)
	case "search":
		result = h.testSearch(config)
	case "asr":
		result = h.testASR(config)
	case "embedding":
		result = h.testEmbedding(config)
	default:
		result = &HealthCheckResult{
			Healthy: false,
			Message: fmt.Sprintf("不支持的配置类型: %s", configType),
		}
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	return result
}

// testLLM 测试 LLM 配置
// 策略：调用 /models 端点验证 API Key（不调用模型，快速）
func (h *ConfigHealthChecker) testLLM(config *entity.UserConfig) *HealthCheckResult {
	if config.Provider == "anthropic" {
		return h.testLLMAnthropic(config)
	}
	return h.testLLMOpenAICompatible(config)
}

// testLLMOpenAICompatible 测试 OpenAI 兼容的 LLM 服务
// 使用 GET /models 端点，只需验证 API Key，不调用模型推理
func (h *ConfigHealthChecker) testLLMOpenAICompatible(config *entity.UserConfig) *HealthCheckResult {
	apiURL := h.resolveAPIURL(config.Provider, config.APIURL)
	if apiURL == "" {
		return &HealthCheckResult{Healthy: false, Message: "API 地址为空"}
	}
	if config.APIKey == "" {
		return &HealthCheckResult{Healthy: false, Message: "API Key 为空"}
	}

	// 用 /models 端点验证，比 /chat/completions 快得多
	url := strings.TrimRight(apiURL, "/") + "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &HealthCheckResult{Healthy: false, Message: "创建请求失败"}
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &HealthCheckResult{Healthy: false, Message: "连接超时（5s）", Detail: "API 服务不可达"}
		}
		return &HealthCheckResult{Healthy: false, Message: "连接失败", Detail: err.Error()}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("关闭 HTTP 响应体失败", zap.String("url", url), zap.Error(err))
		}
	}()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return &HealthCheckResult{Healthy: false, Message: "读取响应失败", Detail: readErr.Error()}
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &HealthCheckResult{
			Healthy: false,
			Message: "API Key 无效或无权限",
			Detail:  fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}
	if resp.StatusCode == 429 {
		return &HealthCheckResult{
			Healthy: true,
			Message: "配置正确（当前被限流，但连通性正常）",
		}
	}

	// 检查是否返回了模型列表
	var modelsResp struct {
		Data []interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &modelsResp); err == nil {
		// 成功获取模型列表
		msg := fmt.Sprintf("API 连通正常，共 %d 个可用模型", len(modelsResp.Data))
		if config.Model != "" {
			// 检查配置的模型是否在列表中
			modelFound := false
			for _, m := range modelsResp.Data {
				if modelMap, ok := m.(map[string]interface{}); ok {
					if id, ok := modelMap["id"].(string); ok && id == config.Model {
						modelFound = true
						break
					}
				}
			}
			if modelFound {
				msg = fmt.Sprintf("API 连通正常，模型 %s 可用", config.Model)
			} else {
				return &HealthCheckResult{
					Healthy: false,
					Message: fmt.Sprintf("API 连通正常，但模型 %s 不存在", config.Model),
					Detail:  fmt.Sprintf("可用模型数: %d", len(modelsResp.Data)),
				}
			}
		}
		return &HealthCheckResult{Healthy: true, Message: msg}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &HealthCheckResult{Healthy: true, Message: "API 连通正常"}
	}

	// /models 不支持时，回退到简单可达性检查
	if resp.StatusCode == 404 || resp.StatusCode == 405 {
		return h.fallbackReachabilityCheck(apiURL)
	}

	return &HealthCheckResult{
		Healthy: false,
		Message: fmt.Sprintf("API 返回异常状态码 %d", resp.StatusCode),
		Detail:  truncate(string(respBody), 200),
	}
}

// fallbackReachabilityCheck 回退的可达性检查
func (h *ConfigHealthChecker) fallbackReachabilityCheck(apiURL string) *HealthCheckResult {
	if err := checkHTTPReachable(apiURL, 3*time.Second); err != nil {
		return &HealthCheckResult{Healthy: false, Message: "API 地址不可达", Detail: err.Error()}
	}
	return &HealthCheckResult{Healthy: true, Message: "API 地址可达（无法验证 API Key）"}
}

// testLLMAnthropic 测试 Anthropic Claude 服务
// Anthropic 没有 /models 端点，使用轻量级检查
func (h *ConfigHealthChecker) testLLMAnthropic(config *entity.UserConfig) *HealthCheckResult {
	apiURL := config.APIURL
	if apiURL == "" {
		apiURL = "https://api.anthropic.com"
	}
	if config.APIKey == "" {
		return &HealthCheckResult{Healthy: false, Message: "API Key 为空"}
	}

	// Anthropic 没有公开的 /models 端点，直接检查 API 格式和可达性
	url := strings.TrimRight(apiURL, "/") + "/v1/messages"

	// 发送一个故意缺字段的请求，验证 API Key 和端点
	// Anthropic 会返回 400（参数错误）表示 Key 正确，401 表示 Key 错误
	reqBody := map[string]interface{}{
		"model":      "test",
		"max_tokens": 1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return &HealthCheckResult{Healthy: false, Message: "序列化请求体失败", Detail: err.Error()}
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return &HealthCheckResult{Healthy: false, Message: "创建请求失败"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &HealthCheckResult{Healthy: false, Message: "连接超时（5s）", Detail: "Claude API 不可达"}
		}
		return &HealthCheckResult{Healthy: false, Message: "连接失败", Detail: err.Error()}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("关闭 HTTP 响应体失败", zap.String("url", url), zap.Error(err))
		}
	}()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &HealthCheckResult{
			Healthy: false,
			Message: "API Key 无效或无权限",
			Detail:  fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	// 400 = 参数错误但 Key 正确（我们故意发了无效的 model）
	if resp.StatusCode == 400 {
		return &HealthCheckResult{
			Healthy: true,
			Message: "Claude API 连通正常，API Key 有效",
		}
	}

	if resp.StatusCode == 429 {
		return &HealthCheckResult{
			Healthy: true,
			Message: "配置正确（当前被限流，但连通性正常）",
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &HealthCheckResult{Healthy: true, Message: "Claude API 连通正常"}
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return &HealthCheckResult{Healthy: false, Message: "读取响应失败", Detail: readErr.Error()}
	}
	return &HealthCheckResult{
		Healthy: false,
		Message: fmt.Sprintf("Claude API 返回异常状态码 %d", resp.StatusCode),
		Detail:  truncate(string(respBody), 200),
	}
}

// testSearch 测试搜索配置
// 策略：只验证配置格式和 API 可达性，不发真实搜索请求
func (h *ConfigHealthChecker) testSearch(config *entity.UserConfig) *HealthCheckResult {
	sc := external.NewServiceConfigFromEntity(
		config.Provider, config.APIURL, config.APIKey, config.Model, config.ExtraConfig)

	// 通过 Registry 验证配置格式
	_, err := h.registry.Create("search", config.Provider, sc)
	if err != nil {
		return &HealthCheckResult{
			Healthy: false,
			Message: "配置格式错误",
			Detail:  err.Error(),
		}
	}

	// 检查 API 可达性
	apiURL := config.APIURL
	if apiURL == "" {
		// 从 provider 默认地址推断
		defaults := map[string]string{
			"searxng": "",
			"tavily":  "https://api.tavily.com",
			"serper":  "https://google.serper.dev",
			"bing":    "https://api.bing.microsoft.com",
			"bocha":   "https://api.bochaai.com",
		}
		apiURL = defaults[config.Provider]
	}

	if apiURL != "" {
		if err := checkHTTPReachable(apiURL, 3*time.Second); err != nil {
			return &HealthCheckResult{
				Healthy: false,
				Message: "搜索 API 地址不可达",
				Detail:  err.Error(),
			}
		}
	}

	return &HealthCheckResult{
		Healthy: true,
		Message: "搜索配置格式正确，API 可达",
	}
}

// testASR 测试 ASR 配置
// 策略：验证配置格式 + API 地址可达性
func (h *ConfigHealthChecker) testASR(config *entity.UserConfig) *HealthCheckResult {
	if config.Provider == "" {
		return &HealthCheckResult{Healthy: false, Message: "服务商为空"}
	}

	sc := external.NewServiceConfigFromEntity(
		config.Provider, config.APIURL, config.APIKey, config.Model, config.ExtraConfig)
	_, err := h.registry.Create("asr", config.Provider, sc)
	if err != nil {
		return &HealthCheckResult{
			Healthy: false,
			Message: "配置格式错误",
			Detail:  err.Error(),
		}
	}

	if config.APIURL != "" {
		if err := checkHTTPReachable(config.APIURL, 3*time.Second); err != nil {
			return &HealthCheckResult{
				Healthy: false,
				Message: "ASR 服务地址不可达",
				Detail:  err.Error(),
			}
		}
	}

	return &HealthCheckResult{
		Healthy: true,
		Message: "ASR 配置格式正确",
	}
}

// testEmbedding 测试 Embedding 配置
// 策略：用 /models 端点验证 API Key（与 LLM 共享同一套 API）
func (h *ConfigHealthChecker) testEmbedding(config *entity.UserConfig) *HealthCheckResult {
	apiURL := h.resolveAPIURL(config.Provider, config.APIURL)
	if apiURL == "" {
		return &HealthCheckResult{Healthy: false, Message: "API 地址为空"}
	}
	if config.APIKey == "" {
		return &HealthCheckResult{Healthy: false, Message: "API Key 为空"}
	}

	// Embedding 通常与 LLM 共享 API，用 /models 验证 Key
	url := strings.TrimRight(apiURL, "/") + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &HealthCheckResult{Healthy: false, Message: "创建请求失败"}
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &HealthCheckResult{Healthy: false, Message: "连接超时（5s）"}
		}
		return &HealthCheckResult{Healthy: false, Message: "连接失败", Detail: err.Error()}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("关闭 HTTP 响应体失败", zap.String("url", url), zap.Error(err))
		}
	}()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &HealthCheckResult{
			Healthy: false,
			Message: "API Key 无效或无权限",
		}
	}
	if resp.StatusCode == 429 {
		return &HealthCheckResult{Healthy: true, Message: "配置正确（当前被限流，但连通性正常）"}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		msg := "Embedding API 连通正常"
		if config.Model != "" {
			msg = fmt.Sprintf("API 连通正常（模型: %s）", config.Model)
		}
		return &HealthCheckResult{Healthy: true, Message: msg}
	}

	// /models 不支持时回退
	if resp.StatusCode == 404 || resp.StatusCode == 405 {
		return h.fallbackReachabilityCheck(apiURL)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return &HealthCheckResult{Healthy: false, Message: "读取响应失败", Detail: readErr.Error()}
	}
	return &HealthCheckResult{
		Healthy: false,
		Message: fmt.Sprintf("API 返回异常状态码 %d", resp.StatusCode),
		Detail:  truncate(string(respBody), 200),
	}
}

// resolveAPIURL 解析 API URL（provider 默认值）
func (h *ConfigHealthChecker) resolveAPIURL(provider, apiURL string) string {
	if apiURL != "" {
		return apiURL
	}
	defaults := map[string]string{
		"openai":     "https://api.openai.com/v1",
		"anthropic":  "https://api.anthropic.com",
		"deepseek":   "https://api.deepseek.com/v1",
		"zhipu":      "https://open.bigmodel.cn/api/paas/v4",
		"qwen":       "https://dashscope.aliyuncs.com/compatible-mode/v1",
		"baichuan":   "https://api.baichuan-ai.com/v1",
		"moonshot":   "https://api.moonshot.cn/v1",
		"minimax":    "https://api.minimax.chat/v1",
		"volcengine": "https://ark.cn-beijing.volces.com/api/v3",
	}
	if url, ok := defaults[provider]; ok {
		return url
	}
	return ""
}

// checkHTTPReachable 检查 HTTP 地址是否可达
func checkHTTPReachable(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("连接超时")
		}
		return checkTCPReachable(url, timeout)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("关闭 HTTP 响应体失败", zap.String("url", url), zap.Error(err))
		}
	}()
	return nil
}

// checkTCPReachable 检查 TCP 地址是否可达
func checkTCPReachable(rawURL string, timeout time.Duration) error {
	host := rawURL
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	if !strings.Contains(host, ":") {
		if strings.HasPrefix(rawURL, "https://") {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return fmt.Errorf("TCP 连接失败: %w", err)
	}
	if err := conn.Close(); err != nil {
		logger.Warn("关闭 TCP 连接失败", zap.String("host", host), zap.Error(err))
	}
	return nil
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
