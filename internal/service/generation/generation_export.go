// generation_export.go 实现生成内容的导出能力。
//
// 支持将生成结果导出为文件：
//   - 文本类（note/mindmap/quiz）：导出为 Markdown 文件
//   - PPT：委托 ppt 子包导出为 .pptx 文件
//
// 提供文件名生成、标题提取、文件名清理等工具函数。
package generation

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"YoudaoNoteLm/internal/service/generation/ppt"
	bizerrors "YoudaoNoteLm/pkg/errors"
)

var invalidFilenameChars = regexp.MustCompile(`[\\/:*?"<>|]+`)
var invalidFilenameWhitespace = regexp.MustCompile(`[\r\n\t]+`)
var invalidFilenameHyphenSpacing = regexp.MustCompile(`\s*-\s*`)

// Export 根据请求将生成内容导出为对应格式的文件。
func (s *generationService) Export(ctx context.Context, req *GenerationExportRequest) (*GenerationExportResult, error) {
	_ = ctx
	if req == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "export request cannot be empty")
	}

	content := req.Content
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "export content cannot be empty")
	}

	switch req.Type {
	case GenerationTypeNote:
		return newTextExportResult(resolveExportFilename(req.Title, trimmed, "note-export", ".md"), "text/markdown; charset=utf-8", content), nil
	case GenerationTypeMindmap:
		return newTextExportResult(resolveExportFilename(req.Title, trimmed, "mindmap-export", ".md"), "text/markdown; charset=utf-8", content), nil
	case GenerationTypeQuiz:
		var payload any
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "quiz export content must be valid JSON", err)
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "marshal quiz export content failed", err)
		}
		return &GenerationExportResult{
			Filename:    resolveExportFilename(req.Title, trimmed, "quiz-export", ".json"),
			ContentType: "application/json",
			Data:        data,
		}, nil
	case GenerationTypePPT:
		result, err := ppt.Export(ctx, content, req.Title, req.Template)
		if err != nil {
			return nil, err
		}
		return &GenerationExportResult{
			Filename:    result.Filename,
			ContentType: result.ContentType,
			Data:        result.Data,
		}, nil
	default:
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "unsupported export type")
	}
}

// newTextExportResult 构造文本类导出结果。
func newTextExportResult(filename, contentType, content string) *GenerationExportResult {
	return &GenerationExportResult{
		Filename:    filename,
		ContentType: contentType,
		Data:        []byte(content),
	}
}

// resolveExportFilename 按标题、内容首行、回退值的优先级生成导出文件名。
func resolveExportFilename(title, content, fallback, ext string) string {
	for _, candidate := range []string{title, extractExportHeading(content), fallback} {
		base := sanitizeExportFilenameBase(candidate)
		if base != "" {
			return base + ext
		}
	}
	return fallback + ext
}

// sanitizeExportFilenameBase 清理文件名中的非法字符和空白。
func sanitizeExportFilenameBase(value string) string {
	base := strings.TrimSpace(value)
	base = invalidFilenameWhitespace.ReplaceAllString(base, " ")
	base = invalidFilenameChars.ReplaceAllString(base, "-")
	base = strings.Join(strings.Fields(base), " ")
	base = invalidFilenameHyphenSpacing.ReplaceAllString(base, "-")
	return strings.Trim(base, ". -")
}

// extractExportHeading 从内容中提取首个非空行作为标题。
func extractExportHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return ""
}
