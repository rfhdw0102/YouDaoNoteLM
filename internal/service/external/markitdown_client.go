package external

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type markitdownClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMarkitdownClient 创建 MarkItDown HTTP 客户端
func NewMarkitdownClient(baseURL string) MarkitdownClient {
	return &markitdownClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Convert 本地文件转 Markdown（上传文件到 MarkItDown 服务）
func (c *markitdownClient) Convert(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return c.ConvertReader(filepath.Base(filePath), file)
}

// ConvertReader 通过 io.Reader 上传文件转 Markdown
func (c *markitdownClient) ConvertReader(filename string, reader io.Reader) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("创建表单文件失败: %w", err)
	}
	if _, err := io.Copy(part, reader); err != nil {
		return "", fmt.Errorf("写入文件内容失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭multipart writer失败: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/convert", writer.FormDataContentType(), body)
	if err != nil {
		return "", fmt.Errorf("请求MarkItDown失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("MarkItDown返回错误 %d，且读取响应体失败: %w", resp.StatusCode, readErr)
		}
		return "", fmt.Errorf("MarkItDown返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// MarkItDown Python 服务返回 {"filename": "...", "markdown": "..."}
	var result struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// 降级：返回原始响应
		return string(respBody), nil
	}

	logger.Info("MarkItDown转换成功", zap.String("file", filename))
	return result.Markdown, nil
}

// ConvertFromURL 网页 URL 转 Markdown
func (c *markitdownClient) ConvertFromURL(url string) (string, error) {
	// MarkItDown 服务的 /convert_url 使用 Form 表单
	formBody := &bytes.Buffer{}
	writer := multipart.NewWriter(formBody)
	if err := writer.WriteField("url", url); err != nil {
		return "", fmt.Errorf("写入 URL 字段失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭writer失败: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/convert_url", writer.FormDataContentType(), formBody)
	if err != nil {
		return "", fmt.Errorf("请求MarkItDown URL转换失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("MarkItDown URL转换返回错误 %d，且读取响应体失败: %w", resp.StatusCode, readErr)
		}
		return "", fmt.Errorf("MarkItDown URL转换返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// MarkItDown Python 服务返回 {"url": "...", "markdown": "..."} 或 {"url": "...", "markdown": "", "message": "..."}
	var result struct {
		Markdown string `json:"markdown"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	if result.Markdown == "" && result.Message != "" {
		logger.Warn("MarkItDown URL转换无内容", zap.String("url", url), zap.String("message", result.Message))
		return "", fmt.Errorf("%s", result.Message)
	}

	logger.Info("MarkItDown URL转换成功", zap.String("url", url))
	return result.Markdown, nil
}
