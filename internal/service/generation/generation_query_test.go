// generation_query_test.go 测试生成查询规划。
package generation

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractGenerationKeywordsSamplesWholeMarkdown(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 25; i++ {
		b.WriteString(fmt.Sprintf("front%d filler%d\n", i, i))
	}
	b.WriteString("## TailImportantConcept\n")

	keywords := extractGenerationKeywords("", b.String(), 10)

	if !containsString(keywords, "TailImportantConcept") {
		t.Fatalf("expected tail concept to be sampled, got %v", keywords)
	}
}
