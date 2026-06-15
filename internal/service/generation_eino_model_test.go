package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeEinoChatModel struct {
	messages []*schema.Message
	opts     []model.Option
}

func (f *fakeEinoChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	f.messages = input
	f.opts = opts
	return schema.AssistantMessage("generated content", nil), nil
}

func (f *fakeEinoChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func TestEinoGenerationModelBuildsMessages(t *testing.T) {
	chat := &fakeEinoChatModel{}
	model := NewEinoGenerationModel(chat)

	out, err := model.Generate(context.Background(), GenerationPrompt{
		AgentName:    "note",
		System:       "system prompt",
		User:         "user prompt",
		Context:      "rag context",
		OutputFormat: "markdown only",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if out != "generated content" {
		t.Fatalf("out = %q", out)
	}
	if len(chat.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(chat.messages))
	}
	if chat.messages[0].Role != schema.System || !strings.Contains(chat.messages[0].Content, "system prompt") {
		t.Fatalf("system message not populated: %#v", chat.messages[0])
	}
	if chat.messages[1].Role != schema.User || !strings.Contains(chat.messages[1].Content, "rag context") || !strings.Contains(chat.messages[1].Content, "markdown only") {
		t.Fatalf("user message not populated: %#v", chat.messages[1])
	}
}

func TestEinoGenerationModelUsesChinesePromptLabels(t *testing.T) {
	chat := &fakeEinoChatModel{}
	genModel := NewEinoGenerationModel(chat)

	_, err := genModel.Generate(context.Background(), GenerationPrompt{
		AgentName:    "note",
		System:       "系统提示词",
		User:         "用户要求",
		Context:      "上下文",
		OutputFormat: "输出格式",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(chat.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(chat.messages))
	}
	userContent := chat.messages[1].Content
	for _, want := range []string{"智能体：note", "上下文：", "用户要求：", "输出格式："} {
		if !strings.Contains(userContent, want) {
			t.Fatalf("user message missing %q: %s", want, userContent)
		}
	}
	for _, unwanted := range []string{"Agent:", "Context:", "User request:", "Output format:"} {
		if strings.Contains(userContent, unwanted) {
			t.Fatalf("user message still contains English label %q: %s", unwanted, userContent)
		}
	}
}

func TestEinoGenerationModelUsesLongGenerationBudget(t *testing.T) {
	chat := &fakeEinoChatModel{}
	genModel := NewEinoGenerationModel(chat)

	_, err := genModel.Generate(context.Background(), GenerationPrompt{
		AgentName:    "note",
		System:       "系统提示词",
		User:         "生成详细学习笔记",
		Context:      "上下文",
		OutputFormat: "输出格式",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	opts := model.GetCommonOptions(nil, chat.opts...)
	if opts.MaxTokens == nil || *opts.MaxTokens < 8192 {
		t.Fatalf("MaxTokens = %v, want at least 8192", opts.MaxTokens)
	}
}
