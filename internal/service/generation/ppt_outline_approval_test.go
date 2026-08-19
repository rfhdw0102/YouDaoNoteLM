package generation

import (
	"context"
	"strings"
	"testing"
)

type scriptedPPTModel struct {
	response string
	err      error
}

func (m scriptedPPTModel) Generate(context.Context, GenerationPrompt) (string, error) {
	return m.response, m.err
}

func TestApprovePPTExpandedOutlineRefinesAndApprovesPlan(t *testing.T) {
	agent := &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:  "ppt",
			model: scriptedPPTModel{response: `{"approved":false,"notes":["duplicate removed"],"outline":"# Learning\n- Cover\n  - Learning\n- Agenda\n  - Core idea\n- Core idea\n  - Definition and mechanism\n  - Evidence and example\n  - Common boundary\n- Closing\n  - Summary"}`},
		},
	}
	state := pptChainState{
		input: generationAgentInput{
			Request: &GenerationRequest{Prompt: "Explain the topic"},
			Context: "source material",
		},
		analysis: learningContentAnalysis{Topic: "Learning"},
		expanded: pptOutlinePlan{
			Title: "Learning",
			Slides: []pptSlidePlan{
				{Title: "Cover", Bullets: []string{"Learning"}},
				{Title: "Agenda", Bullets: []string{"Core idea"}},
				{Title: "Core idea", Bullets: []string{"Definition and mechanism", "Evidence and example", "Common boundary"}},
				{Title: "Closing", Bullets: []string{"Summary"}},
			},
		},
	}

	got, err := agent.approvePPTExpandedOutline(context.Background(), state)
	if err != nil {
		t.Fatalf("approvePPTExpandedOutline returned error: %v", err)
	}
	if !got.outlineApproved {
		t.Fatal("expected outline to be approved after refinement")
	}
	if len(got.expanded.Slides) != 4 {
		t.Fatalf("slides = %d, want 4", len(got.expanded.Slides))
	}
	if got.expanded.Slides[2].Title != "Core idea" {
		t.Fatalf("content title = %q", got.expanded.Slides[2].Title)
	}
	if !strings.Contains(got.outline, "Common boundary") {
		t.Fatalf("approved outline lost revised content: %q", got.outline)
	}
}

func TestPPTOrchestrationUsesApprovalAndDropsCSSGeneration(t *testing.T) {
	steps := generationOrchestrationSteps()

	var hasApproval, hasEnrich, hasCSS bool
	for _, step := range steps {
		switch step {
		case "outline_refine_approve":
			hasApproval = true
		case "content_enrich":
			hasEnrich = true
		case "css_generate":
			hasCSS = true
		}
	}
	if !hasApproval || !hasEnrich {
		t.Fatalf("steps = %#v, want approval and enrichment nodes", steps)
	}
	if hasCSS {
		t.Fatalf("steps = %#v, fixed css generation should be removed", steps)
	}
}

func TestAdaptivePPTStyleRespondsToContentDensity(t *testing.T) {
	sparse := adaptivePPTStyleBlock(pptOutlinePlan{
		Title:  "Sparse",
		Slides: []pptSlidePlan{{Title: "Topic", Bullets: []string{"One", "Two"}}},
	}, testPPTTheme())
	dense := adaptivePPTStyleBlock(pptOutlinePlan{
		Title:  "Dense",
		Slides: []pptSlidePlan{{Title: "Topic", Bullets: []string{"1", "2", "3", "4", "5", "6", "7"}}},
	}, testPPTTheme())

	if !strings.Contains(sparse, "padding: 104px 112px") {
		t.Fatalf("sparse style did not use relaxed spacing:\n%s", sparse)
	}
	if !strings.Contains(dense, "padding: 66px 112px") {
		t.Fatalf("dense style did not use compact spacing:\n%s", dense)
	}
	if sparse == dense {
		t.Fatal("adaptive style should vary with content density")
	}
}

func testPPTTheme() pptStyleTheme {
	return pptStyleTheme{
		Name:        "test",
		Primary:     "#0f766e",
		Secondary:   "#c2410c",
		Background:  "#f8fafc",
		Surface:     "#ffffff",
		Text:        "#111827",
		Heading:     "#0f172a",
		Muted:       "#4b5563",
		FontHeading: "Arial",
		FontBody:    "Arial",
	}
}
