package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type pptExpandedOutlineApproval struct {
	Approved bool     `json:"approved"`
	Notes    []string `json:"notes,omitempty"`
	Outline  string   `json:"outline"`
}

// approvePPTExpandedOutline is the focused quality gate after deterministic
// outline supplementation and before content enrichment/rendering.
func (a *pptGenerationAgent) approvePPTExpandedOutline(ctx context.Context, state pptChainState) (pptChainState, error) {
	state.expanded = approvePPTOutlinePlan(state.expanded, state.analysis)
	state.outline = renderPPTOutlineMarkdown(state.expanded)

	if a.model == nil {
		state.outlineApproved = true
		state.approvalNotes = append(state.approvalNotes, "deterministic")
		return state, nil
	}

	strategy := pptExpandedOutlineApprovalPromptStrategy()
	userPrompt := ""
	if state.input.Request != nil {
		userPrompt = strings.TrimSpace(state.input.Request.Prompt)
	}
	reviewed, err := a.model.Generate(ctx, GenerationPrompt{
		AgentName:    a.name + "_expanded_outline_approval",
		System:       strategy.System,
		User:         userPrompt,
		Context:      appendPPTApprovalContext(state.input.Context, state.outline, state.expanded, state.analysis),
		OutputFormat: strategy.OutputFormat,
	})
	if err != nil || strings.TrimSpace(reviewed) == "" {
		state.outlineApproved = true
		state.approvalNotes = append(state.approvalNotes, "fallback_after_model_error")
		return state, nil
	}

	var result pptExpandedOutlineApproval
	jsonText := extractFirstJSONObject(strings.TrimSpace(reviewed))
	if jsonText == "" || json.Unmarshal([]byte(jsonText), &result) != nil {
		state.outlineApproved = true
		state.approvalNotes = append(state.approvalNotes, "fallback_after_invalid_response")
		return state, nil
	}

	if strings.TrimSpace(result.Outline) != "" {
		if parsed, ok := parsePPTOutlineMarkdown(result.Outline); ok && len(parsed.Slides) > 0 {
			state.expanded = approvePPTOutlinePlan(parsed, state.analysis)
			state.outline = renderPPTOutlineMarkdown(state.expanded)
			state.outlineApproved = true
			state.approvalNotes = append(state.approvalNotes, result.Notes...)
			if !result.Approved {
				state.approvalNotes = append(state.approvalNotes, "revised_before_approval")
			}
			return state, nil
		}
	}

	state.outlineApproved = true
	state.approvalNotes = append(state.approvalNotes, result.Notes...)
	state.approvalNotes = append(state.approvalNotes, "kept_original_after_invalid_outline")
	return state, nil
}

func approvePPTOutlinePlan(plan pptOutlinePlan, analysis learningContentAnalysis) pptOutlinePlan {
	if strings.TrimSpace(plan.Title) == "" {
		plan.Title = analysis.Topic
	}
	plan = ensurePPTPlanFrame(plan)
	for i := range plan.Slides {
		normalizePPTBullets(&plan.Slides[i])
		for i > 1 && i < len(plan.Slides)-1 && len(plan.Slides[i].Bullets) < 3 {
			plan.Slides[i].Bullets = append(plan.Slides[i].Bullets, supplementBullet(plan.Slides[i].Title, len(plan.Slides[i].Bullets)+1))
		}
	}
	return deduplicatePPTPlanBullets(plan)
}

func appendPPTApprovalContext(contextValue, outline string, expanded pptOutlinePlan, analysis learningContentAnalysis) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nPPT_EXPANDED_OUTLINE_APPROVAL\n")
	b.WriteString("Original outline:\n")
	b.WriteString(strings.TrimSpace(outline))
	b.WriteString("\n\nSupplemented outline:\n")
	b.WriteString(renderPPTOutlineMarkdown(expanded))
	if len(analysis.Gaps) > 0 {
		b.WriteString("\n\nKnown content gaps:\n")
		b.WriteString(strings.Join(analysis.Gaps, "\n"))
	}
	b.WriteString("\n\nApproval rules:\n")
	b.WriteString("- Preserve source-grounded claims and the user's requested focus.\n")
	b.WriteString("- Remove duplicated, generic, or planning-only slide points.\n")
	b.WriteString("- Keep a clear learning arc: framing, concepts, mechanism, application, and synthesis.\n")
	b.WriteString("- Return a complete revised outline even when the input is already acceptable.\n")
	return strings.TrimSpace(b.String())
}

func renderPPTOutlineMarkdown(plan pptOutlinePlan) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(firstNonEmpty(plan.Title, "PPT"))
	b.WriteString("\n")
	for _, slide := range plan.Slides {
		title := strings.TrimSpace(slide.Title)
		if title == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(title)
		b.WriteString("\n")
		for _, bullet := range slide.Bullets {
			bullet = strings.TrimSpace(bullet)
			if bullet == "" {
				continue
			}
			b.WriteString("  - ")
			b.WriteString(bullet)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func appendPPTLayoutDirectionToContext(contextValue string, plan pptOutlinePlan) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(contextValue))
	b.WriteString("\n\nPPT_LAYOUT_DIRECTION\n")
	b.WriteString("Do not reuse a fixed CSS template. Choose layout and visual emphasis from each slide's content.\n")
	for i, slide := range plan.Slides {
		layout := "two-column"
		if i > 0 {
			layout = pptSlideLayoutForIndex(i, slide)
		}
		b.WriteString(fmt.Sprintf(
			"Slide %02d | title=%q | layout=%s | bullets=%d\n",
			i+1,
			strings.TrimSpace(slide.Title),
			layout,
			len(slide.Bullets),
		))
	}
	b.WriteString("Use editable HTML text, vary composition across slides, and let content density determine spacing and typography.\n")
	return strings.TrimSpace(b.String())
}

func pptExpandedOutlineApprovalPromptStrategy() generationPromptStrategy {
	return generationPromptStrategy{
		System: "You are the final PPT outline refinement and approval gate. " +
			"Review the supplemented outline against the source material and return a corrected, presentation-ready outline. " +
			"Preserve useful detail, remove duplication and planning labels, repair weak slide titles, and keep the learning sequence coherent. " +
			"Do not invent unsupported facts.",
		OutputFormat: `Return only JSON:
{
  "approved": true,
  "notes": ["short issue or decision"],
  "outline": "# Deck title
- Slide title
  - concrete point
  - concrete point"
}
The outline must contain a cover, agenda, substantive content slides, and a closing slide. ` +
			"Use the same Markdown outline format as the input.",
	}
}
