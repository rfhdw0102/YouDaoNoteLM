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
	"YoudaoNoteLm/internal/service/external/asr"
	"YoudaoNoteLm/internal/service/external/embedding"
	"YoudaoNoteLm/internal/service/external/search"
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
// 策略：发起真实的测试搜索请求验证配置有效性
func (h *ConfigHealthChecker) testSearch(config *entity.UserConfig) *HealthCheckResult {
	sc := external.NewServiceConfigFromEntity(
		config.Provider, config.APIURL, config.APIKey, config.Model, config.ExtraConfig)

	// 通过 Registry 创建搜索引擎实例
	engineInterface, err := h.registry.Create("search", config.Provider, sc)
	if err != nil {
		return &HealthCheckResult{
			Healthy: false,
			Message: "配置格式错误",
			Detail:  err.Error(),
		}
	}

	// 类型断言为 SearchEngine
	engine, ok := engineInterface.(search.SearchEngine)
	if !ok {
		return &HealthCheckResult{
			Healthy: false,
			Message: "搜索引擎类型断言失败",
		}
	}

	// 发起真实的测试搜索请求
	_, err = engine.Search("test connectivity", 1)
	if err != nil {
		return &HealthCheckResult{
			Healthy: false,
			Message: "搜索 API 连接失败",
			Detail:  err.Error(),
		}
	}

	return &HealthCheckResult{
		Healthy: true,
		Message: fmt.Sprintf("搜索 API 连通正常（%s）", config.Provider),
	}
}

// testASR 测试 ASR 配置
// 策略：验证配置格式 + 验证 API 凭证有效性
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

	// Whisper 类型：使用 /models 端点验证 API Key
	if config.Provider == "whisper" || config.Provider == "openai" {
		apiURL := config.APIURL
		if apiURL == "" {
			apiURL = "https://api.openai.com/v1"
		}
		if config.APIKey == "" {
			return &HealthCheckResult{Healthy: false, Message: "API Key 为空"}
		}

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
				Detail:  fmt.Sprintf("HTTP %d", resp.StatusCode),
			}
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &HealthCheckResult{Healthy: true, Message: "ASR API 连通正常，API Key 有效"}
		}
		if resp.StatusCode == 429 {
			return &HealthCheckResult{Healthy: true, Message: "配置正确（当前被限流，但连通性正常）"}
		}

		return &HealthCheckResult{
			Healthy: false,
			Message: fmt.Sprintf("ASR API 返回异常状态码 %d", resp.StatusCode),
		}
	}

	// 阿里云 NLS：验证 AccessKey 凭证
	if config.Provider == "aliyun_nls" {
		// 解析 extra_config
		var extraConfig map[string]interface{}
		if config.ExtraConfig != "" {
			if err := json.Unmarshal([]byte(config.ExtraConfig), &extraConfig); err != nil {
				return &HealthCheckResult{
					Healthy: false,
					Message: "阿里云 ASR 扩展配置格式错误",
					Detail:  err.Error(),
				}
			}
		}

		accessKeyID := config.APIKey
		if v, ok := extraConfig["access_key_id"].(string); ok && v != "" {
			accessKeyID = v
		}
		accessKeySecret, _ := extraConfig["access_key_secret"].(string)
		appKey, _ := extraConfig["app_key"].(string)

		if accessKeyID == "" || accessKeySecret == "" || appKey == "" {
			return &HealthCheckResult{
				Healthy: false,
				Message: "阿里云 ASR 配置不完整",
				Detail:  "access_key_id, access_key_secret, app_key 均为必填",
			}
		}

		// 创建 SDK 客户端，验证凭证格式（构造失败返回 nil）
		client := asr.NewAliyunNLSASRService(accessKeyID, accessKeySecret, appKey)
		if client == nil {
			return &HealthCheckResult{
				Healthy: false,
				Message: "创建阿里云 ASR 客户端失败",
				Detail:  "AccessKey 凭证格式非法或 SDK 初始化失败",
			}
		}

		// 真实验证凭证：发送轻量请求到阿里云触发鉴权
		if err := asr.ValidateAliyunCredentials(client); err != nil {
			return &HealthCheckResult{
				Healthy: false,
				Message: "阿里云 ASR 凭证验证失败",
				Detail:  err.Error(),
			}
		}

		return &HealthCheckResult{
			Healthy: true,
			Message: "阿里云 ASR 配置验证通过，AccessKey 凭证有效",
		}
	}

	// 其他 ASR 服务：仅验证配置格式（未实现真实连通性检查，不轻易放行）
	return &HealthCheckResult{
		Healthy: false,
		Message: fmt.Sprintf("ASR 服务商 %s 暂不支持测试连接", config.Provider),
	}
}

// testEmbedding 测试 Embedding 配置
// 策略：实际调用 embedding API，验证 API Key 有效性 + 向量维度是否匹配
func (h *ConfigHealthChecker) testEmbedding(config *entity.UserConfig) *HealthCheckResult {
	apiURL := h.resolveAPIURL(config.Provider, config.APIURL)
	if apiURL == "" {
		return &HealthCheckResult{Healthy: false, Message: "API 地址为空"}
	}
	if config.APIKey == "" {
		return &HealthCheckResult{Healthy: false, Message: "API Key 为空"}
	}
	if config.Model == "" {
		return &HealthCheckResult{Healthy: false, Message: "模型名称未配置"}
	}

	// 获取配置的维度
	configuredDimensions := 0
	if config.Dimensions != nil && *config.Dimensions > 0 {
		configuredDimensions = *config.Dimensions
	}

	// 根据 provider 类型创建 embedding 服务
	embedService, err := h.createEmbeddingService(config)
	if err != nil {
		return &HealthCheckResult{
			Healthy: false,
			Message: "创建 Embedding 服务失败",
			Detail:  err.Error(),
		}
	}

	// 实际调用 embedding API，发送测试文本
	testText := "Hello, this is a connectivity test."
	vector, err := embedService.Embed(testText)
	if err != nil {
		// 解析错误信息，提供有用的反馈
		errMsg := err.Error()

		// 常见错误处理
		if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || strings.Contains(errMsg, "Unauthorized") {
			return &HealthCheckResult{
				Healthy: false,
				Message: "API Key 无效或无权限",
				Detail:  errMsg,
			}
		}
		if strings.Contains(errMsg, "429") || strings.Contains(errMsg, "rate_limit") {
			return &HealthCheckResult{
				Healthy: true,
				Message: "配置正确（当前被限流，但连通性正常）",
				Detail:  errMsg,
			}
		}
		if strings.Contains(errMsg, "model_not_found") || strings.Contains(errMsg, "does not exist") {
			return &HealthCheckResult{
				Healthy: false,
				Message: fmt.Sprintf("模型 %s 不存在", config.Model),
				Detail:  errMsg,
			}
		}
		if strings.Contains(errMsg, "dimension") || strings.Contains(errMsg, "dimensions") {
			// 尝试从错误信息中解析支持的维度列表
			// 格式示例: "its value should be in [64, 128, 256, 512, 768, 1024, 1536, 2048, 3072]"
			supportedDims := parseSupportedDimensions(errMsg)
			if supportedDims != "" {
				return &HealthCheckResult{
					Healthy: false,
					Message: fmt.Sprintf("向量维度错误，该向量模型支持 %s 维度，请更改后重试", supportedDims),
				}
			}
			return &HealthCheckResult{
				Healthy: false,
				Message: "向量维度配置错误",
				Detail:  errMsg,
			}
		}

		return &HealthCheckResult{
			Healthy: false,
			Message: "Embedding API 调用失败",
			Detail:  errMsg,
		}
	}

	// 验证返回的向量维度
	actualDimensions := len(vector)
	if configuredDimensions > 0 && actualDimensions != configuredDimensions {
		return &HealthCheckResult{
			Healthy: false,
			Message: fmt.Sprintf("向量维度不匹配: 配置 %d 维, 模型返回 %d 维", configuredDimensions, actualDimensions),
			Detail:  fmt.Sprintf("请将向量维度修改为 %d", actualDimensions),
		}
	}

	// 测试通过
	msg := fmt.Sprintf("Embedding API 连通正常（模型: %s, 维度: %d）", config.Model, actualDimensions)
	return &HealthCheckResult{Healthy: true, Message: msg}
}

// parseSupportedDimensions 从错误信息中解析支持的维度列表
// 示例输入: "its value should be in [64, 128, 256, 512, 768, 1024, 1536, 2048, 3072]"
// 示例输出: "64、128、256、512、768、1024、1536、2048、3072"
func parseSupportedDimensions(errMsg string) string {
	// 查找 [ 和 ] 之间的内容
	start := strings.Index(errMsg, "[")
	end := strings.Index(errMsg, "]")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	dimsStr := errMsg[start+1 : end]
	if dimsStr == "" {
		return ""
	}
	// 将逗号替换为顿号，去除空格
	dims := strings.ReplaceAll(dimsStr, ",", "、")
	dims = strings.ReplaceAll(dims, " ", "")
	return dims
}

// createEmbeddingService 根据配置创建 embedding 服务实例
func (h *ConfigHealthChecker) createEmbeddingService(config *entity.UserConfig) (embedding.EmbeddingService, error) {
	dimensions := 2048 // 默认维度
	if config.Dimensions != nil && *config.Dimensions > 0 {
		dimensions = *config.Dimensions
	}

	apiURL := h.resolveAPIURL(config.Provider, config.APIURL)

	// 火山引擎使用 Ark 原生接口
	if config.Provider == "volcengine" || config.Provider == "doubao" {
		return embedding.NewArkEmbeddingService(config.APIKey, config.Model, "multi_modal_api", apiURL, dimensions)
	}

	// 其他 provider 使用 OpenAI 兼容接口
	return embedding.NewOpenAIEmbeddingService(config.APIKey, config.Model, apiURL, dimensions)
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
		"doubao":     "https://ark.cn-beijing.volces.com/api/v3",
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
