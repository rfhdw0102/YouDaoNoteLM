package chat

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ChatAgentConfig Agent 配置
type ChatAgentConfig struct {
	Model        model.ToolCallingChatModel // 带 tool calling 的 LLM
	Tools        []tool.BaseTool            // 可用工具集
	MaxSteps     int                        // 最大推理步数，默认 10
	SystemPrompt string                     // 系统提示词
}

// ChatAgent 基于 eino ADK 的对话 Agent
type ChatAgent struct {
	agent  *adk.ChatModelAgent
	config *ChatAgentConfig
}

// NewChatAgent 创建 Agent 实例
func NewChatAgent(ctx context.Context, cfg *ChatAgentConfig) (*ChatAgent, error) {
	if cfg.Model == nil {
		return nil, fmt.Errorf("ChatModel 不能为空")
	}
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 10
	}

	// 创建 ChatModelAgent（ReAct 循环）
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       cfg.Model,
		Instruction: cfg.SystemPrompt,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: cfg.Tools,
			},
		},
		MaxIterations: cfg.MaxSteps,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModelAgent 失败: %w", err)
	}

	return &ChatAgent{
		agent:  agent,
		config: cfg,
	}, nil
}

// Run 执行 Agent，返回流式事件迭代器
func (a *ChatAgent) Run(ctx context.Context, messages []*schema.Message) *adk.AsyncIterator[*adk.AgentEvent] {
	input := &adk.AgentInput{
		Messages:        messages,
		EnableStreaming: true,
	}
	return a.agent.Run(ctx, input)
}
