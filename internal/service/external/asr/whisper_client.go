// internal/service/external/asr/whisper_client.go
package asr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const defaultWhisperURL = "https://api.openai.com/v1"

// WhisperClient OpenAI Whisper ASR 客户端
type WhisperClient struct {
	apiURL string
	apiKey string
	client *http.Client
}

// NewWhisperClient 创建 Whisper 客户端
func NewWhisperClient(apiURL, apiKey string) ASRService {
	if apiURL == "" {
		apiURL = defaultWhisperURL
	}
	return &WhisperClient{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// whisperResponse Whisper API 响应
type whisperResponse struct {
	Text string `json:"text"`
}

func (c *WhisperClient) Transcribe(filePath string) (string, error) {
	// 打开音频文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer file.Close()

	// 创建 multipart 请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("创建表单文件失败: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("写入文件内容失败: %w", err)
	}

	// 添加 model 字段
	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("写入 model 字段失败: %w", err)
	}

	// 添加语言字段（可选，提高准确率）
	if err := writer.WriteField("language", "zh"); err != nil {
		return "", fmt.Errorf("写入 language 字段失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭multipart writer失败: %w", err)
	}

	// 创建请求
	url := fmt.Sprintf("%s/audio/transcriptions", c.apiURL)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 Whisper API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Whisper API 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	var result whisperResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	return result.Text, nil
}

// 确保 WhisperClient 实现了 ASRService 接口
var _ ASRService = (*WhisperClient)(nil)
