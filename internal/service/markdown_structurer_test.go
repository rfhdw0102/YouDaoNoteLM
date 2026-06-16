package service

import (
	"context"
	"strings"
	"testing"

	"YoudaoNoteLm/internal/model/entity"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// --- Mock ChatModel ---

type mockStructureChatModel struct {
	generateFn func(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

func (m *mockStructureChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, input, opts...)
	}
	return schema.AssistantMessage("## 标题\n\n结构化内容", nil), nil
}

func (m *mockStructureChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

// --- Mock ConfigService（嵌入真实接口，只覆盖需要的方法） ---

type mockStructureConfigService struct {
	ConfigService // 嵌入接口，其他方法自动满足
	llmConfig     *entity.UserLLMConfig
	llmErr        error
}

func (m *mockStructureConfigService) GetUserLLMConfig(userID uint) (*entity.UserLLMConfig, error) {
	return m.llmConfig, m.llmErr
}

// --- 测试用例 ---

func TestHasSufficientStructure(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"无标题", "just plain text\nno headings here", false},
		{"一个标题", "## Title\nsome content", false},
		{"两个标题", "## Title 1\ncontent\n## Title 2\nmore", true},
		{"三个标题", "## A\n## B\n## C", true},
		{"只有 h1", "# Title\ncontent", false},
		{"只有 h3", "### Sub\ncontent", false},
		{"混合层级", "## A\n### B\n## C", true},
		{"空内容", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSufficientStructure(tt.content)
			if got != tt.want {
				t.Errorf("hasSufficientStructure(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestStructure_空内容(t *testing.T) {
	svc := &markdownStructurer{}
	result, err := svc.Structure(context.Background(), 1, "", StructureMeta{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty, got %q", result.Content)
	}
	if result.ActuallyCalled {
		t.Errorf("expected ActuallyCalled=false for empty content")
	}
}

func TestStructure_已有结构时跳过(t *testing.T) {
	configSvc := &mockStructureConfigService{
		llmConfig: &entity.UserLLMConfig{Provider: "test", APIKey: "key", Model: "model", Enabled: true},
	}
	svc := NewMarkdownStructurer(configSvc)

	input := "## 第一章\n\n内容一\n\n## 第二章\n\n内容二"
	result, err := svc.Structure(context.Background(), 1, input, StructureMeta{SourceType: "file"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != input {
		t.Errorf("已有结构时应返回原始内容，但返回了不同的输出")
	}
	if result.ActuallyCalled {
		t.Errorf("已有结构时不应调用 
}

func TestStructure_LLM配置为nil时降级(t *testing.T) {
	configSvc := &mockStructureConfigService{
		llmConfig: nil,
	}
	svc := NewMarkdownStructurer(configSvc)

	input := "some unstructured text without headings"
	result, err := svc.Structure(context.Background(), 1, input, StructureMeta{SourceType: "file"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
	if result.Content != input {
	}
}
 nil 时应返回原始内容，但返回了不同的输出")
	}
	if result.ActuallyCalled {
		t.Errorf("LLM 配置为 nil 时不应调用 L

func TestStructure_LLM未启用时降级(t *testing.T) {
	configSvc := &mockStructureConfigService{
		llmConfig: &entity.UserLLMConfig{Enabled: false},
	}
	svc := NewMarkdownStructurer(configSvc)

	input := "plain text no structure"
	result, err := svc.Structure(context.Background(), 1, input, StructureMeta{SourceType: "url"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != input {
		t.Errorf("LLM 未启用时应返回原始内容，但返回了不同的输出")
	}
的输出")
	}
	if result.ActuallyCalled {
		t.Errorf("LLM 
}

func TestStructure_Prompt模板包含格式占位符(t *testing.T) {
	template := structureUserPromptTemplate
	if !strings.Contains(template, "%s") {
		t.Error("user prompt template 应包含格式占位符")
	}
}
