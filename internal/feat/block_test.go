package feat

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMarkdownFromPages_TrustTypeAndFillGaps(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "1、简述三次论战？", Order: 1, BBox: []float64{100, 40, 900, 110}},
			{Type: "text", Content: "答：五四运动前后，", Order: 2, BBox: []float64{100, 140, 900, 170}},
			{Type: "text", Content: "改造社会的呼声高涨。", Order: 3, BBox: []float64{100, 175, 900, 205}},
			// type 标成标题 → 直接信任（无题号也可）
			{Type: "paragraph_title", Content: "政策、纪律和理论教育", Order: 4, BBox: []float64{120, 280, 420, 310}},
			{Type: "text", Content: "这是足够长的正文用来估计正文字号代理数值的句子继续加长一些。", Order: 5, BBox: []float64{100, 360, 900, 480}},
			{Type: "paragraph_title", Content: "“三查三整”的整党运动", Order: 6, BBox: []float64{120, 520, 450, 560}},
			{Type: "text", Content: strings.Repeat("整党运动背景论述要足够长。", 8), Order: 7, BBox: []float64{100, 580, 900, 900}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "# 1、简述三次论战？") && !strings.Contains(md, "## 1、简述三次论战？") {
		t.Fatalf("question should be heading:\n%s", md)
	}
	if !hasMDHeading(md, "政策、纪律和理论教育") {
		t.Fatalf("paragraph_title must be trusted:\n%s", md)
	}
	if !hasMDHeading(md, "“三查三整”的整党运动") {
		t.Fatalf("paragraph_title must be trusted:\n%s", md)
	}
	if !strings.Contains(md, "答：五四运动前后，改造社会的呼声高涨。") {
		t.Fatalf("soft-wrap should join:\n%s", md)
	}
}

func hasMDHeading(md, title string) bool {
	for _, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "#") && strings.Contains(t, title) {
			return true
		}
	}
	return false
}

func TestMarkdownFromPages_TitleWithTrailingPeriod(t *testing.T) {
	// oar 标 paragraph_title，但文末有「。」——不应被当成散文丢掉标题
	pages := []Page{{
		Blocks: []Block{
			{Type: "paragraph_title", Content: "3.新时代中国特色社会主义思想。", BBox: []float64{340, 630, 950, 680}},
			{Type: "text", Content: strings.Repeat("习近平新时代中国特色社会主义思想论述要足够长。", 10), BBox: []float64{260, 750, 1520, 1500}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !hasMDHeading(md, "3.新时代中国特色社会主义思想。") {
		t.Fatalf("paragraph_title ending with 。 must stay heading:\n%s", md)
	}
}

func TestMarkdownFromPages_SameSizeQuestionBeforeTall(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Content: "3、试论述延安整风运动的方针及基本经验", BBox: []float64{100, 80, 700, 130}},
			{Content: strings.Repeat("正文论述内容要足够长才能被当成大段正文块用于邻接判断。", 8), BBox: []float64{100, 160, 900, 700}},
			{Content: "三、基本经验：", BBox: []float64{120, 720, 320, 760}},
			{Content: strings.Repeat("经验展开也要足够长作为后续大段。", 6), BBox: []float64{100, 780, 900, 1100}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "# 3、试论述延安整风") {
		t.Fatalf("same-size question before tall body:\n%s", md)
	}
	if !strings.Contains(md, "# 三、基本经验") {
		t.Fatalf("narrow/short label before tall:\n%s", md)
	}
	if strings.Contains(md, "# 正文论述") {
		t.Fatalf("long body must not be heading:\n%s", md)
	}
}

func TestEstimateLineEm(t *testing.T) {
	em := estimateLineEm(70, 900, "1、简述三次论战？")
	if em < 60 {
		t.Fatalf("single-line large title em should ≈ boxH, got %v", em)
	}
	em2 := estimateLineEm(120, 800, strings.Repeat("字", 80))
	if em2 >= 110 || em2 < 15 {
		t.Fatalf("wrapped line-em, got %v", em2)
	}
}

func TestRealJiandaHeadingsUsable(t *testing.T) {
	root := findModRoot()
	files, _ := filepath.Glob(filepath.Join(root, "testdata/out/oar_structure_json/*_p*.json"))
	if len(files) < 8 {
		t.Skip("need generated structure json")
	}
	var pages []Page
	for i := 1; i <= 16; i++ {
		matches, _ := filepath.Glob(filepath.Join(root, "testdata/out/oar_structure_json", "*_p"+strconv.Itoa(i)+".json"))
		if len(matches) == 0 {
			continue
		}
		b, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		p, err := ParsePageJSON(b)
		if err != nil {
			t.Fatal(err)
		}
		pages = append(pages, p)
	}
	md := MarkdownFromPages(pages)
	checks := []string{
		"1、简述",
		"2、黄埔",
		"3、试论述",
		"三、基本经验",
		"4、怎样理解长征精神",
		"5、解放战争",
	}
	for _, c := range checks {
		ok := false
		for _, ln := range strings.Split(md, "\n") {
			t := strings.TrimSpace(ln)
			if strings.HasPrefix(t, "#") && strings.Contains(t, c) {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("missing heading containing %q:\n%s", c, md[:min(800, len(md))])
		}
	}
	// 1、2、3、 大题应同级
	lvlOf := func(key string) int {
		for _, ln := range strings.Split(md, "\n") {
			t := strings.TrimSpace(ln)
			if strings.HasPrefix(t, "#") && strings.Contains(t, key) {
				n := 0
				for _, c := range t {
					if c == '#' {
						n++
					} else {
						break
					}
				}
				return n
			}
		}
		return 0
	}
	if a, b := lvlOf("1、简述"), lvlOf("2、黄埔"); a == 0 || a != b {
		t.Fatalf("Q1/Q2 peer level mismatch: %d vs %d", a, b)
	}
	if a, c := lvlOf("1、简述"), lvlOf("3、试论述"); a == 0 || a != c {
		t.Fatalf("Q1/Q3 peer level mismatch: %d vs %d", a, c)
	}
	// 长「一、延安整风…」不得整段抬标题
	for _, ln := range strings.Split(md, "\n") {
		if strings.HasPrefix(ln, "#") && strings.Contains(ln, "一、延安整风运动是中国共产党第一次广泛") {
			t.Fatalf("long 一、 body must not be heading:\n%s", ln)
		}
	}
	heads := strings.Count(md, "\n#") + strings.Count(md, "\n##") 
	// rough: at least ~10 question/section heads across doc
	nHead := 0
	for _, ln := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "#") {
			nHead++
		}
	}
	if nHead < 12 {
		t.Fatalf("too few headings (%d), still unusable", nHead)
	}
	if strings.Contains(md, "# 想认识问题") {
		t.Fatalf("page-break prose fragment must not be heading")
	}
	if !hasMDHeading(md, "政策、纪律和理论教育") {
		t.Fatalf("oar paragraph_title 政策… should be heading")
	}
	if !hasMDHeading(md, "“三查三整”的整党运动") && !hasMDHeading(md, "三查三整”的整党运动") {
		t.Fatalf("oar paragraph_title 三查三整… should be heading")
	}
	if !hasMDHeading(md, "1)长征精神") {
		t.Fatalf("embedded (1) should be split out as heading peer of (2)")
	}
	if !hasMDHeading(md, "2)原因") {
		t.Fatalf("(2)原因 should remain heading")
	}
	if !strings.Contains(md, "解决思想认识问题") {
		t.Fatalf("broken page wrap 解决思|想认识 should merge")
	}
	_ = heads
	out := filepath.Join(root, "testdata/out/ocr_probe/jianda_feat_tuned.md")
	_ = os.WriteFile(out, []byte(md), 0o644)
}

func findModRoot() string {
	wd, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		wd = filepath.Dir(wd)
	}
	return "."
}
