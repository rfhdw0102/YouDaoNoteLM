// internal/service/external/llm/openai_client.go
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type openaiClient struct {
	name   string
	apiURL string
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAIClient 创建 OpenAI 兼容 LLM 客户端
// 支持 OpenAI、通义千问、DeepSeek 等 OpenAI 兼容 API
func NewOpenAIClient(name, apiURL, apiKey, model string) LLMClient {
	return &openaiClient{
		name:   name,
		apiURL: apiURL,
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// openaiRequest OpenAI API 请求结构
type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	Tools    []openaiTool    `json:"tools,omitempty"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// openaiResponse OpenAI API 响应结构
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content   string           `json:"content"`
			ToolCalls []openaiToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 普通对话
func (c *openaiClient) Chat(messages []Message) (string, error) {
	resp, err := c.doRequest(messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

// ChatWithTools 带工具调用的对话
func (c *openaiClient) ChatWithTools(messages []Message, tools []ToolDef) (*ToolCallResponse, error) {
	resp, err := c.doRequest(messages, tools)
	if err != nil {
		return nil, err
	}

	result := &ToolCallResponse{
		Content: resp.Choices[0].Message.Content,
	}

	for _, tc := range resp.Choices[0].Message.ToolCalls {
		args := make(map[string]any)
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			logger.Warn("解析工具参数失败",
				zap.String("tool", tc.Function.Name),
				zap.String("raw_args", tc.Function.Arguments),
				zap.Error(err),
			)
			continue
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return result, nil
}

// doRequest 执行 OpenAI API 请求
func (c *openaiClient) doRequest(messages []Message, tools []ToolDef) (*openaiResponse, error) {
	start := time.Now()

	// 转换消息格式
	oaiMessages := make([]openaiMessage, len(messages))
	for i, m := range messages {
		oaiMessages[i] = openaiMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		// 转换 tool_calls（assistant 消息携带）
		if len(m.ToolCalls) > 0 {
			oaiMessages[i].ToolCalls = make([]openaiToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				argsBytes, err := json.Marshal(tc.Arguments)
				if err != nil {
					return nil, fmt.Errorf("序列化工具调用参数失败: %w", err)
				}
				oaiMessages[i].ToolCalls[j] = openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Name,
						Arguments: string(argsBytes),
					},
				}
			}
		}
	}

	reqBody := openaiRequest{
		Model:    c.model,
		Messages: oaiMessages,
	}

	// 转换工具定义
	if len(tools) > 0 {
		reqBody.Tools = make([]openaiTool, len(tools))
		for i, t := range tools {
			reqBody.Tools[i] = openaiTool{
				Type: "function",
				Function: openaiFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			}
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	serializeMs := time.Since(start).Milliseconds()

	req, err := http.NewRequest("POST", c.apiURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	reqStart := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM请求失败: %w", err)
	}
	defer resp.Body.Close()

	networkMs := time.Since(reqStart).Milliseconds()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取LLM响应失败: %w", err)
	}

	readMs := time.Since(reqStart).Milliseconds() - networkMs

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("解析LLM响应失败: %w", err)
	}

	if oaiResp.Error != nil {
		return nil, fmt.Errorf("LLM错误: %s", oaiResp.Error.Message)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM返回空结果")
	}

	totalMs := time.Since(start).Milliseconds()
	logger.Info("LLM调用成功",
		zap.String("model", c.model),
		zap.Int("tool_calls", len(oaiResp.Choices[0].Message.ToolCalls)),
		zap.Int64("total_ms", totalMs),
		zap.Int64("serialize_ms", serializeMs),
		zap.Int64("network_ms", networkMs),
		zap.Int64("read_ms", readMs),
		zap.Int("resp_bytes", len(respBody)),
	)

	return &oaiResp, nil
}
