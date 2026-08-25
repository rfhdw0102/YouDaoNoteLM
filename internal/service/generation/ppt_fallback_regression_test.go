package generation

import (
	"strings"
	"testing"
)

func TestEnsurePPTStyleBlockForPlanMergesThemeIntoExistingStyle(t *testing.T) {
	content := `<style>.custom-slide { color: #123456; }</style>
<section class="ppt-slide"><h2>主题保留</h2><p>内容</p></section>`

	plan := &pptOutlinePlan{Slides: []pptSlidePlan{{Title: "主题保留"}}}
	got := ensurePPTStyleBlockForPlan(content, plan, testPPTTheme())

	if !strings.Contains(got, "--accent: #0f766e") {
		t.Fatalf("theme tokens were not merged into existing style: %s", got)
	}
	if !strings.Contains(got, ".custom-slide { color: #123456; }") {
		t.Fatalf("existing custom CSS was discarded: %s", got)
	}
}

func TestPPTNeedsStructureRepairAcceptsCompactCanvasCSS(t *testing.T) {
	content := `<style>.ppt-slide{width:1920px;height:1080px;overflow:hidden}</style>
<section class="ppt-slide"><h1>一</h1><p>二</p></section>
<section class="ppt-slide"><h2>三</h2><p>四</p></section>
<section class="ppt-slide"><h2>五</h2><p>六</p></section>
<section class="ppt-slide"><h2>七</h2><p>八</p></section>`

	if pptNeedsStructureRepair(content) {
		t.Fatalf("compact but valid canvas CSS was treated as structural failure")
	}
}

func TestPPTVisiblePlaceholderCheckIgnoresNormalLearningTerms(t *testing.T) {
	content := `<section><h2>关键要点</h2><p>本页总结步骤一和核心论点。</p></section>`

	if pptContainsVisiblePlaceholderText(content) {
		t.Fatalf("normal learning language was treated as placeholder text")
	}
}

func TestPPTRepairKeepsValidCustomHTML(t *testing.T) {
	content := `<style>.ppt-slide{width:1920px;height:1080px;overflow:hidden}.custom{color:#123456}</style>
<section class="ppt-slide"><h1>封面</h1><p>自定义内容一</p></section>
<section class="ppt-slide"><h2>关键要点</h2><p>本页总结步骤一和核心论点。</p></section>
<section class="ppt-slide"><h2>内容</h2><p>自定义内容二</p></section>
<section class="ppt-slide"><h2>总结</h2><p>自定义内容三</p></section>`

	draft := generationDraft{
		input:         generationAgentInput{},
		content:       content,
		pptStyleTheme: testPPTTheme(),
		pptRepairPlan: nil,
		fallbackUsed:  false,
	}
	agent := &pptGenerationAgent{}
	draft, err := agent.polishPPTHTML(nil, draft)
	if err != nil {
		t.Fatalf("polishPPTHTML returned error: %v", err)
	}
	got, err := agent.repairPPTStructure(nil, draft)
	if err != nil {
		t.Fatalf("repairPPTStructure returned error: %v", err)
	}
	if got.fallbackUsed {
		t.Fatalf("valid custom HTML unexpectedly used fallback: %s", got.content)
	}
	if !strings.Contains(got.content, "自定义内容一") || !strings.Contains(got.content, "--accent: #0f766e") {
		t.Fatalf("custom content or theme tokens were lost: %s", got.content)
	}
}
