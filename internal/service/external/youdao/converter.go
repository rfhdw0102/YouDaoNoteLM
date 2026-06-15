package youdao

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// NoteConverter 有道云笔记格式转换器
type NoteConverter interface {
	// ConvertToMarkdown 将 XML/JSON 格式的有道云笔记内容转换为 Markdown
	ConvertToMarkdown(content string, formatType string) (string, error)
	// ConvertFile 将文件内容转换为 Markdown
	ConvertFile(filePath string) (string, error)
}

// noteConverter 实现
type noteConverter struct {
	scriptPath string
}

// NewNoteConverter 创建转换器实例
// scriptPath: convert_to_markdown.py 脚本的路径
func NewNoteConverter(scriptPath string) NoteConverter {
	return &noteConverter{scriptPath: scriptPath}
}

// convertResult Python 脚本返回的结果
type convertResult struct {
	Success bool   `json:"success"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

// ConvertToMarkdown 将内容转换为 Markdown
func (c *noteConverter) ConvertToMarkdown(content string, formatType string) (string, error) {
	// 检查脚本是否存在
	if _, err := os.Stat(c.scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("转换脚本不存在: %s", c.scriptPath)
	}

	// 创建临时文件保存内容
	tmpFile, err := os.CreateTemp("", "youdao-convert-*.tmp")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer func() {
		os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	// 调用转换脚本
	return c.convertFileWithScript(tmpFile.Name())
}

// ConvertFile 将文件转换为 Markdown
func (c *noteConverter) ConvertFile(filePath string) (string, error) {
	// 检查脚本是否存在
	if _, err := os.Stat(c.scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("转换脚本不存在: %s", c.scriptPath)
	}

	// 检查输入文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("输入文件不存在: %s", filePath)
	}

	return c.convertFileWithScript(filePath)
}

// convertFileWithScript 调用 Python 脚本进行转换
func (c *noteConverter) convertFileWithScript(filePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取脚本所在目录
	scriptDir := filepath.Dir(c.scriptPath)

	// 构建命令：python convert_to_markdown.py <input_file>
	cmd := exec.CommandContext(ctx, "python", c.scriptPath, filePath)
	cmd.Dir = scriptDir

	// 执行命令
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("转换超时（30s）")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("转换失败: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("转换失败: %w", err)
	}

	// 解析 JSON 结果
	var result convertResult
	if err := json.Unmarshal(output, &result); err != nil {
		// 如果不是 JSON，返回原始输出
		content := strings.TrimSpace(string(output))
		if content == "" {
			return "", fmt.Errorf("转换结果为空")
		}
		return content, nil
	}

	if !result.Success {
		return "", fmt.Errorf("转换失败: %s", result.Error)
	}

	return result.Content, nil
}
