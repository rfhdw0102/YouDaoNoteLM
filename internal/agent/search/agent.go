// internal/agent/search/agent.go
package search

import (
	"context"
	"net/http"
	"time"

	"YoudaoNoteLm/internal/service"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	einoOpenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const maxAgentRounds = 2

// SearchAgent 搜索 Agent（基于 Eino 框架）
type SearchAgent struct {
	configService service.ConfigService
	importer      service.ImporterService
}

// NewSearchAgent 创建搜索 Agent
func NewSearchAgent(
	configService service.ConfigService,
	importer service.ImporterService,
) *SearchAgent {
	return &SearchAgent{
		configService: configService,
		importer:      importer,
	}
}

// createEinoChatModel 根据用户配置创建 Eino OpenAI ChatModel
func (a *SearchAgent) createEinoChatModel(userID uint) (*einoOpenai.ChatModel, error) {
	cfg, err := a.configService.GetChatModelConfig(userID)
	if err != nil {
		return nil, err
	}

	return einoOpenai.NewChatModel(context.Background(), &einoOpenai.ChatModelConfig{
		Model:      cfg.Model,
		BaseURL:    cfg.BaseURL,
		APIKey:     cfg.APIKey,
		Timeout:    60 * time.Second,
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	})
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

// createToolsWithImport 创建搜索 Agent 的工具列表（web_search + import_urls，自动导入模式）
func (a *SearchAgent) createToolsWithImport() ([]tool.BaseTool, error) {
	tools := make([]tool.BaseTool, 0, 2)

	webSearchTool, err := NewWebSearchTool(a.configService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 web_search 工具失败", err)
	}
	tools = append(tools, webSearchTool)

	importTool, err := NewImportURLsTool(a.importer)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 import_urls 工具失败", err)
	}
	tools = append(tools, importTool)

	return tools, nil
}

// createAgent 创建 Eino Agent（用户交互模式：只搜索不自动导入）
func (a *SearchAgent) createAgent(ctx context.Context, userID uint) (*adk.ChatModelAgent, error) {
	chatModel, err := a.createEinoChatModel(userID)
	if err != nil {
		return nil, err
	}

	tools, err := a.createTools()
	if err != nil {
		return nil, err
	}

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
		MaxIterations: 4, // 2 轮搜索 + 最终回复 + 余量
	})
}

// createAgentWithImport 创建 Eino Agent（自动导入模式：搜索并自动导入）
func (a *SearchAgent) createAgentWithImport(ctx context.Context, userID uint) (*adk.ChatModelAgent, error) {
	chatModel, err := a.createEinoChatModel(userID)
	if err != nil {
		return nil, err
	}

	tools, err := a.createToolsWithImport()
	if err != nil {
		return nil, err
	}

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
		MaxIterations: 5, // 2 轮搜索 + 导入 + 最终回复 + 余量
	})
}

// Execute 执行搜索任务（非流式）
func (a *SearchAgent) Execute(ctx context.Context, userID, notebookID uint, task string) (*service.SearchAgentResult, error) {
	ctx = WithUserID(ctx, userID)
	ctx = WithNotebookID(ctx, notebookID)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

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

	searchRounds := 0
	totalToolCalls := 0
	var finalContent string

	iter := runner.Query(ctx, task)
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

		ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()

		agent, err := a.createAgent(ctx, userID)
		if err != nil {
			logger.Error("创建搜索 Agent 失败",
				zap.Uint("user_id", userID),
				zap.Error(err),
			)
			// 如果是业务错误，提取错误码
			event := &service.SearchAgentEvent{Type: "error", Error: err.Error()}
			if bizErr, ok := err.(*bizerrors.BizError); ok {
				event.ErrorCode = bizErr.Code
				event.Error = bizErr.Message // 使用友好的错误消息，而不是包含错误码的完整信息
			}
			eventCh <- event
			return
		}

		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
		})

		iter := runner.Query(ctx, task)
		searchRounds := 0
		totalToolCalls := 0

		for {
			event, ok := iter.Next()
			if !ok {
				break
			}

			if event.Err != nil {
				if ctx.Err() != nil {
					eventCh <- &service.SearchAgentEvent{Type: "error", Error: "搜索超时，请稍后重试"}
				} else {
					eventCh <- &service.SearchAgentEvent{Type: "error", Error: event.Err.Error()}
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

			// 统计搜索轮数
			if event.Output.MessageOutput.ToolName == "web_search" {
				searchRounds++
				eventCh <- &service.SearchAgentEvent{
					Type:         "search_round",
					SearchRounds: searchRounds,
				}
			}

			// 发送内容事件
			if msg.Content != "" {
				eventCh <- &service.SearchAgentEvent{
					Type:    "content",
					Content: msg.Content,
					Role:    string(msg.Role),
				}
			}

			// 工具调用事件 + 轮数截断
			if len(msg.ToolCalls) > 0 {
				totalToolCalls++
				for _, tc := range msg.ToolCalls {
					eventCh <- &service.SearchAgentEvent{
						Type:     "tool_call",
						ToolName: tc.Function.Name,
						ToolArgs: tc.Function.Arguments,
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

		eventCh <- &service.SearchAgentEvent{
			Type:         "done",
			SearchRounds: searchRounds,
		}

		logger.Info("Agent 流式执行完成", zap.Int("searchRounds", searchRounds))
	}()

	return eventCh
}

// ExecuteWithImport 执行搜索并自动导入任务（主Agent调用模式）
func (a *SearchAgent) ExecuteWithImport(ctx context.Context, userID, notebookID uint, task string) (*service.SearchAgentResult, error) {
	ctx = WithUserID(ctx, userID)
	ctx = WithNotebookID(ctx, notebookID)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute) // 自动导入模式给更长超时
	defer cancel()

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

	searchRounds := 0
	totalToolCalls := 0
	var finalContent string

	iter := runner.Query(ctx, task)
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

	return &service.SearchAgentResult{
		Content:      finalContent,
		SearchRounds: searchRounds,
	}, nil
}
