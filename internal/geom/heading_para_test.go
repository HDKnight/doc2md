package geom

import (
	"strings"
	"testing"
)

func TestNewParaBareDigitAfterParenTitle(t *testing.T) {
	prev := "（1）马克思主义的物质观的理论意义"
	cur := "1坚持了唯物主义一元论,同唯心主义一元论和二元论划清了界限。"
	if !newPara(prev, cur) {
		t.Fatalf("expected new para between paren title and bare digit item")
	}
	if !newPara(prev, "1 坚持了唯物主义一元论") {
		t.Fatalf("expected new para for spaced bare digit")
	}
	if !newPara("派生性。", "2坚持了能动的反映论和可知论") {
		t.Fatalf("expected new para between items 1 and 2")
	}
}

func TestJoinParaDoesNotGlueMatterViewItems(t *testing.T) {
	// Simulate BuildHeadingMarkdown para accumulation with the fixed newPara.
	parts := []string{
		"(1)马克思主义的物质观的理论意义",
		"1坚持了唯物主义一元论,同唯心主义一元论和二元论划清了界限。派生性。",
		"2坚持了能动的反映论和可知论,批判了不可知论。深化。",
		"3体现了唯物论和辩证法的统一,缺陷。",
		"4体现了唯物主义自然观与唯物主义历史观的统一,统一。",
	}
	var blocks []string
	var para []string
	flush := func() {
		if len(para) == 0 {
			return
		}
		text := strings.TrimSpace(joinPara(para))
		para = nil
		if text != "" {
			blocks = append(blocks, text)
		}
	}
	for _, text := range parts {
		if len(para) > 0 && newPara(para[len(para)-1], text) {
			flush()
		}
		para = append(para, text)
	}
	flush()
	if len(blocks) != 5 {
		t.Fatalf("want 5 blocks, got %d:\n%s", len(blocks), strings.Join(blocks, "\n---\n"))
	}
	if strings.Contains(blocks[0], "1坚持") || strings.Contains(blocks[0], "1 坚持") {
		t.Fatalf("title glued with item1: %q", blocks[0])
	}
	joined := strings.Join(blocks, "\n")
	if strings.Contains(joined, "意义1") || strings.Contains(joined, "派生性。2") {
		t.Fatalf("items still glued:\n%s", joined)
	}
}
