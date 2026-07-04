package markitdown

import (
	"bytes"
	"context"
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

// ConvertError 转换错误，包含用户友好的消息
type ConvertError struct {
	Code       string // 错误码：timeout, network, forbidden, not_found, server_error, unknown
	UserMsg    string // 用户友好的错误消息
	DetailMsg  string // 详细技术信息（用于日志）
	HTTPStatus int    // HTTP 状态码（如果适用）
}

func (e *ConvertError) Error() string {
	return e.DetailMsg
}

// newConvertError 创建转换错误
func newConvertError(code, userMsg, detailMsg string, httpStatus int) *ConvertError {
	return &ConvertError{
		Code:       code,
		UserMsg:    userMsg,
		DetailMsg:  detailMsg,
		HTTPStatus: httpStatus,
	}
}

const (
	defaultTimeout     = 180 * time.Second // 默认超时
	fileConvertTimeout = 180 * time.Second // 文件转换超时（大文件 + LLM 结构化需要更多时间）
	urlConvertTimeout  = 120 * time.Second // URL 转换超时（网页抓取需要更多时间）
)

type client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建 MarkItDown HTTP 客户端
func NewClient(baseURL string) Client {
	return &client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// Convert 本地文件转 Markdown（上传文件到 MarkItDown 服务）
func (c *client) Convert(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logger.Errorf("关闭文件失败:%s", err)
		}
	}(file)

	ctx, cancel := context.WithTimeout(context.Background(), fileConvertTimeout)
	defer cancel()

	return c.ConvertReaderWithContext(ctx, filepath.Base(filePath), file)
}

// ConvertReader 通过 io.Reader 上传文件转 Markdown
func (c *client) ConvertReader(filename string, reader io.Reader) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fileConvertTimeout)
	defer cancel()

	return c.ConvertReaderWithContext(ctx, filename, reader)
}

// ConvertReaderWithContext 通过 io.Reader 上传文件转 Markdown（带 context）
func (c *client) ConvertReaderWithContext(ctx context.Context, filename string, reader io.Reader) (string, error) {
	start := time.Now()

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/convert", body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	logger.Info("开始请求 MarkItDown 转换", zap.String("file", filename))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Error("MarkItDown 请求超时",
				zap.String("file", filename),
				zap.Duration("elapsed", time.Since(start)),
				zap.Duration("timeout", fileConvertTimeout),
			)
			return "", fmt.Errorf("请求MarkItDown超时（%v）", fileConvertTimeout)
		}
		logger.Error("MarkItDown 请求失败",
			zap.String("file", filename),
			zap.Duration("elapsed", time.Since(start)),
			zap.Error(err),
		)
		return "", fmt.Errorf("请求MarkItDown失败: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf("关闭缓冲区失败:%s", err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusRequestTimeout {
		logger.Error("MarkItDown 服务端转换超时",
			zap.String("file", filename),
			zap.Duration("elapsed", time.Since(start)),
		)
		return "", fmt.Errorf("MarkItDown 服务端转换超时")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("MarkItDown返回错误 %d（读取响应体失败: %v）", resp.StatusCode, readErr)
		}
		logger.Error("MarkItDown 返回错误",
			zap.String("file", filename),
			zap.Int("status", resp.StatusCode),
			zap.Duration("elapsed", time.Since(start)),
			zap.String("response", string(respBody)),
		)
		return "", fmt.Errorf("MarkItDown返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// MarkItDown Python 服务返回 {"filename": "...", "markdown": "..."}
	var result struct {
		Markdown string `json:"markdown"`
		Cached   bool   `json:"cached"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// 降级：返回原始响应
		logger.Info("MarkItDown 转换完成（降级解析）",
			zap.String("file", filename),
			zap.Duration("elapsed", time.Since(start)),
		)
		return string(respBody), nil
	}

	logger.Info("MarkItDown 转换成功",
		zap.String("file", filename),
		zap.Bool("cached", result.Cached),
		zap.Int("content_len", len(result.Markdown)),
		zap.Duration("elapsed", time.Since(start)),
	)
	return result.Markdown, nil
}

// ConvertFromURL 网页 URL 转 Markdown
func (c *client) ConvertFromURL(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), urlConvertTimeout)
	defer cancel()

	return c.ConvertFromURLWithContext(ctx, url)
}

// ConvertFromURLWithContext 网页 URL 转 Markdown（带 context）
func (c *client) ConvertFromURLWithContext(ctx context.Context, url string) (string, error) {
	start := time.Now()

	// 在传入的 ctx 基础上叠加超时控制，确保单个请求不会无限等待
	ctx, cancel := context.WithTimeout(ctx, urlConvertTimeout)
	defer cancel()

	// MarkItDown 服务的 /convert_url 使用 Form 表单
	formBody := &bytes.Buffer{}
	writer := multipart.NewWriter(formBody)
	if err := writer.WriteField("url", url); err != nil {
		return "", fmt.Errorf("写入表单字段失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭writer失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/convert_url", formBody)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	logger.Info("开始请求 MarkItDown URL 转换", zap.String("url", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Error("MarkItDown URL 转换超时",
				zap.String("url", url),
				zap.Duration("elapsed", time.Since(start)),
				zap.Duration("timeout", urlConvertTimeout),
			)
			return "", newConvertError(
				"timeout",
				"网页内容获取超时，请稍后重试或检查网址是否可访问",
				fmt.Sprintf("请求MarkItDown URL转换超时（%v）", urlConvertTimeout),
				0,
			)
		}
		logger.Error("MarkItDown URL 转换请求失败",
			zap.String("url", url),
			zap.Duration("elapsed", time.Since(start)),
			zap.Error(err),
		)
		return "", newConvertError(
			"network",
			"网络连接失败，请检查网络后重试",
			fmt.Sprintf("请求MarkItDown URL转换失败: %v", err),
			0,
		)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf("关闭缓冲区失败:%s", err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusRequestTimeout {
		logger.Error("MarkItDown 服务端 URL 转换超时",
			zap.String("url", url),
			zap.Duration("elapsed", time.Since(start)),
		)
		return "", newConvertError(
			"timeout",
			"网页内容获取超时，请稍后重试",
			"MarkItDown 服务端转换超时",
			http.StatusRequestTimeout,
		)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", newConvertError("server_error", "网页内容获取失败", fmt.Sprintf("读取响应体失败: %v", readErr), resp.StatusCode)
		}
		detailMsg := fmt.Sprintf("MarkItDown URL转换返回错误 %d: %s", resp.StatusCode, string(respBody))

		// 根据 HTTP 状态码返回用户友好的错误信息
		var userMsg string
		var code string
		switch resp.StatusCode {
		case http.StatusForbidden:
			code = "forbidden"
			userMsg = "网页拒绝访问，该网站可能限制了外部访问"
		case http.StatusNotFound:
			code = "not_found"
			userMsg = "网页不存在，请检查网址是否正确"
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			code = "server_error"
			userMsg = "网页服务暂时不可用，请稍后重试"
		default:
			code = "server_error"
			userMsg = "网页内容获取失败，请稍后重试"
		}

		logger.Error("MarkItDown URL 转换返回错误",
			zap.String("url", url),
			zap.Int("status", resp.StatusCode),
			zap.Duration("elapsed", time.Since(start)),
			zap.String("code", code),
		)
		return "", newConvertError(code, userMsg, detailMsg, resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// MarkItDown Python 服务返回 {"url": "...", "markdown": "..."} 或 {"url": "...", "markdown": "", "message": "..."}
	var result struct {
		Markdown string `json:"markdown"`
		Message  string `json:"message"`
		Cached   bool   `json:"cached"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		logger.Info("MarkItDown URL 转换完成（降级解析）",
			zap.String("url", url),
			zap.Duration("elapsed", time.Since(start)),
		)
		return string(respBody), nil
	}

	if result.Markdown == "" && result.Message != "" {
		logger.Warn("MarkItDown URL转换无内容",
			zap.String("url", url),
			zap.String("message", result.Message),
			zap.Duration("elapsed", time.Since(start)),
		)
		return "", fmt.Errorf("%s", result.Message)
	}

	logger.Info("MarkItDown URL 转换成功",
		zap.String("url", url),
		zap.Bool("cached", result.Cached),
		zap.Int("content_len", len(result.Markdown)),
		zap.Duration("elapsed", time.Since(start)),
	)
	return result.Markdown, nil
}
