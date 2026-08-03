package search

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	agentTools "YoudaoNoteLm/internal/agent/tools"
	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/config"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const maxAgentRounds = 2

// extractBizError 从错误链中提取 BizError（兼容 fmt.Errorf %w 包装），提取不到返回 nil
func extractBizError(err error) *bizerrors.BizError {
	if err == nil {
		return nil
	}
	var bizErr *bizerrors.BizError
	if errors.As(err, &bizErr) {
		return bizErr
	}
	return nil
}

// jsonBlockRegexp 匹配 ```json ... ``` 或 ``` ... ``` 代码块
var jsonBlockRegexp = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```")

// jsonObjRegexp 兜底：从文本中提取第一个 JSON 对象
var jsonObjRegexp = regexp.MustCompile(`(?s)\{.*\}`)

// verifyFinalResultCount 硬约束（结果检查）：从最终回复内容中解析 JSON 并校验 results 数量是否为 10
// 仅记录告警，不修改内容；流式场景下内容已发出，此处用于事后排查
func verifyFinalResultCount(content string) {
	if content == "" {
		return
	}

	jsonStr := ""
	if match := jsonBlockRegexp.FindStringSubmatch(content); match != nil {
		jsonStr = match[1]
	} else if match := jsonObjRegexp.FindStringSubmatch(content); match != nil {
		jsonStr = match[0] // jsonObjRegexp 无捕获组，match[0] 为完整 {...} 匹配
	} else {
		jsonStr = content
	}

	var parsed struct {
		Results []interface{} `json:"results"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		logger.Warn("结果校验：JSON 解析失败，跳过数量校验",
			zap.Error(err),
		)
		return
	}

	got := len(parsed.Results)
	if got != requiredResultCount {
		logger.Warn("结果校验：最终 results 数量不符合要求",
			zap.Int("got", got),
			zap.Int("required", requiredResultCount),
		)
	} else {
		logger.Info("结果校验：results 数量校验通过", zap.Int("count", got))
	}
}

// SearchAgent 搜索 Agent（基于 Eino 框架）
type SearchAgent struct {
	configService service.ConfigService
	importer      service.ImporterService
	limiter       *userLimiter // per-user 并发限流（eino 未提供，自行实现）
}

// NewSearchAgent 创建搜索 Agent
func NewSearchAgent(
	configService service.ConfigService,
	importer service.ImporterService,
) *SearchAgent {
	maxConcurrent := 1
	if cfg := config.Get(); cfg != nil && cfg.Agent.MaxConcurrent > 0 {
		maxConcurrent = cfg.Agent.MaxConcurrent
	}
	return &SearchAgent{
		configService: configService,
		importer:      importer,
		limiter:       newUserLimiter(maxConcurrent), // per-user 并发上限（可配置，默认 1）
	}
}

// agentRunParams 返回运行参数（config 零值时用代码内默认值，无需在 yaml 配置）
func (a *SearchAgent) agentRunParams() (maxIter, importMaxIter int, execTimeout, execImportTimeout, cancelTimeout time.Duration) {
	maxIter = 4
	importMaxIter = 5
	execTimeout = 3 * time.Minute
	execImportTimeout = 5 * time.Minute
	cancelTimeout = 5 * time.Second
	if cfg := config.Get(); cfg != nil {
		ac := cfg.Agent
		if ac.MaxIterations > 0 {
			maxIter = ac.MaxIterations
			importMaxIter = ac.MaxIterations + 1
		}
		if ac.ExecuteTimeout > 0 {
			execTimeout = ac.ExecuteTimeout
		}
		if ac.ExecuteWithImportTimeout > 0 {
			execImportTimeout = ac.ExecuteWithImportTimeout
		}
		if ac.CancelTimeout > 0 {
			cancelTimeout = ac.CancelTimeout
		}
	}
	return
}

// createChatModel 根据用户配置创建 ToolCallingChatModel（支持多 Provider）
func (a *SearchAgent) createChatModel(ctx context.Context, userID uint) (model.ToolCallingChatModel, error) {
	cfg, err := a.configService.GetUserLLMConfig(userID)
	if err != nil {
		return nil, err
	}
	return llm.NewToolCallingChatModel(ctx, cfg)
}

// createTools 创建搜索 Agent 的工具列表（仅 web_search，用户交互模式）
func (a *SearchAgent) createTools() ([]tool.BaseTool, error) {
	tools := make([]tool.BaseTool, 0, 1)

	webSearchTool, err := NewWebSearchTool(a.configService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 web_search 工具失败", err)
	}
	tools = append(tools, webSearchTool)

	return tools, nil
}

// createToolsWithImport 创建搜索 Agent 的工具列表（web_search + import_document，自动导入模式）
func (a *SearchAgent) createToolsWithImport() ([]tool.BaseTool, error) {
	tools := make([]tool.BaseTool, 0, 2)

	webSearchTool, err := NewWebSearchTool(a.configService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 web_search 工具失败", err)
	}
	tools = append(tools, webSearchTool)

	// 统一导入工具（替代旧的 import_urls）；search agent 只用 url 来源，不依赖 youdao
	importDocTool, err := agentTools.NewImportDocumentTool(nil, a.importer)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 import_document 工具失败", err)
	}
	tools = append(tools, importDocTool)

	return tools, nil
}

// createAgent 创建 Eino Agent（用户交互模式：只搜索不自动导入）
func (a *SearchAgent) createAgent(ctx context.Context, userID uint) (*adk.ChatModelAgent, error) {
	chatModel, err := a.createChatModel(ctx, userID)
	if err != nil {
		return nil, err
	}

	tools, err := a.createTools()
	if err != nil {
		return nil, err
	}

	maxIter, _, _, _, _ := a.agentRunParams()
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "SearchAgent",
		Description: "网络搜索助手，帮助用户搜索和分析网络内容",
		Instruction: SearchSystemPrompt,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
		MaxIterations:    maxIter, // 2 轮搜索 + 最终回复 + 余量
		ModelRetryConfig: buildRetryConfig(),
		Handlers:         []adk.ChatModelAgentMiddleware{newMetricsHandler()},
	})
}

// createAgentWithImport 创建 Eino Agent（自动导入模式：搜索并自动导入）
func (a *SearchAgent) createAgentWithImport(ctx context.Context, userID uint) (*adk.ChatModelAgent, error) {
	chatModel, err := a.createChatModel(ctx, userID)
	if err != nil {
		return nil, err
	}

	tools, err := a.createToolsWithImport()
	if err != nil {
		return nil, err
	}

	_, importMaxIter, _, _, _ := a.agentRunParams()
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "SearchAndImportAgent",
		Description: "网络搜索助手，帮助用户搜索并自动导入网络内容",
		Instruction: SearchAndImportSystemPrompt,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
		MaxIterations:    importMaxIter, // 2 轮搜索 + 导入 + 最终回复 + 余量
		ModelRetryConfig: buildRetryConfig(),
		Handlers:         []adk.ChatModelAgentMiddleware{newMetricsHandler()},
	})
}

// buildRetryConfig 构造 LLM 重试配置（基于 eino ModelRetryConfig，替代手写重试包装）
// 仅对返回 error 的 LLM 调用重试，业务错误不在此路径。
func buildRetryConfig() *adk.ModelRetryConfig {
	return &adk.ModelRetryConfig{
		MaxRetries: 2,
		ShouldRetry: func(ctx context.Context, retryCtx *adk.RetryContext) *adk.RetryDecision {
			if retryCtx.Err != nil {
				return &adk.RetryDecision{Retry: true}
			}
			return &adk.RetryDecision{Retry: false}
		},
	}
}

// Execute 执行搜索任务（非流式）
func (a *SearchAgent) Execute(ctx context.Context, userID, notebookID uint, task string) (*service.SearchAgentResult, error) {
	ctx = WithUserID(ctx, userID)
	ctx = WithNotebookID(ctx, notebookID)
	ctx = agentTools.WithUserID(ctx, userID)
	ctx = agentTools.WithNotebookID(ctx, notebookID)

	_, _, execTimeout, _, cancelTimeout := a.agentRunParams()
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	// per-user 限流（eino 未提供，自行实现）
	release, err := a.limiter.acquire(userID)
	if err != nil {
		return nil, err
	}
	defer release()

	agent, err := a.createAgent(ctx, userID)
	if err != nil {
		logger.Error("创建搜索 Agent 失败",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 中断传播：eino 的 iter.Next() 不响应 ctx，必须用 adk.WithCancel() 才能让迭代器在客户端断开时退出
	cancelOpt, cancelFn := adk.WithCancel()
	go func() {
		<-ctx.Done()
		cancelFn(adk.WithAgentCancelMode(adk.CancelAfterChatModel), adk.WithRecursive(), adk.WithAgentCancelTimeout(cancelTimeout))
	}()

	searchRounds := 0
	totalToolCalls := 0
	var finalContent string

	iter := runner.Query(ctx, task, cancelOpt)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			if ctx.Err() != nil {
				logger.Error("Agent 执行超时", zap.Error(ctx.Err()))
				return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "搜索超时，请稍后重试", ctx.Err())
			}
			logger.Error("Agent 执行错误", zap.Error(event.Err))
			// 保留工具返回的业务错误码（如搜索引擎未配置、API Key 无效等），便于前端精确提示
			if bizErr := extractBizError(event.Err); bizErr != nil {
				return nil, bizErr
			}
			return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "Agent 执行失败", event.Err)
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			logger.Warn("获取消息失败", zap.Error(err))
			continue
		}

		// 统计搜索轮数
		if event.Output.MessageOutput.ToolName == "web_search" {
			searchRounds++
		}

		// 获取最终内容（assistant 消息且无工具调用）
		if msg.Role == schema.Assistant && len(msg.ToolCalls) == 0 {
			finalContent = msg.Content
		}

		// 工具调用计数
		if len(msg.ToolCalls) > 0 {
			totalToolCalls++
			if totalToolCalls > maxAgentRounds {
				logger.Warn("达到最大工具调用轮数，强制结束",
					zap.Int("maxRounds", maxAgentRounds),
					zap.Int("searchRounds", searchRounds),
				)
				if finalContent == "" {
					finalContent = "搜索已完成，但达到轮数限制。以下是已搜索到的结果。"
				}
				break
			}
		}
	}

	logger.Info("Agent 执行完成",
		zap.Int("searchRounds", searchRounds),
		zap.Int("contentLength", len(finalContent)),
	)

	// 硬约束（结果检查）：校验最终 results 数量是否为 10
	verifyFinalResultCount(finalContent)

	return &service.SearchAgentResult{
		Content:      finalContent,
		SearchRounds: searchRounds,
	}, nil
}

// ExecuteStream 执行搜索任务（流式返回）
func (a *SearchAgent) ExecuteStream(ctx context.Context, userID, notebookID uint, task string) <-chan *service.SearchAgentEvent {
	eventCh := make(chan *service.SearchAgentEvent, 16)

	go func() {
		defer close(eventCh)

		ctx = WithUserID(ctx, userID)
		ctx = WithNotebookID(ctx, notebookID)
		ctx = agentTools.WithUserID(ctx, userID)
		ctx = agentTools.WithNotebookID(ctx, notebookID)

		_, _, execTimeout, _, cancelTimeout := a.agentRunParams()
		ctx, cancel := context.WithTimeout(ctx, execTimeout)
		defer cancel()

		// send helper：ctx 取消时停止发送，防止缓冲满+客户端断开导致 goroutine 泄漏
		send := func(e *service.SearchAgentEvent) bool {
			select {
			case eventCh <- e:
				return true
			case <-ctx.Done():
				return false
			}
		}

		// per-user 限流
		release, err := a.limiter.acquire(userID)
		if err != nil {
			event := &service.SearchAgentEvent{Type: "error", Error: err.Error()}
			if bizErr, ok := err.(*bizerrors.BizError); ok {
				event.ErrorCode = bizErr.Code
				event.Error = bizErr.Message
			}
			send(event)
			return
		}
		defer release()

		agent, err := a.createAgent(ctx, userID)
		if err != nil {
			logger.Error("创建搜索 Agent 失败",
				zap.Uint("user_id", userID),
				zap.Error(err),
			)
			event := &service.SearchAgentEvent{Type: "error", Error: err.Error()}
			if bizErr, ok := err.(*bizerrors.BizError); ok {
				event.ErrorCode = bizErr.Code
				event.Error = bizErr.Message // 使用友好的错误消息，而不是包含错误码的完整信息
			}
			send(event)
			return
		}

		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
		})

		// 中断传播：eino 的 iter.Next() 不响应 ctx，必须用 adk.WithCancel() 才能让迭代器在客户端断开时退出
		cancelOpt, cancelFn := adk.WithCancel()
		go func() {
			<-ctx.Done()
			cancelFn(adk.WithAgentCancelMode(adk.CancelAfterChatModel), adk.WithRecursive(), adk.WithAgentCancelTimeout(cancelTimeout))
		}()

		iter := runner.Query(ctx, task, cancelOpt)
		searchRounds := 0
		totalToolCalls := 0
		var fullContent string // 累积完整内容，用于结果校验

		for {
			event, ok := iter.Next()
			if !ok {
				break
			}

			if event.Err != nil {
				if ctx.Err() != nil {
					send(&service.SearchAgentEvent{Type: "error", Error: "搜索超时，请稍后重试"})
				} else if bizErr := extractBizError(event.Err); bizErr != nil {
					// 保留工具返回的业务错误码（如搜索引擎未配置、API Key 无效等），便于前端精确提示
					send(&service.SearchAgentEvent{Type: "error", ErrorCode: bizErr.Code, Error: bizErr.Message})
				} else {
					send(&service.SearchAgentEvent{Type: "error", Error: event.Err.Error()})
				}
				return
			}

			if event.Output == nil || event.Output.MessageOutput == nil {
				continue
			}

			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				logger.Warn("获取消息失败", zap.Error(err))
				continue
			}

			// 调试日志：记录事件详情
			logger.Debug("搜索Agent事件",
				zap.String("tool_name", event.Output.MessageOutput.ToolName),
				zap.String("role", string(msg.Role)),
				zap.Int("tool_calls_count", len(msg.ToolCalls)),
				zap.Int("content_length", len(msg.Content)),
			)

			// 统计搜索轮数
			if event.Output.MessageOutput.ToolName == "web_search" {
				searchRounds++
				if !send(&service.SearchAgentEvent{
					Type:         "search_round",
					SearchRounds: searchRounds,
				}) {
					return
				}
			}

			// 发送内容事件
			if msg.Content != "" {
				fullContent += msg.Content
				if !send(&service.SearchAgentEvent{
					Type:    "content",
					Content: msg.Content,
					Role:    string(msg.Role),
				}) {
					return
				}
			}

			// 工具调用事件 + 轮数截断
			if len(msg.ToolCalls) > 0 {
				totalToolCalls++
				for _, tc := range msg.ToolCalls {
					if !send(&service.SearchAgentEvent{
						Type:     "tool_call",
						ToolName: tc.Function.Name,
						ToolArgs: tc.Function.Arguments,
					}) {
						return
					}
				}
				if totalToolCalls > maxAgentRounds {
					logger.Warn("达到最大工具调用轮数，强制结束",
						zap.Int("maxRounds", maxAgentRounds),
						zap.Int("searchRounds", searchRounds),
					)
					break
				}
			}
		}

		// 硬约束（结果检查）：校验最终 results 数量是否为 10
		verifyFinalResultCount(fullContent)

		send(&service.SearchAgentEvent{
			Type:         "done",
			SearchRounds: searchRounds,
		})

		logger.Info("Agent 流式执行完成", zap.Int("searchRounds", searchRounds))
	}()

	return eventCh
}

// ExecuteWithImport 执行搜索并自动导入任务（主Agent调用模式）
func (a *SearchAgent) ExecuteWithImport(ctx context.Context, userID, notebookID uint, task string) (*service.SearchAgentResult, error) {
	ctx = WithUserID(ctx, userID)
	ctx = WithNotebookID(ctx, notebookID)
	ctx = agentTools.WithUserID(ctx, userID)
	ctx = agentTools.WithNotebookID(ctx, notebookID)

	_, _, _, execImportTimeout, cancelTimeout := a.agentRunParams()
	ctx, cancel := context.WithTimeout(ctx, execImportTimeout) // 自动导入模式给更长超时
	defer cancel()

	// per-user 限流
	release, err := a.limiter.acquire(userID)
	if err != nil {
		return nil, err
	}
	defer release()

	agent, err := a.createAgentWithImport(ctx, userID)
	if err != nil {
		logger.Error("创建搜索导入 Agent 失败",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 中断传播：客户端断开时让迭代器尽快退出
	cancelOpt, cancelFn := adk.WithCancel()
	go func() {
		<-ctx.Done()
		cancelFn(adk.WithAgentCancelMode(adk.CancelAfterChatModel), adk.WithRecursive(), adk.WithAgentCancelTimeout(cancelTimeout))
	}()

	searchRounds := 0
	totalToolCalls := 0
	var finalContent string

	iter := runner.Query(ctx, task, cancelOpt)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			if ctx.Err() != nil {
				logger.Error("Agent 执行超时", zap.Error(ctx.Err()))
				return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "搜索超时，请稍后重试", ctx.Err())
			}
			logger.Error("Agent 执行错误", zap.Error(event.Err))
			// 保留工具返回的业务错误码（如搜索引擎未配置、API Key 无效等），便于前端精确提示
			if bizErr := extractBizError(event.Err); bizErr != nil {
				return nil, bizErr
			}
			return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "Agent 执行失败", event.Err)
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			logger.Warn("获取消息失败", zap.Error(err))
			continue
		}

		// 统计搜索轮数
		if event.Output.MessageOutput.ToolName == "web_search" {
			searchRounds++
		}

		// 获取最终内容（assistant 消息且无工具调用）
		if msg.Role == schema.Assistant && len(msg.ToolCalls) == 0 {
			finalContent = msg.Content
		}

		// 工具调用计数
		if len(msg.ToolCalls) > 0 {
			totalToolCalls++
			if totalToolCalls > 5 { // 自动导入模式允许多一点轮数
				logger.Warn("达到最大工具调用轮数，强制结束",
					zap.Int("maxRounds", 5),
					zap.Int("searchRounds", searchRounds),
				)
				if finalContent == "" {
					finalContent = "搜索和导入已完成，但达到轮数限制。"
				}
				break
			}
		}
	}

	logger.Info("Agent 执行完成（自动导入模式）",
		zap.Int("searchRounds", searchRounds),
		zap.Int("contentLength", len(finalContent)),
	)

	// 硬约束（结果检查）：校验最终 results 数量是否为 10
	verifyFinalResultCount(finalContent)

	return &service.SearchAgentResult{
		Content:      finalContent,
		SearchRounds: searchRounds,
	}, nil
}
