package feat

import (
	"strings"
	"testing"
)

func TestOrdinalStemsAreHeadings_AfterSoftWrap(t *testing.T) {
	// 特殊序号 → 标题；长短一致；先断行再定题
	items := []item{
		{text: "30、1935年12月，中国共产党在（D ）召开政治局扩大会议，制定了抗日民族统一战线的新政策。", em: 12, w: 400, boxH: 14, y0: 10, y1: 24, x0: 40, typ: "text"},
		{text: "A、洛川", em: 12, w: 80, boxH: 14, y0: 30, y1: 44, x0: 40, typ: "text"},
		{text: "B、直罗镇", em: 12, w: 80, boxH: 14, y0: 50, y1: 64, x0: 40, typ: "text"},
		{text: "31、1937年8月，中国共产党在（B ）召开会议，提出了党在抗日战争统一战线中的领导权问题和加强党对抗日战争的政治领导的主张。", em: 12, w: 500, boxH: 14, y0: 70, y1: 84, x0: 40, typ: "text"},
		{text: "A、瓦窑堡", em: 12, w: 80, boxH: 14, y0: 90, y1: 104, x0: 40, typ: "text"},
		{text: "11、简述马克思主义传入中国初期与反马克思主义进行的三次论战？", em: 12, w: 400, boxH: 14, y0: 110, y1: 124, x0: 40, typ: "text"},
		{text: "答：第一次是问题与主义之争。", em: 12, w: 400, boxH: 40, y0: 130, y1: 170, x0: 40, typ: "text"},
	}
	segs := buildPageSegs(items)
	var heads []string
	for _, s := range segs {
		if s.kind == "heading" {
			heads = append(heads, s.text)
		}
	}
	joined := strings.Join(heads, "\n")
	for _, want := range []string{"30、", "31、", "11、"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("ordinal stem %q should be heading:\n%s", want, joined)
		}
	}
	for _, s := range segs {
		if s.kind == "heading" && strings.HasPrefix(s.text, "A、") {
			t.Fatalf("option must not be heading: %q", s.text)
		}
	}
}

func TestSoftWrapBeforeHeading_RejoinsBrokenStem(t *testing.T) {
	// 半截「…是党」+「在军队中…」须先拼再成标题
	items := []item{
		{text: "18、党在（ A）的思想政治工作是中国军事史上军队政治工作的开始，是党", em: 12, w: 420, boxH: 14, y0: 100, y1: 114, x0: 50, typ: "text"},
		{text: "在军队中从事思想政治工作的开端。", em: 12, w: 280, boxH: 14, y0: 116, y1: 130, x0: 50, typ: "text"},
		{text: "A、黄埔军校", em: 12, w: 100, boxH: 14, y0: 140, y1: 154, x0: 50, typ: "text"},
	}
	segs := buildPageSegs(items)
	if len(segs) < 2 {
		t.Fatalf("want stem+option, got %d: %+v", len(segs), segs)
	}
	if segs[0].kind != "heading" {
		t.Fatalf("joined stem should be heading: %+v", segs[0])
	}
	if !strings.Contains(segs[0].text, "是党在军队中从事") {
		t.Fatalf("broken stem not rejoined: %q", segs[0].text)
	}
	if segs[1].kind == "heading" || !strings.HasPrefix(segs[1].text, "A、") {
		t.Fatalf("option row broken: %+v", segs[1])
	}
}

func TestAlignPeerKeepsOrdinalStems(t *testing.T) {
	segs := []seg{
		{kind: "heading", text: "30、短题（D ）。", series: "arab_dun", num: 30},
		{kind: "para", text: "A、洛川"},
		{kind: "para", text: "31、很长很长的选择题干仍然后接选项。", series: "arab_dun", num: 31},
		{kind: "para", text: "A、瓦窑堡"},
	}
	for i := range segs {
		if segs[i].series == "" {
			segs[i].series, segs[i].num = seriesOf(segs[i].text)
		}
	}
	alignPeerHeadings(segs)
	for _, s := range segs {
		if strings.HasPrefix(s.text, "30、") || strings.HasPrefix(s.text, "31、") {
			if s.kind != "heading" {
				t.Fatalf("ordinal stem should stay/become heading: %+v", s)
			}
		}
	}
}
