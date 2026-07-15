// internal/service/external/llm/anthropic_client.go
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultAnthropicURL = "https://api.anthropic.com"

// AnthropicClient Anthropic Claude 客户端
type AnthropicClient struct {
	apiURL string
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropicClient 创建 Anthropic 客户端
func NewAnthropicClient(apiURL, apiKey, model string) *AnthropicClient {
	if apiURL == "" {
		apiURL = defaultAnthropicURL
	}
	return &AnthropicClient{
		apiURL: apiURL,
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

// anthropicMessage Anthropic 消息格式
type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string 或 []contentBlock
}

// contentBlock 内容块（用于图片等）
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// anthropicRequest Anthropic API 请求
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

// anthropicTool Anthropic 工具定义
type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// anthropicResponse Anthropic API 响应
type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type      string         `json:"type"`
		Text      string         `json:"text,omitempty"`
		ID        string         `json:"id,omitempty"`
		Name      string         `json:"name,omitempty"`
		Input     map[string]any `json:"input,omitempty"`
		ToolUseID string         `json:"tool_use_id,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

func (c *AnthropicClient) Name() string {
	return "anthropic"
}

func (c *AnthropicClient) Chat(messages []Message) (string, error) {
	resp, err := c.doRequest(messages, nil)
	if err != nil {
		return "", err
	}

	// 提取文本内容
	for _, content := range resp.Content {
		if content.Type == "text" {
			return content.Text, nil
		}
	}

	return "", fmt.Errorf("Anthropic 响应中没有文本内容")
}

func (c *AnthropicClient) ChatWithTools(messages []Message, tools []ToolDef) (*ToolCallResponse, error) {
	// 转换工具定义
	anthropicTools := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		anthropicTools = append(anthropicTools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	resp, err := c.doRequest(messages, anthropicTools)
	if err != nil {
		return nil, err
	}

	result := &ToolCallResponse{}

	// 解析响应内容
	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			result.Content = content.Text
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        content.ID,
				Name:      content.Name,
				Arguments: content.Input,
			})
		}
	}

	return result, nil
}

func (c *AnthropicClient) doRequest(messages []Message, tools []anthropicTool) (*anthropicResponse, error) {
	// 分离 system 消息
	var systemMsg string
	var chatMessages []Message
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
		} else {
			chatMessages = append(chatMessages, msg)
		}
	}

	// 转换消息格式
	anthropicMsgs := make([]anthropicMessage, 0, len(chatMessages))
	for _, msg := range chatMessages {
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemMsg,
		Messages:  anthropicMsgs,
		Tools:     tools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/v1/messages", c.apiURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Anthropic API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result anthropicResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}
