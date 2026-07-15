package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

// cancelTrigger 根据 ctx.Err() 返回取消触发原因。
// 前端关闭搜索面板（AbortController.abort）和客户端网络断开在 ctx 层面都是 Canceled，
// 语义上都是"放弃结果"，统一归类为 user_cancel。
func cancelTrigger(ctx context.Context) string {
	switch ctx.Err() {
	case context.DeadlineExceeded:
		return "timeout"
	case context.Canceled:
		return "user_cancel"
	default:
		return ""
	}
}

// cancelModeString 将 adk.CancelMode 转为可读字符串。
func cancelModeString(m adk.CancelMode) string {
	switch m {
	case adk.CancelImmediate:
		return "immediate"
	case adk.CancelAfterChatModel:
		return "after_chat_model"
	case adk.CancelAfterToolCalls:
		return "after_tool_calls"
	default:
		return fmt.Sprintf("combined(%d)", int(m))
	}
}

// waitResultString 将 CancelHandle.Wait() 的返回值转为可读字符串。
//   - nil: 取消成功，agent 在安全点停下
//   - ErrCancelTimeout: 安全点等待超时，已升级为 immediate
//   - ErrExecutionEnded: agent 已自然结束，本次取消未生效（contributed=false）
func waitResultString(err error) string {
	switch {
	case err == nil:
		return "succeeded"
	case errors.Is(err, adk.ErrCancelTimeout):
		return "cancel_timeout_escalated"
	case errors.Is(err, adk.ErrExecutionEnded):
		return "execution_ended"
	default:
		return err.Error()
	}
}

// verifyFinalResultCount 硬约束（结果检查）：从最终回复内容中解析 JSON 并校验 results 数量是否为 10
// 仅记录告警，不修改内容；流式场景下内容已发出，此处用于事后排查
func verifyFinalResultCount(content string) {
	if content == "" {
		return
	}

	got, ok := searchResultCount(content)
	if !ok {
		logger.Warn("结果校验：JSON 解析失败，跳过数量校验",
			zap.String("reason", "未找到包含 results 的 JSON 对象"),
		)
		return
	}
	if got != requiredResultCount {
		logger.Warn("结果校验：最终 results 数量不符合要求",
			zap.Int("got", got),
			zap.Int("required", requiredResultCount),
		)
	} else {
		logger.Info("结果校验：results 数量校验通过", zap.Int("count", got))
	}
}

func searchResultCount(content string) (int, bool) {
	best := 0
	found := false
	for offset := 0; offset < len(content); {
		rel := strings.IndexByte(content[offset:], '{')
		if rel < 0 {
			break
		}
		start := offset + rel
		var parsed struct {
			Results []json.RawMessage `json:"results"`
		}
		if err := json.NewDecoder(strings.NewReader(content[start:])).Decode(&parsed); err == nil && parsed.Results != nil {
			best = len(parsed.Results)
			found = true
		}
		offset = start + 1
	}
	return best, found
}

// SearchAgent 搜索 Agent（基于 Eino 框架）
type SearchAgent struct {
	configService service.ConfigService
	limiter       *userLimiter // per-user 并发限流（eino 未提供，自行实现）
}

// NewSearchAgent 创建搜索 Agent
func NewSearchAgent(configService service.ConfigService) *SearchAgent {
	maxConcurrent := 1
	if cfg := config.Get(); cfg != nil && cfg.Agent.MaxConcurrent > 0 {
		maxConcurrent = cfg.Agent.MaxConcurrent
	}
	return &SearchAgent{
		configService: configService,
		limiter:       newUserLimiter(maxConcurrent), // per-user 并发上限（可配置，默认 1）
	}
}

// agentRunParams 返回运行参数（config 零值时用代码内默认值，无需在 yaml 配置）
func (a *SearchAgent) agentRunParams() (maxIter int, execTimeout, cancelTimeout time.Duration) {
	maxIter = 6
	execTimeout = 3 * time.Minute
	cancelTimeout = 5 * time.Second
	if cfg := config.Get(); cfg != nil {
		ac := cfg.Agent
		if ac.MaxIterations > 0 {
			maxIter = ac.MaxIterations
		}
		if ac.ExecuteTimeout > 0 {
			execTimeout = ac.ExecuteTimeout
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

// InjectContext 注入子 agent 工具执行所需的 userID context（实现 chat.SubAgentBuilder 接口）。
// 主 agent 通过 NewAgentTool 调用子 agent 时，context 缺 search 包的 userID，
// 会导致 web_search 工具读不到用户搜索引擎配置。此处补注入。
func (a *SearchAgent) InjectContext(ctx context.Context, userID uint) context.Context {
	ctx = WithUserID(ctx, userID) // search 包的 context key（web_search 工具用 GetUserID 读取）
	return ctx
}

// BuildAgent 创建 Eino Agent（用户交互模式：只搜索不自动导入）。
// 导出供主 Agent 通过 adk.NewAgentTool 包装调用。
func (a *SearchAgent) BuildAgent(ctx context.Context, userID uint) (*adk.ChatModelAgent, error) {
	chatModel, err := a.createChatModel(ctx, userID)
	if err != nil {
		return nil, err
	}

	tools, err := a.createTools()
	if err != nil {
		return nil, err
	}

	maxIter, _, _ := a.agentRunParams()
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
func (a *SearchAgent) Execute(ctx context.Context, userID, _ uint, task string) (*service.SearchAgentResult, error) {
	ctx = WithUserID(ctx, userID)

	_, execTimeout, cancelTimeout := a.agentRunParams()
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	// per-user 限流（eino 未提供，自行实现）
	release, err := a.limiter.acquire(userID)
	if err != nil {
		return nil, err
	}
	defer release()

	agent, err := a.BuildAgent(ctx, userID)
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

	// 中断传播：eino 的 iter.Next() 不响应 ctx，必须用 adk.WithCancel() 才能让迭代器在客户端断开时退出。
	// 保留 CancelHandle 并异步 Wait() 收集取消结局（nil/ErrCancelTimeout/ErrExecutionEnded），
	// 同时记录 contributed（本次取消是否实际生效）。
	cancelOpt, cancelFn := adk.WithCancel()
	go func() {
		<-ctx.Done()
		handle, contributed := cancelFn(
			adk.WithAgentCancelMode(adk.CancelAfterChatModel),
			adk.WithRecursive(),
			adk.WithAgentCancelTimeout(cancelTimeout),
		)
		go func() {
			waitErr := handle.Wait()
			if contributed {
				logger.Info("搜索 Agent 取消操作结局",
					zap.String("trigger", cancelTrigger(ctx)),
					zap.Bool("contributed", contributed),
					zap.String("wait_result", waitResultString(waitErr)),
				)
			} else {
				// contributed=false：取消未生效（agent 已自然结束，defer cancel 触发的桥接），降级 Debug 避免噪音
				logger.Debug("搜索 Agent 取消操作结局（未生效）",
					zap.String("trigger", cancelTrigger(ctx)),
					zap.String("wait_result", waitResultString(waitErr)),
				)
			}
		}()
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
			// 优先识别 ADK 取消错误：拿到 Mode/Escalated/Timeout 这些 ctx.Err() 拿不到的信息
			var cancelErr *adk.CancelError
			if errors.As(event.Err, &cancelErr) {
				trigger := cancelTrigger(ctx)
				logger.Info("搜索 Agent 被取消",
					zap.String("trigger", trigger),
					zap.String("mode", cancelModeString(cancelErr.Info.Mode)),
					zap.Bool("escalated", cancelErr.Info.Escalated),
					zap.Bool("timeout", cancelErr.Info.Timeout),
				)
				if trigger == "timeout" {
					return nil, bizerrors.NewWithErr(bizerrors.CodeSearchAgentTimeout, "搜索超时，请稍后重试", ctx.Err())
				}
				// user_cancel：用户主动关闭搜索面板，返回 ctx.Err() 让上层感知
				return nil, ctx.Err()
			}
			// 兜底：ctx 已取消但 ADK 还没投递 CancelError（如 immediate 中断 stream 的边界情况）
			if ctx.Err() != nil {
				trigger := cancelTrigger(ctx)
				logger.Info("搜索 Agent 因 ctx 取消退出（未收到 CancelError）",
					zap.String("trigger", trigger),
					zap.Error(event.Err),
				)
				if trigger == "timeout" {
					return nil, bizerrors.NewWithErr(bizerrors.CodeSearchAgentTimeout, "搜索超时，请稍后重试", ctx.Err())
				}
				return nil, ctx.Err()
			}
			logger.Error("Agent 执行错误", zap.Error(event.Err))
			// 保留工具返回的业务错误码（如搜索引擎未配置、API Key 无效等），便于前端精确提示
			if bizErr := extractBizError(event.Err); bizErr != nil {
				return nil, bizErr
			}
			return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "搜索服务暂时不可用，请稍后重试", event.Err)
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			if ctx.Err() != nil {
				logger.Debug("获取消息失败（ctx 已取消）", zap.Error(err))
			} else {
				logger.Warn("获取消息失败", zap.Error(err))
			}
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
func (a *SearchAgent) ExecuteStream(ctx context.Context, userID, _ uint, task string) <-chan *service.SearchAgentEvent {
	eventCh := make(chan *service.SearchAgentEvent, 16)

	go func() {
		defer close(eventCh)

		ctx = WithUserID(ctx, userID)

		_, execTimeout, cancelTimeout := a.agentRunParams()
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

		agent, err := a.BuildAgent(ctx, userID)
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

		if !send(&service.SearchAgentEvent{Type: "started"}) {
			return
		}

		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
		})

		// 中断传播：eino 的 iter.Next() 不响应 ctx，必须用 adk.WithCancel() 才能让迭代器在客户端断开时退出。
		// 保留 CancelHandle 并异步 Wait() 收集取消结局（nil/ErrCancelTimeout/ErrExecutionEnded），
		// 同时记录 contributed（本次取消是否实际生效）。
		cancelOpt, cancelFn := adk.WithCancel()
		go func() {
			<-ctx.Done()
			handle, contributed := cancelFn(
				adk.WithAgentCancelMode(adk.CancelAfterChatModel),
				adk.WithRecursive(),
				adk.WithAgentCancelTimeout(cancelTimeout),
			)
			go func() {
				waitErr := handle.Wait()
				if contributed {
					logger.Info("搜索 Agent 取消操作结局",
						zap.String("trigger", cancelTrigger(ctx)),
						zap.Bool("contributed", contributed),
						zap.String("wait_result", waitResultString(waitErr)),
					)
				} else {
					// contributed=false：取消未生效（agent 已自然结束，defer cancel 触发的桥接），降级 Debug 避免噪音
					logger.Debug("搜索 Agent 取消操作结局（未生效）",
						zap.String("trigger", cancelTrigger(ctx)),
						zap.String("wait_result", waitResultString(waitErr)),
					)
				}
			}()
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
				// 优先识别 ADK 取消错误：拿到 Mode/Escalated/Timeout 这些 ctx.Err() 拿不到的信息
				var cancelErr *adk.CancelError
				if errors.As(event.Err, &cancelErr) {
					trigger := cancelTrigger(ctx)
					logger.Info("搜索 Agent 被取消",
						zap.String("trigger", trigger),
						zap.String("mode", cancelModeString(cancelErr.Info.Mode)),
						zap.Bool("escalated", cancelErr.Info.Escalated),
						zap.Bool("timeout", cancelErr.Info.Timeout),
					)
					if trigger == "timeout" {
						// 真超时：前端可能还在等，发送超时错误事件
						send(&service.SearchAgentEvent{Type: "error", ErrorCode: bizerrors.CodeSearchAgentTimeout, Error: "搜索超时，请稍后重试"})
					}
					// user_cancel：前端已断开（关面板），send 也会因 ctx.Done 返回 false，直接 return
					return
				}
				// 兜底：ctx 已取消但 ADK 还没投递 CancelError（如 immediate 中断 stream 的边界情况）
				if ctx.Err() != nil {
					trigger := cancelTrigger(ctx)
					logger.Info("搜索 Agent 因 ctx 取消退出（未收到 CancelError）",
						zap.String("trigger", trigger),
						zap.Error(event.Err),
					)
					if trigger == "timeout" {
						send(&service.SearchAgentEvent{Type: "error", ErrorCode: bizerrors.CodeSearchAgentTimeout, Error: "搜索超时，请稍后重试"})
					}
					return
				}
				logger.Error("Agent 执行错误", zap.Error(event.Err))
				if bizErr := extractBizError(event.Err); bizErr != nil {
					// 保留工具返回的业务错误码（如搜索引擎未配置、API Key 无效等），便于前端精确提示
					send(&service.SearchAgentEvent{Type: "error", ErrorCode: bizErr.Code, Error: bizErr.Message})
				} else {
					// 非业务错误（如 LLM 网络故障）：返回友好提示，原始错误已由上方 logger.Error 记录
					send(&service.SearchAgentEvent{Type: "error", ErrorCode: bizerrors.CodeLLMCallFailed, Error: "搜索服务暂时不可用，请稍后重试"})
				}
				return
			}

			if event.Output == nil || event.Output.MessageOutput == nil {
				continue
			}

			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				if ctx.Err() != nil {
					logger.Debug("获取消息失败（ctx 已取消）", zap.Error(err))
				} else {
					logger.Warn("获取消息失败", zap.Error(err))
				}
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
