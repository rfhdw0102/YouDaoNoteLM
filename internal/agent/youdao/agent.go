package youdao

import (
	"context"
	"net/http"
	"time"

	"YoudaoNoteLm/internal/service"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	einoOpenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// YoudaoAgent 有道云笔记 Agent（基于 Eino 框架）
type YoudaoAgent struct {
	configService service.ConfigService
	youdaoService service.YoudaoService
	youdaoCLI     externalYoudao.CLI
}

// NewYoudaoAgent 创建有道云笔记 Agent
func NewYoudaoAgent(
	configService service.ConfigService,
	youdaoService service.YoudaoService,
	youdaoCLI externalYoudao.CLI,
) *YoudaoAgent {
	return &YoudaoAgent{
		configService: configService,
		youdaoService: youdaoService,
		youdaoCLI:     youdaoCLI,
	}
}

// createEinoChatModel 根据用户配置创建 Eino OpenAI ChatModel
func (a *YoudaoAgent) createEinoChatModel(userID uint) (*einoOpenai.ChatModel, error) {
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

// createTools 创建 Agent 的工具列表
func (a *YoudaoAgent) createTools() ([]tool.BaseTool, error) {
	tools := make([]tool.BaseTool, 0, 7)

	listTool, err := NewListNotesTool(a.youdaoService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 list_notes 工具失败", err)
	}
	tools = append(tools, listTool)

	readTool, err := NewReadNoteTool(a.youdaoCLI, a.youdaoService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 read_note 工具失败", err)
	}
	tools = append(tools, readTool)

	searchTool, err := NewSearchNotesTool(a.youdaoCLI, a.youdaoService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 search_notes 工具失败", err)
	}
	tools = append(tools, searchTool)

	createTool, err := NewCreateNoteTool(a.youdaoCLI, a.youdaoService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 create_note 工具失败", err)
	}
	tools = append(tools, createTool)

	importTool, err := NewImportNoteTool(a.youdaoService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 import_note 工具失败", err)
	}
	tools = append(tools, importTool)

	importBatchTool, err := NewImportNotesBatchTool(a.youdaoService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 import_notes_batch 工具失败", err)
	}
	tools = append(tools, importBatchTool)

	return tools, nil
}

// createAgent 创建 Eino Agent
func (a *YoudaoAgent) createAgent(ctx context.Context, userID uint) (*adk.ChatModelAgent, error) {
	chatModel, err := a.createEinoChatModel(userID)
	if err != nil {
		return nil, err
	}

	tools, err := a.createTools()
	if err != nil {
		return nil, err
	}

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "YoudaoNoteAgent",
		Description: "有道云笔记助手，帮助用户操作有道云笔记",
		Instruction: YoudaoAgentSystemPrompt,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
		MaxIterations: 8,
	})
}

// Execute 执行有道云笔记 Agent 任务
func (a *YoudaoAgent) Execute(ctx context.Context, userID uint, task string) (string, error) {
	ctx = WithUserID(ctx, userID)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	agent, err := a.createAgent(ctx, userID)
	if err != nil {
		return "", err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

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
				logger.Error("YoudaoAgent 执行超时", zap.Error(ctx.Err()))
				return "", bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "操作超时，请稍后重试", ctx.Err())
			}
			logger.Error("YoudaoAgent 执行错误", zap.Error(event.Err))
			return "", bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "Agent 执行失败", event.Err)
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			logger.Warn("获取消息失败", zap.Error(err))
			continue
		}

		if msg.Role == schema.Assistant && len(msg.ToolCalls) == 0 {
			finalContent = msg.Content
		}

		if len(msg.ToolCalls) > 0 {
			totalToolCalls++
			if totalToolCalls > 8 {
				logger.Warn("达到最大工具调用轮数，强制结束", zap.Int("maxRounds", 8))
				if finalContent == "" {
					finalContent = "操作已完成，但达到轮数限制。"
				}
				break
			}
		}
	}

	logger.Info("YoudaoAgent 执行完成",
		zap.Uint("user_id", userID),
		zap.Int("toolCalls", totalToolCalls),
		zap.Int("contentLength", len(finalContent)),
	)

	return finalContent, nil
}
