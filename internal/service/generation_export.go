package service

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	bizerrors "YoudaoNoteLm/pkg/errors"
)

var invalidFilenameChars = regexp.MustCompile(`[\\/:*?"<>|]+`)
var invalidFilenameWhitespace = regexp.MustCompile(`[\r\n\t]+`)
var invalidFilenameHyphenSpacing = regexp.MustCompile(`\s*-\s*`)

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
		return exportPPT(ctx, content, req.Title, req.Template)
	default:
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "unsupported export type")
	}
}

func newTextExportResult(filename, contentType, content string) *GenerationExportResult {
	return &GenerationExportResult{
		Filename:    filename,
		ContentType: contentType,
		Data:        []byte(content),
	}
}

func resolveExportFilename(title, content, fallback, ext string) string {
	for _, candidate := range []string{title, extractExportHeading(content), fallback} {
		base := sanitizeExportFilenameBase(candidate)
		if base != "" {
			return base + ext
		}
	}
	return fallback + ext
}

func sanitizeExportFilenameBase(value string) string {
	base := strings.TrimSpace(value)
	base = invalidFilenameWhitespace.ReplaceAllString(base, " ")
	base = invalidFilenameChars.ReplaceAllString(base, "-")
	base = strings.Join(strings.Fields(base), " ")
	base = invalidFilenameHyphenSpacing.ReplaceAllString(base, "-")
	return strings.Trim(base, ". -")
}

func extractExportHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return ""
}
