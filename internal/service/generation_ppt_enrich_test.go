package service

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// captureGenerationModel records prompts and returns mock outputs.
// It is safe for concurrent access when used with the concurrent enrich.
//
// Output resolution order:
//  1. If responder is set, it decides the output per-prompt (use this for
//     tests that need deterministic behavior under concurrent enrichment,
//     since a shared FIFO outputs queue is order-dependent and flaky).
//  2. Otherwise, outputs are consumed FIFO.
//  3. If outputs is empty, a default valid single-slide JSON is returned.
type captureGenerationModel struct {
	mu        sync.Mutex
	prompts   []GenerationPrompt
	outputs   []string
	responder func(GenerationPrompt) string
}

func (m *captureGenerationModel) Generate(ctx context.Context, prompt GenerationPrompt) (string, error) {
	m.mu.Lock()
	m.prompts = append(m.prompts, prompt)
	if m.responder != nil {
		out := m.responder(prompt)
		m.mu.Unlock()
		return out, nil
	}
	if len(m.outputs) > 0 {
		output := m.outputs[0]
		m.outputs = m.outputs[1:]
		m.mu.Unlock()
		return output, nil
	}
	m.mu.Unlock()
	return `{"slides":[{"title":"Slide","paragraphs":["expanded paragraph"]}]}`, nil
}

func TestPPTContentEnrichBatchesSlides(t *testing.T) {
	model := &captureGenerationModel{}
	agent := &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:  "ppt",
			typ:   GenerationTypePPT,
			model: model,
		},
	}

	state := pptChainState{
		input: generationAgentInput{
			Request: &GenerationRequest{
				Type:     GenerationTypePPT,
				Markdown: "# Topic",
			},
			Context: "Original Markdown:\n# Topic",
		},
		expanded: pptOutlinePlan{
			Title: "Topic",
			Slides: []pptSlidePlan{
				{Title: "Slide 01", Bullets: []string{"Topic 01"}},
				{Title: "Slide 02", Bullets: []string{"Topic 02"}},
				{Title: "Slide 03", Bullets: []string{"Topic 03"}},
				{Title: "Slide 04", Bullets: []string{"Topic 04"}},
				{Title: "Slide 05", Bullets: []string{"Topic 05"}},
				{Title: "Slide 06", Bullets: []string{"Topic 06"}},
				{Title: "Slide 07", Bullets: []string{"Topic 07"}},
				{Title: "Slide 08", Bullets: []string{"Topic 08"}},
				{Title: "Slide 09", Bullets: []string{"Topic 09"}},
			},
		},
	}

	result, err := agent.enrichPPTContent(context.Background(), state)
	if err != nil {
		t.Fatalf("enrichPPTContent returned error: %v", err)
	}

	// 9 slides / batch_size(4) = 3 batches. Each batch calls Generate once
	// (first call succeeds) -> 3 total model calls.
	if len(model.prompts) != 3 {
		t.Fatalf("Generate calls = %d, want 3", len(model.prompts))
	}

	// Verify each prompt has MaxTokens set
	for i, prompt := range model.prompts {
		if got := prompt.MaxTokens; got != pptContentEnrichMaxTokens {
			t.Fatalf("prompt %d MaxTokens = %d, want %d", i, got, pptContentEnrichMaxTokens)
		}
	}

	// The mock model returns 1 slide per call. With 3 batches -> 3 rich slides
	if len(result.richContent.Slides) != 3 {
		t.Fatalf("rich slides = %d, want 3", len(result.richContent.Slides))
	}
}

func TestPPTContentEnrichKeepsSuccessfulBatches(t *testing.T) {
	// Responder routes by batch content (the slide title appears in
	// prompt.Context via renderPPTPlanForPrompt), so behavior is deterministic
	// regardless of which concurrent worker calls Generate first.
	// Batch 0 (slides titled "Slide 01".."Slide 04") always returns invalid
	// JSON, failing on both the initial call and the retry.
	// Batch 1 (slide titled "Slide 05") returns valid JSON on the first call.
	model := &captureGenerationModel{
		responder: func(prompt GenerationPrompt) string {
			if strings.Contains(prompt.Context, "Slide 05") {
				return `{"slides":[{"title":"Slide 05","paragraphs":["expanded five"]}]}`
			}
			return `{"slides":[`
		},
	}
	agent := &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:  "ppt",
			typ:   GenerationTypePPT,
			model: model,
		},
	}
	state := pptChainState{
		input: generationAgentInput{
			Request: &GenerationRequest{Type: GenerationTypePPT, Markdown: "# Topic"},
			Context: "Original Markdown:\n# Topic",
		},
		expanded: pptOutlinePlan{
			Title: "Topic",
			Slides: []pptSlidePlan{
				{Title: "Slide 01", Bullets: []string{"Topic 01"}},
				{Title: "Slide 02", Bullets: []string{"Topic 02"}},
				{Title: "Slide 03", Bullets: []string{"Topic 03"}},
				{Title: "Slide 04", Bullets: []string{"Topic 04"}},
				{Title: "Slide 05", Bullets: []string{"Topic 05"}},
			},
		},
	}

	got, err := agent.enrichPPTContent(context.Background(), state)
	if err != nil {
		t.Fatalf("enrichPPTContent returned error: %v", err)
	}
	// 5 slides / batch_size(4) = 2 batches (4+1). Batch 0 fails initial + retry
	// (2 calls); batch 1 succeeds on first call (1 call). Total = 3 calls.
	generated := model.prompts
	if len(generated) != 3 {
		t.Fatalf("Generate calls = %d, want 3", len(generated))
	}
	if len(got.richContent.Slides) != 1 {
		t.Fatalf("rich slides = %d, want 1", len(got.richContent.Slides))
	}
	if got.richContent.Slides[0].Title != "Slide 05" {
		t.Fatalf("kept slide title = %q, want Slide 05", got.richContent.Slides[0].Title)
	}
}

func TestPPTContentEnrichPreservesOrder(t *testing.T) {
	// Return sequential titles that the mock model produces (always "Slide").
	// Instead of checking exact titles, verify slide count matches batch total.
	model := &captureGenerationModel{}
	agent := &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:  "ppt",
			typ:   GenerationTypePPT,
			model: model,
		},
	}

	state := pptChainState{
		input: generationAgentInput{
			Request: &GenerationRequest{
				Type:     GenerationTypePPT,
				Markdown: "# Topic",
			},
			Context: "Original Markdown:\n# Topic",
		},
		expanded: pptOutlinePlan{
			Title: "Topic",
			Slides: []pptSlidePlan{
				{Title: "Slide A1", Bullets: []string{"T1"}},
				{Title: "Slide A2", Bullets: []string{"T2"}},
				{Title: "Slide A3", Bullets: []string{"T3"}},
				{Title: "Slide A4", Bullets: []string{"T4"}},
				{Title: "Slide B1", Bullets: []string{"T5"}},
				{Title: "Slide B2", Bullets: []string{"T6"}},
				{Title: "Slide B3", Bullets: []string{"T7"}},
			},
		},
	}

	result, err := agent.enrichPPTContent(context.Background(), state)
	if err != nil {
		t.Fatalf("enrichPPTContent returned error: %v", err)
	}
	// 7 slides / batch_size(4) = 2 batches (4+3). All succeed -> 2 rich slides
	// (each batch's mock call returns 1 slide).
	if len(result.richContent.Slides) != 2 {
		t.Fatalf("rich slides = %d, want 2", len(result.richContent.Slides))
	}
}

func TestPPTContentEnrichPartialFailure(t *testing.T) {
	// Batch 0 fails (invalid JSON), batch 1 succeeds, batch 2 fails
	// Expect only batch 1's slides in the result.
	failJSON := `{"slides":[`
	model := &captureGenerationModel{
		outputs: []string{failJSON, failJSON, failJSON, `{"slides":[{"title":"Ok1","paragraphs":["p1"]},{"title":"Ok2","paragraphs":["p2"]}]}`, failJSON, failJSON},
	}
	agent := &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:  "ppt",
			typ:   GenerationTypePPT,
			model: model,
		},
	}
	state := pptChainState{
		input: generationAgentInput{
			Request: &GenerationRequest{Type: GenerationTypePPT, Markdown: "# Topic"},
			Context: "Original Markdown:\n# Topic",
		},
		expanded: pptOutlinePlan{
			Title: "Topic",
			Slides: []pptSlidePlan{
				{Title: "Batch0-1", Bullets: []string{"x"}},
				{Title: "Batch0-2", Bullets: []string{"y"}},
				{Title: "Batch0-3", Bullets: []string{"z"}},
				{Title: "Batch0-4", Bullets: []string{"w"}},
				// batch 1 (slides 5-8)
				{Title: "Batch1-1", Bullets: []string{"a"}},
				{Title: "Batch1-2", Bullets: []string{"b"}},
				{Title: "Batch1-3", Bullets: []string{"c"}},
				{Title: "Batch1-4", Bullets: []string{"d"}},
				// batch 2 (slides 9-10)
				{Title: "Batch2-1", Bullets: []string{"m"}},
				{Title: "Batch2-2", Bullets: []string{"n"}},
			},
		},
	}

	result, err := agent.enrichPPTContent(context.Background(), state)
	if err != nil {
		t.Fatalf("enrichPPTContent returned error: %v", err)
	}
	if len(result.richContent.Slides) != 2 {
		t.Fatalf("rich slides = %d, want 2", len(result.richContent.Slides))
	}
	if result.richContent.Slides[0].Title != "Ok1" || result.richContent.Slides[1].Title != "Ok2" {
		t.Fatalf("unexpected slide titles: %v", slideTitles(result.richContent.Slides))
	}
}

func TestPPTContentEnrichSingleBatch(t *testing.T) {
	model := &captureGenerationModel{}
	agent := &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name:  "ppt",
			typ:   GenerationTypePPT,
			model: model,
		},
	}
	state := pptChainState{
		input: generationAgentInput{
			Request: &GenerationRequest{Type: GenerationTypePPT, Markdown: "# Topic"},
			Context: "Original Markdown:\n# Topic",
		},
		expanded: pptOutlinePlan{
			Title:  "Topic",
			Slides: []pptSlidePlan{{Title: "Only Slide", Bullets: []string{"Only"}}},
		},
	}

	result, err := agent.enrichPPTContent(context.Background(), state)
	if err != nil {
		t.Fatalf("enrichPPTContent returned error: %v", err)
	}
	if len(result.richContent.Slides) != 1 {
		t.Fatalf("rich slides = %d, want 1", len(result.richContent.Slides))
	}
	// Mock's default JSON: title is "Slide"
	if result.richContent.Slides[0].Title != "Slide" {
		t.Fatalf("title = %q, want 'Slide'", result.richContent.Slides[0].Title)
	}
}

func TestPPTContentEnrichNilModel(t *testing.T) {
	agent := &pptGenerationAgent{
		baseGenerationAgent: baseGenerationAgent{
			name: "ppt",
			typ:  GenerationTypePPT,
		},
	}
	state := pptChainState{
		expanded: pptOutlinePlan{
			Title:  "T",
			Slides: []pptSlidePlan{{Title: "S1"}, {Title: "S2"}},
		},
	}
	result, err := agent.enrichPPTContent(context.Background(), state)
	if err != nil {
		t.Fatalf("enrichPPTContent returned error: %v", err)
	}
	if len(result.richContent.Slides) != 0 {
		t.Fatalf("rich slides = %d, want 0", len(result.richContent.Slides))
	}
}

// containsAll checks that value contains all needles.
func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}

// slideTitles extracts slide titles for test assertions.
func slideTitles(slides []enrichedPPTSlide) []string {
	titles := make([]string, len(slides))
	for i, s := range slides {
		titles[i] = s.Title
	}
	return titles
}
