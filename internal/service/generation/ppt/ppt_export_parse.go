package ppt

import (
	bizerrors "YoudaoNoteLm/pkg/errors"
	"context"
	"html"
	"regexp"
	"strings"
)

func Export(ctx context.Context, content, title, templateID string) (*ExportResult, error) {
	filename := resolveExportFilename(title, content, "ppt-export", ".pptx")

	trimmedTemplateID := strings.TrimSpace(templateID)
	var (
		data []byte
		err  error
	)

	if trimmedTemplateID == "" {
		data, err = exportPPTWithDefaultEngine(ctx, content, strings.TrimSuffix(filename, ".pptx"))
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "build ppt export failed", err)
		}
	} else {
		slides, parseErr := parsePPTExportSlides(content)
		if parseErr != nil {
			return nil, parseErr
		}
		template, templateErr := resolvePPTExportTemplate(trimmedTemplateID)
		if templateErr != nil {
			return nil, templateErr
		}
		data, err = buildPPTXBytes(slides, strings.TrimSuffix(filename, ".pptx"), template)
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalServiceError, "build ppt export failed", err)
		}
	}

	return &ExportResult{
		Filename:    filename,
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Data:        data,
	}, nil
}

func resolvePPTExportTemplate(templateID string) (pptExportTemplate, error) {
	id := strings.ToLower(strings.TrimSpace(templateID))
	template, ok := pptExportTemplates[id]
	if !ok {
		return pptExportTemplate{}, bizerrors.New(bizerrors.CodeInvalidParam, "unsupported ppt template")
	}
	return template, nil
}

func parsePPTExportSlides(content string) ([]pptExportSlide, error) {
	matches := pptSectionPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "ppt export content must contain at least one <section> slide")
	}

	slides := make([]pptExportSlide, 0, len(matches))
	for _, match := range matches {
		body := match[1]
		title := extractPPTSlideTitle(body)
		bullets := extractPPTSlideBullets(body)
		if title == "" && len(bullets) == 0 {
			continue
		}
		slides = append(slides, pptExportSlide{
			Title:   title,
			Bullets: bullets,
		})
	}

	if len(slides) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "ppt export content does not contain any valid slides")
	}
	return slides, nil
}

func extractPPTSlideTitle(section string) string {
	for _, pattern := range []*regexp.Regexp{pptH1Pattern, pptH2Pattern} {
		match := pattern.FindStringSubmatch(section)
		if len(match) >= 2 {
			return normalizePPTExportText(match[1])
		}
	}
	return ""
}

func extractPPTSlideBullets(section string) []string {
	matches := pptBulletPattern.FindAllStringSubmatch(section, -1)
	bullets := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		bullets = append(bullets, normalizePPTExportText(match[1]))
	}
	return uniqueNonEmpty(bullets)
}

func normalizePPTExportText(value string) string {
	value = strings.ReplaceAll(value, "&nbsp;", " ")
	value = stripSimpleHTML(value)
	value = html.UnescapeString(value)

	if cleaned, ok := stripFencedCodeBlockForPPT(value); ok {
		return cleaned
	}

	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return cleanPPTVisibleText(value)
}
