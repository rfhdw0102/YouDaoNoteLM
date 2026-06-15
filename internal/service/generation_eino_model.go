package service

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

// NewEinoGenerationModel adapts an Eino chat model to GenerationModel.
func NewEinoGenerationModel(chat model.BaseChatModel) GenerationModel {
	return &einoGenerationModel{chat: chat}
}

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

	msg, err := m.chat.Generate(ctx, []*schema.Message{
		schema.SystemMessage(prompt.System),
		schema.UserMessage(user.String()),
	})
	if err != nil {
		return "", fmt.Errorf("eino generation failed: %w", err)
	}
	if msg == nil {
		return "", nil
	}
	return strings.TrimSpace(msg.Content), nil
}
