package textnorm

import (
	"strings"
	"testing"

	"doc2md/internal/markdown"
)

func TestToBlocksPlain_OutlineOnlyNoGlueSplit(t *testing.T) {
	lines := []string{
		"中国共产党思想政治教育史-知识点总结",
		"1、两个时期各五个阶段:中国共产党成立,思想政治教育开端",
		"2、中共思想政治教育史的研究对象:党的思想政治教育产生、发展及其规律。",
		"3、中共对农民思政教育的主要目的:宣传党的路线方针和政策",
		"名词解释:",
		"1.灌输论:一种思想政治教育的理念。",
		"2.古田会议决议:1929年决议。",
	}
	blocks := ToBlocksPlain(lines)
	md := FinishPlainMarkdown(markdown.Render(blocks))

	if strings.Contains(md, "\n政治教育") {
		t.Fatalf("must not OCR-split 思想|政治:\n%s", md)
	}
	empty := 0
	heads := 0
	for _, ln := range strings.Split(md, "\n") {
		trim := strings.TrimSpace(ln)
		if !strings.HasPrefix(trim, "#") {
			continue
		}
		heads++
		if strings.Trim(trim, "# ") == "" {
			empty++
		}
	}
	if empty != 0 {
		t.Fatalf("empty headings=%d\n%s", empty, md)
	}
	if heads < 3 {
		t.Fatalf("expected outline headings, got %d\n%s", heads, md)
	}
	if !strings.Contains(md, "党的思想政治教育产生") {
		t.Fatalf("missing full stem:\n%s", md)
	}
}

func TestToBlocksPlain_DoesNotNeedFormatMarkdownOrdinals(t *testing.T) {
	blocks := ToBlocksPlain([]string{
		"标题",
		"1、甲",
		"2、乙",
		"3、丙",
	})
	md := markdown.Render(blocks)
	// 直接 Render 即应带层级；再跑 FormatMarkdownOrdinals 曾会拆空 #
	if !strings.Contains(md, "### 1、") && !strings.Contains(md, "## 1、") {
		// level depends on stack; just require a hashed 1、
		ok := false
		for _, ln := range strings.Split(md, "\n") {
			if strings.Contains(ln, "1、") && strings.HasPrefix(strings.TrimSpace(ln), "#") {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("1、 should be heading:\n%s", md)
		}
	}
}
