// generation_eino_model.go 提供 eino 框架的模型适配器。
//
// einoGenerationModel 将 GenerationModel 接口适配为 eino 框架所需的
// chat model 接口，使 Agent 链可以统一调用不同 LLM 后端。
package generation

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type einoGenerationModel struct {
	chat model.BaseChatModel
}

const (
	defaultGenerationMaxTokens = 8192
	pptContentEnrichMaxTokens  = defaultGenerationMaxTokens
	defaultGenerationTopP      = 0.9
	defaultGenerationTemp      = 0.7
)

// NewEinoGenerationModel 创建适配 eino chat model 的 GenerationModel 实例。
func NewEinoGenerationModel(chat model.BaseChatModel) GenerationModel {
	return &einoGenerationModel{chat: chat}
}

// Generate 将提示词拼装为消息并调用 eino chat model 生成内容。
func (m *einoGenerationModel) Generate(ctx context.Context, prompt GenerationPrompt) (string, error) {
	if m == nil || m.chat == nil {
		return "", nil
	}
	user := strings.Builder{}
	user.WriteString("智能体：")
	user.WriteString(prompt.AgentName)
	user.WriteString("\n\n上下文：\n")
	user.WriteString(prompt.Context)
	user.WriteString("\n\n用户要求：\n")
	user.WriteString(prompt.User)
	user.WriteString("\n\n输出格式：\n")
	user.WriteString(prompt.OutputFormat)

	maxTokens := prompt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultGenerationMaxTokens
	}

	msg, err := m.chat.Generate(ctx, []*schema.Message{
		schema.SystemMessage(prompt.System),
		schema.UserMessage(user.String()),
	},
		model.WithMaxTokens(maxTokens),
		model.WithTopP(defaultGenerationTopP),
		model.WithTemperature(defaultGenerationTemp),
	)
	if err != nil {
		return "", fmt.Errorf("eino generation failed: %w", err)
	}
	if msg == nil {
		return "", nil
	}
	return strings.TrimSpace(msg.Content), nil
}
