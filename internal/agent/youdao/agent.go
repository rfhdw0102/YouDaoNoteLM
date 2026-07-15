package youdao

import (
	"context"
	"time"

	agentTools "YoudaoNoteLm/internal/agent/tools"
	"YoudaoNoteLm/internal/llm"
	"YoudaoNoteLm/internal/service"
	externalYoudao "YoudaoNoteLm/internal/service/external/youdao"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// YoudaoAgent 有道云笔记 Agent（基于 Eino 框架）
type YoudaoAgent struct {
	configService   service.ConfigService
	youdaoService   service.YoudaoService
	youdaoCLI       externalYoudao.CLI
	importerService service.ImporterService // 用于 import_document tool
}

// NewYoudaoAgent 创建有道云笔记 Agent
func NewYoudaoAgent(
	configService service.ConfigService,
	youdaoService service.YoudaoService,
	youdaoCLI externalYoudao.CLI,
	importerService service.ImporterService,
) *YoudaoAgent {
	return &YoudaoAgent{
		configService:   configService,
		youdaoService:   youdaoService,
		youdaoCLI:       youdaoCLI,
		importerService: importerService,
	}
}

// createChatModel 根据用户配置创建 ToolCallingChatModel（支持多 Provider）
func (a *YoudaoAgent) createChatModel(ctx context.Context, userID uint) (model.ToolCallingChatModel, error) {
	cfg, err := a.configService.GetUserLLMConfig(userID)
	if err != nil {
		return nil, err
	}
	return llm.NewToolCallingChatModel(ctx, cfg)
}

// createTools 创建 Agent 的工具列表
func (a *YoudaoAgent) createTools() ([]tool.BaseTool, error) {
	tools := make([]tool.BaseTool, 0, 5)

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

	// 统一导入工具（替代旧的 import_note + import_notes_batch）
	importDocTool, err := agentTools.NewImportDocumentTool(a.youdaoService, a.importerService)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMCallFailed, "创建 import_document 工具失败", err)
	}
	tools = append(tools, importDocTool)

	return tools, nil
}

// InjectContext 注入子 agent 工具执行所需的 userID context（实现 chat.SubAgentBuilder 接口）。
// 主 agent 通过 NewAgentTool 调用子 agent 时，context 缺 youdao 包的 userID，
// 会导致有道工具读不到用户身份。此处补注入。
func (a *YoudaoAgent) InjectContext(ctx context.Context, userID uint) context.Context {
	ctx = WithUserID(ctx, userID)            // youdao 包的 context key（有道工具用 GetUserID 读取）
	ctx = agentTools.WithUserID(ctx, userID) // agent/tools 包的 context key（import_document 工具用）
	return ctx
}

// BuildAgent 创建 Eino Agent（导出供主 Agent 通过 adk.NewAgentTool 包装调用）
func (a *YoudaoAgent) BuildAgent(ctx context.Context, userID uint) (*adk.ChatModelAgent, error) {
	chatModel, err := a.createChatModel(ctx, userID)
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
	ctx = agentTools.WithUserID(ctx, userID) // import_document 工具读取这个

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	agent, err := a.BuildAgent(ctx, userID)
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
