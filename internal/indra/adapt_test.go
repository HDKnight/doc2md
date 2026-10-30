package indra

import (
	"strings"
	"testing"
)

func TestMapLabel(t *testing.T) {
	cases := []struct {
		in   string
		typ  string
		drop bool
	}{
		{"Section-Header", "paragraph_title", false},
		{"Table-Of-Contents", "paragraph_title", false},
		{"Text", "text", false},
		{"List-Group", "text", false},
		{"Table", "text", false}, // 假表格当正文
		{"Page-Header", "", true},
		{"Page-Footer", "", true},
		{"Weird", "text", false},
	}
	for _, c := range cases {
		typ, drop := MapLabel(c.in)
		if typ != c.typ || drop != c.drop {
			t.Fatalf("%q: got typ=%q drop=%v want typ=%q drop=%v", c.in, typ, drop, c.typ, c.drop)
		}
	}
}

func TestToFeatPages_PrefersLines(t *testing.T) {
	jsonl := []byte(`{"page":0,"blocks":[{"label":"Table","text":"坏表格","bbox":[0,0,1,1]}],"lines":[{"text":"好行1","box":[1,2,3,24],"conf":0.9},{"text":"2","box":[0,100,1,120],"conf":0.5},{"text":"好行2","box":[5,200,7,220],"conf":0.9}]}
`)
	pages, err := ToFeatPages(jsonl)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages=%d", len(pages))
	}
	// 页码 "2" 丢弃；不用 Table 块
	if len(pages[0].Blocks) != 2 {
		t.Fatalf("blocks=%d want 2, got %+v", len(pages[0].Blocks), pages[0].Blocks)
	}
	if pages[0].Blocks[0].Content != "好行1" || pages[0].Blocks[1].Content != "好行2" {
		t.Fatalf("%+v", pages[0].Blocks)
	}
}

func TestSortLinesRowMajor_TwoColumnOptions(t *testing.T) {
	// OrderNet 常见坏序：A,C,B,D（先左列再右列）
	in := []rawLine{
		{Text: "A、李大钊", Box: []float64{376, 960, 600, 1006}},
		{Text: "C、毛泽东", Box: []float64{377, 1045, 594, 1089}},
		{Text: "B、陈独秀", Box: []float64{704, 960, 925, 1006}},
		{Text: "D、蔡和森", Box: []float64{703, 1045, 919, 1089}},
	}
	got := sortLinesRowMajor(in)
	want := []string{"A、李大钊", "B、陈独秀", "C、毛泽东", "D、蔡和森"}
	for i, w := range want {
		if got[i].Text != w {
			t.Fatalf("i=%d got=%q want=%q full=%v", i, got[i].Text, w, texts(got))
		}
	}
}

func TestMergeRowBands_ParenSplit(t *testing.T) {
	// 同一视觉行被拆成「2、（ A」+「）是…」
	in := []rawLine{
		{Text: "2、（ A", Box: []float64{377, 792, 557, 840}},
		{Text: "）是比较系统地、正确地传播马克思主义", Box: []float64{601, 792, 2104, 840}},
		{Text: "A、李大钊", Box: []float64{376, 960, 600, 1006}},
		{Text: "B、陈独秀", Box: []float64{704, 960, 925, 1006}},
	}
	ordered := sortLinesRowMajor(in)
	got := mergeRowBands(ordered)
	if len(got) != 2 {
		t.Fatalf("bands=%d texts=%v", len(got), texts(got))
	}
	if !strings.Contains(got[0].Text, "2、（ A）是比较系统") && !strings.Contains(got[0].Text, "2、（ A ）是比较系统") && !strings.HasPrefix(got[0].Text, "2、（ A") {
		// 允许中间无空格粘合
		if got[0].Text != "2、（ A）是比较系统地、正确地传播马克思主义" &&
			got[0].Text != "2、（ A ）是比较系统地、正确地传播马克思主义" {
			// joinNoSpace: prev last rune of "A" is Latin → space before ）? next is ） so joinNoSpace true
			t.Fatalf("merged stem=%q", got[0].Text)
		}
	}
	if got[0].Text != "2、（ A）是比较系统地、正确地传播马克思主义" {
		t.Fatalf("stem=%q", got[0].Text)
	}
	if got[1].Text != "A、李大钊 B、陈独秀" && got[1].Text != "A、李大钊B、陈独秀" {
		t.Fatalf("opts=%q", got[1].Text)
	}
}

func texts(ls []rawLine) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Text
	}
	return out
}

func TestToFeatPages_BlocksFallback(t *testing.T) {
	jsonl := []byte(`{"page":0,"blocks":[{"label":"Page-Header","text":"hdr","bbox":[0,0,1,1]},{"label":"Section-Header","text":"标题","bbox":[1,2,3,4]},{"label":"Text","text":"正文","bbox":[5,6,7,8]}]}
`)
	pages, err := ToFeatPages(jsonl)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages[0].Blocks) != 2 {
		t.Fatalf("want 2 got %+v", pages[0].Blocks)
	}
	if pages[0].Blocks[0].Type != "paragraph_title" {
		t.Fatalf("%+v", pages[0].Blocks[0])
	}
}

func TestResolveDPI(t *testing.T) {
	t.Setenv("DOC2MD_INDRA_DPI", "")
	if ResolveDPI(0) != 200 {
		t.Fatalf("default=%d", ResolveDPI(0))
	}
	if ResolveDPI(150) != 200 {
		t.Fatal("convert sentinel 150 -> 200")
	}
	if ResolveDPI(144) != 144 {
		t.Fatal("explicit")
	}
}
