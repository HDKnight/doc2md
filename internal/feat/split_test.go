package feat

import (
	"strings"
	"testing"
)

func TestSplitEmbeddedParen_AnswerGlue(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "答：（1)长征精神：红军不怕远征难，万水千山只等闲。长征精神是中国工农红军在长征中锤炼出来的。"},
		{kind: "heading", lvl: 4, text: "(2)原因：", series: "paren", num: 2},
		{kind: "para", text: "生命线是形象比喻。"},
	}
	segs = splitEmbeddedParenMarkers(segs)
	alignPeerHeadings(segs)
	normalizeHeadingLevels(segs)

	md := renderSegs(segs)
	if !strings.Contains(md, "答：") {
		t.Fatalf("keep 答：\n%s", md)
	}
	if !strings.Contains(md, "# (1)长征精神：红军不怕远征难，万水千山只等闲。") &&
		!strings.Contains(md, "# （1)长征精神：红军不怕远征难，万水千山只等闲。") {
		t.Fatalf("(1) title should end at first period:\n%s", md)
	}
	if !strings.Contains(md, "长征精神是中国工农红军") {
		t.Fatalf("body after period should be separate:\n%s", md)
	}
	// (1)(2) 同级
	l1, l2 := 0, 0
	for _, ln := range strings.Split(md, "\n") {
		tln := strings.TrimSpace(ln)
		if strings.HasPrefix(tln, "#") && strings.Contains(tln, "1)长征") {
			l1 = strings.Count(tln[:strings.Index(tln, " ")], "#")
		}
		if strings.HasPrefix(tln, "#") && strings.Contains(tln, "2)原因") {
			l2 = strings.Count(tln[:strings.Index(tln, " ")], "#")
		}
	}
	if l1 == 0 || l1 != l2 {
		t.Fatalf("(1)(2) peer levels %d vs %d\n%s", l1, l2, md)
	}
}

func renderSegs(segs []seg) string {
	var b strings.Builder
	for _, s := range segs {
		if s.kind == "heading" {
			b.WriteString(strings.Repeat("#", s.lvl))
			b.WriteByte(' ')
		}
		b.WriteString(s.text)
		b.WriteString("\n\n")
	}
	return b.String()
}
