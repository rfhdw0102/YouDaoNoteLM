package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"YoudaoNoteLm/pkg/logger"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// AnthropicChatModel 实现 eino model.ChatModel 接口，适配 Anthropic Claude API
type AnthropicChatModel struct {
	client  anthropic.Client
	modelID string
	tools   []anthropic.ToolUnionParam
}

// NewAnthropicChatModel 创建 Anthropic ChatModel
func NewAnthropicChatModel(_ context.Context, apiKey, modelID, baseURL string) (*AnthropicChatModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API Key 不能为空")
	}
	if modelID == "" {
		return nil, fmt.Errorf("Anthropic Model 不能为空")
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" && baseURL != "https://api.anthropic.com" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := anthropic.NewClient(opts...)

	return &AnthropicChatModel{
		client:  client,
		modelID: modelID,
	}, nil
}

// Generate 同步生成
func (m *AnthropicChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	params := m.buildParams(input)

	resp, err := m.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("Anthropic API 调用失败: %w", err)
	}

	return m.convertResponse(resp), nil
}

// Stream 流式生成
func (m *AnthropicChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	params := m.buildParams(input)

	stream := m.client.Messages.NewStreaming(ctx, params)

	sr, sw := schema.Pipe[*schema.Message](8)

	go func() {
		defer sw.Close()

		// 用于跟踪 tool_use 块的构建
		var currentToolID, currentToolName string
		var currentToolInput string
		var isToolUse bool

		for stream.Next() {
			event := stream.Current()
			switch event.Type {
			case "content_block_start":
				if event.ContentBlock.Type == "tool_use" {
					isToolUse = true
					currentToolID = event.ContentBlock.ID
					currentToolName = event.ContentBlock.Name
					currentToolInput = ""
				}
			case "content_block_delta":
				if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					sw.Send(&schema.Message{
						Role:    schema.Assistant,
						Content: event.Delta.Text,
					}, nil)
				} else if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
					currentToolInput += event.Delta.PartialJSON
				}
			case "content_block_stop":
				if isToolUse {
					// 工具调用块结束，发送完整的 tool call 消息
					sw.Send(&schema.Message{
						Role: schema.Assistant,
						ToolCalls: []schema.ToolCall{
							{
								ID:   currentToolID,
								Type: "function",
								Function: schema.FunctionCall{
									Name:      currentToolName,
									Arguments: currentToolInput,
								},
							},
						},
					}, nil)
					isToolUse = false
					currentToolID = ""
					currentToolName = ""
					currentToolInput = ""
				}
			}
		}

		if err := stream.Err(); err != nil && err != io.EOF {
			sw.Send(nil, fmt.Errorf("Anthropic 流式读取失败: %w", err))
		}
	}()

	return sr, nil
}

// BindTools 绑定工具（已废弃，使用 WithTools）
func (m *AnthropicChatModel) BindTools(tools []*schema.ToolInfo) error {
	return m.bindTools(tools)
}

// WithTools 返回绑定了工具的新实例（并发安全）
func (m *AnthropicChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	newModel := &AnthropicChatModel{
		client:  m.client,
		modelID: m.modelID,
	}
	if err := newModel.bindTools(tools); err != nil {
		return nil, err
	}
	return newModel, nil
}

// bindTools 内部工具绑定实现
func (m *AnthropicChatModel) bindTools(tools []*schema.ToolInfo) error {
	toolParams := make([]anthropic.ToolParam, 0, len(tools))
	for _, t := range tools {
		toolParam := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Desc),
		}

		if t.ParamsOneOf != nil {
			jsonSchema, err := t.ParamsOneOf.ToJSONSchema()
			if err == nil {
				schemaBytes, err := json.Marshal(jsonSchema)
				if err == nil {
					var schemaMap map[string]any
					if json.Unmarshal(schemaBytes, &schemaMap) == nil {
						inputSchema := anthropic.ToolInputSchemaParam{
							Type: "object",
						}
						if props, ok := schemaMap["properties"]; ok {
							inputSchema.Properties = props
						}
						if req, ok := schemaMap["required"].([]any); ok {
							required := make([]string, 0, len(req))
							for _, r := range req {
								if s, ok := r.(string); ok {
									required = append(required, s)
								}
							}
							inputSchema.Required = required
						}
						toolParam.InputSchema = inputSchema
					}
				}
			}
		}

		toolParams = append(toolParams, toolParam)
	}

	m.tools = make([]anthropic.ToolUnionParam, 0, len(toolParams))
	for _, tp := range toolParams {
		m.tools = append(m.tools, anthropic.ToolUnionParam{OfTool: &tp})
	}
	return nil
}

// buildParams 构建 Anthropic 请求参数，提取 system prompt
func (m *AnthropicChatModel) buildParams(input []*schema.Message) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     m.modelID,
		MaxTokens: 4096,
	}

	var messages []anthropic.MessageParam

	for _, msg := range input {
		switch msg.Role {
		case schema.System:
			params.System = []anthropic.TextBlockParam{
				{Text: msg.Content},
			}
		case schema.User:
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case schema.Assistant:
			if len(msg.ToolCalls) > 0 {
				// 包含 tool_use 的 assistant 消息
				var blocks []anthropic.ContentBlockParamUnion
				if msg.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
				}
				for _, tc := range msg.ToolCalls {
					var inputMap map[string]any
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputMap); err != nil {
						inputMap = make(map[string]any)
					}
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, inputMap, tc.Function.Name))
				}
				messages = append(messages, anthropic.NewAssistantMessage(blocks...))
			} else {
				messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
			}
		case schema.Tool:
			// 工具结果消息
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
			))
		}
	}

	params.Messages = messages

	if len(m.tools) > 0 {
		params.Tools = m.tools
	}

	return params
}

// convertResponse 将 Anthropic 响应转为 eino 消息
func (m *AnthropicChatModel) convertResponse(resp *anthropic.Message) *schema.Message {
	msg := &schema.Message{
		Role: schema.Assistant,
	}

	var textContent string
	var toolCalls []schema.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			inputBytes, marshalErr := json.Marshal(block.Input)
			if marshalErr != nil {
				// 序列化失败时记录日志，使用空对象作为 fallback
				logger.Error("序列化工具调用参数失败", zap.Error(marshalErr))
				inputBytes = []byte("{}")
			}
			toolCalls = append(toolCalls, schema.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      block.Name,
					Arguments: string(inputBytes),
				},
			})
		}
	}

	msg.Content = textContent
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		msg.ResponseMeta = &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     int(resp.Usage.InputTokens),
				CompletionTokens: int(resp.Usage.OutputTokens),
				TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
			},
		}
	}

	return msg
}

// String 返回模型标识
func (m *AnthropicChatModel) String() string {
	return fmt.Sprintf("anthropic/%s", m.modelID)
}

// IsCallbacksEnabled 回调支持
func (m *AnthropicChatModel) IsCallbacksEnabled() bool {
	return true
}

// GetType 返回组件类型
func (m *AnthropicChatModel) GetType() string {
	return "ChatModel"
}
