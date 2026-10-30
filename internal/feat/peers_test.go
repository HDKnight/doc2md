package feat

import (
	"strings"
	"testing"
)

func TestSplitLongSeriesStems_ThenPeersMatch(t *testing.T) {
	// 同型 10、粘长文 vs 17、短题干：拆开后都应是同级标题
	long10 := "10、干部教育第一；是抗日战争时期干部教育的指导方针，也是全党全军必须长期坚持的重要原则和基本要求。"
	long11 := "11、“思想领先”原则：思想领先原则是思想政治工作的重要原则，要求在一切工作中把思想工作做在前面并且贯穿始终。"
	// 无题干标点的长粘连：不得按字数硬切成「…指」「明了…」
	noCut := "1、十一届三中全会开创了党内思想政治建设的新局面，指明了思想政治工作制度化、法制化的目标方向，成为新时期思想政治工作的开端。"
	segs := []seg{
		{kind: "para", text: long10, w: 800},
		{kind: "para", text: long11, w: 800},
		{kind: "para", text: noCut, w: 800},
		{kind: "heading", text: "17、新式整军运动：", series: "arab_dun", num: 17},
		{kind: "para", text: "（一）历史意义：……"},
		{kind: "heading", text: "11、“政治工作是红军的生命线”论断的提出有何重大意义？", series: "arab_dun", num: 11},
	}
	for i := range segs {
		segs[i].series, segs[i].num = seriesOf(segs[i].text)
	}
	segs = splitLongSeriesStems(segs)
	alignPeerHeadings(segs)
	normalizeHeadingLevels(segs)

	var arabHeads []seg
	for _, s := range segs {
		if s.kind == "heading" && s.series == "arab_dun" {
			arabHeads = append(arabHeads, s)
		}
	}
	if len(arabHeads) < 4 {
		t.Fatalf("want >=4 arab_dun headings after split+align, got %d segs=%v", len(arabHeads), dumpKinds(segs))
	}
	lvl := arabHeads[0].lvl
	for _, s := range arabHeads {
		if s.lvl != lvl {
			t.Fatalf("peer levels must match: %+v", arabHeads)
		}
	}
	found10 := false
	for _, s := range arabHeads {
		if strings.HasPrefix(s.text, "10、") {
			found10 = true
		}
	}
	if !found10 {
		t.Fatalf("10、 should be heading: %v", arabHeads)
	}
	for _, s := range segs {
		if strings.Contains(s.text, "指明了") && strings.HasPrefix(strings.TrimSpace(s.text), "明了") {
			t.Fatalf("must not hard-split mid-word 指|明: %q", s.text)
		}
		if strings.HasSuffix(strings.TrimSpace(s.text), "局面，指") {
			t.Fatalf("must not leave orphan …指 heading: %q", s.text)
		}
	}
}

func TestSplitSeriesStemBody_NoBlindCut(t *testing.T) {
	s := "1、十一届三中全会开创了党内思想政治建设的新局面，指明了思想政治工作制度化、法制化的目标方向，成为新时期思想政治工作的开端。"
	_, _, ok := splitSeriesStemBody(s)
	if ok {
		t.Fatal("long stem without ：/？/； cut point must not split")
	}
}

func TestSplitSeriesStemBody_KeepsPeriodInsideStem(t *testing.T) {
	// 原文一整句：句号后仍是题干「它是指（A）」，不得拆成标题+正文
	s := "54、毛泽东在《论人民民主专政》一文中，提出了人民内部思想政治教育的原则方法。它是指（A )。"
	head, body, ok := splitSeriesStemBody(s)
	if ok {
		t.Fatalf("must not split at 。 inside MCQ stem: head=%q body=%q", head, body)
	}
}

func dumpKinds(segs []seg) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		t := s.text
		if r := []rune(t); len(r) > 20 {
			t = string(r[:20]) + "…"
		}
		parts = append(parts, s.kind+"/"+t)
	}
	return strings.Join(parts, " ; ")
}

func TestAlignPeerHeadings_PromoteMissing(t *testing.T) {
	// 1、 已是标题；2、 漏判为正文；3、 又是标题 → 2 应抬为同级
	segs := []seg{
		{kind: "heading", lvl: 2, text: "1、第一题？", series: "arab_dun", num: 1},
		{kind: "para", text: "答：内容一。", em: 12},
		{kind: "para", text: "2、第二题？", series: "arab_dun", num: 2},
		{kind: "para", text: "答：内容二。", em: 12},
		{kind: "heading", lvl: 3, text: "3、第三题", series: "arab_dun", num: 3},
		{kind: "para", text: strings.Repeat("长正文", 20), em: 12},
	}
	// refresh series from text for middle
	segs[2].series, segs[2].num = seriesOf(segs[2].text)
	alignPeerHeadings(segs)
	normalizeHeadingLevels(segs)

	if segs[2].kind != "heading" {
		t.Fatalf("2、 should be promoted: %+v", segs[2])
	}
	if segs[0].lvl != segs[2].lvl || segs[2].lvl != segs[4].lvl {
		t.Fatalf("peer levels should match: %d %d %d", segs[0].lvl, segs[2].lvl, segs[4].lvl)
	}
}

func TestNormalizeHeadingLevels_NoSkip(t *testing.T) {
	segs := []seg{
		{kind: "heading", lvl: 2, text: "1、大题", series: "arab_dun", num: 1},
		{kind: "heading", lvl: 5, text: "1.小节", series: "arab_dot", num: 1}, // 跳级
		{kind: "para", text: "正文"},
	}
	normalizeHeadingLevels(segs)
	if segs[1].lvl != 3 {
		t.Fatalf("child should be parent+1, got %d", segs[1].lvl)
	}
}

func TestNormalizeHeadingLevels_QuizSectionOverQuestions(t *testing.T) {
	// 复习题：##书名 → ###一、单项选择题 → ####1、2、3、题干（不得 ## 压过 ###）
	segs := []seg{
		{kind: "heading", lvl: 2, text: "《中国共产党思想政治教育史》复习题", series: "", num: 0},
		{kind: "heading", lvl: 3, text: "一、单项选择题", series: "cn_dun", num: 1},
		{kind: "heading", lvl: 2, text: "1、1945 年毛泽东在中共七大预备会议上称（A）为“五四运动时期的总司令”。", series: "arab_dun", num: 1},
		{kind: "para", text: "A、陈独秀"},
		{kind: "heading", lvl: 2, text: "2、（A）是比较系统地、正确地传播马克思主义…", series: "arab_dun", num: 2},
		{kind: "para", text: "A、李大钊"},
		{kind: "heading", lvl: 3, text: "二、简答题", series: "cn_dun", num: 2},
		{kind: "heading", lvl: 2, text: "1、简述三次论战？", series: "arab_dun", num: 1},
	}
	normalizeHeadingLevels(segs)
	lvl := func(sub string) int {
		for _, s := range segs {
			if s.kind == "heading" && strings.Contains(s.text, sub) {
				return s.lvl
			}
		}
		return 0
	}
	sec := lvl("一、单项选择题")
	q1 := lvl("1、1945")
	q2 := lvl("2、（A）是比较")
	if sec < 2 {
		t.Fatalf("section lvl=%d", sec)
	}
	if q1 != sec+1 || q2 != sec+1 {
		t.Fatalf("questions must nest under section: sec=%d q1=%d q2=%d segs=%v", sec, q1, q2, dumpLvls(segs))
	}
	sec2 := lvl("二、简答题")
	q3 := lvl("1、简述三次")
	if q3 != sec2+1 {
		t.Fatalf("brief Q under 二、: sec2=%d q3=%d", sec2, q3)
	}
	if lvl("《中国共产党") >= sec {
		// doc title should stay coarser or equal-ok; at least not deeper than section wrongly
	}
	if q1 <= sec {
		t.Fatalf("question must be deeper than 一、: sec=%d q1=%d", sec, q1)
	}
}

func dumpLvls(segs []seg) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		if s.kind != "heading" {
			continue
		}
		t := s.text
		if r := []rune(t); len(r) > 16 {
			t = string(r[:16]) + "…"
		}
		parts = append(parts, strings.Repeat("#", s.lvl)+" "+t)
	}
	return strings.Join(parts, " | ")
}

func TestNormalizeHeadingLevels_MajorQuestionNotUnderDot(t *testing.T) {
	// #### 1.小节 之后的 4、大题 不得被挂成 #####
	segs := []seg{
		{kind: "heading", lvl: 2, text: "3、试论述", series: "arab_dun", num: 3},
		{kind: "heading", lvl: 3, text: "三、基本经验：", series: "cn_dun", num: 3},
		{kind: "heading", lvl: 4, text: "4.运用唯物辩证的原则方法", series: "arab_dot", num: 4},
		{kind: "para", text: "长正文"},
		{kind: "heading", lvl: 2, text: "4、怎样理解长征精神？", series: "arab_dun", num: 4},
		{kind: "heading", lvl: 4, text: "(1)长征精神：短句。", series: "paren", num: 1},
		{kind: "heading", lvl: 4, text: "(2)原因：", series: "paren", num: 2},
		{kind: "heading", lvl: 2, text: "5、解放战争…", series: "arab_dun", num: 5},
		{kind: "heading", lvl: 3, text: "“三查三整”的整党运动", series: "", num: 0},
		{kind: "heading", lvl: 3, text: "(1)整党运动的背景原因", series: "paren", num: 1},
		{kind: "heading", lvl: 3, text: "(2)方法步骤", series: "paren", num: 2},
	}
	normalizeHeadingLevels(segs)
	lvl := func(sub string) int {
		for _, s := range segs {
			if s.kind == "heading" && strings.Contains(s.text, sub) {
				return s.lvl
			}
		}
		return 0
	}
	if lvl("3、试论述") != 2 || lvl("4、怎样") != 2 || lvl("5、解放") != 2 {
		t.Fatalf("major questions want h2, got %d %d %d", lvl("3、试论述"), lvl("4、怎样"), lvl("5、解放"))
	}
	if lvl("(1)整党运动") != lvl("三查三整”的整党")+1 {
		t.Fatalf("(1) should nest under 三查三整: parent=%d child=%d", lvl("三查三整”的整党"), lvl("(1)整党运动"))
	}
}

func TestNormalizeHeadingLevels_NestParenUnderSection(t *testing.T) {
	// ## 5、 → ### 三查三整 → (1)(2) 必须 ####，不能与三查三整同级 ###
	segs := []seg{
		{kind: "heading", lvl: 2, text: "5、解放战争…", series: "arab_dun", num: 5},
		{kind: "para", text: "解放战争时期党内教育的核心内容…"},
		{kind: "heading", lvl: 3, text: "“三查三整”的整党运动", series: "", num: 0},
		{kind: "para", text: "1947年10月至1949年春…"},
		{kind: "heading", lvl: 3, text: "(1)整党运动的背景原因", series: "paren", num: 1},
		{kind: "para", text: "第一，保证土地改革…"},
		{kind: "heading", lvl: 3, text: "(2)“三查三整”的方法步骤", series: "paren", num: 2},
		{kind: "para", text: "这次整党是结合土地改革…"},
		{kind: "heading", lvl: 3, text: "政策、纪律和理论教育", series: "", num: 0},
		{kind: "para", text: "在解放战争特别是…"},
		{kind: "heading", lvl: 4, text: "(1)增强政策和策略观念。", series: "paren", num: 1},
		{kind: "para", text: "解放战争期间…"},
		{kind: "heading", lvl: 4, text: "(2)克服无纪律和无政府状态。", series: "paren", num: 2},
	}
	normalizeHeadingLevels(segs)

	lvl := func(substr string) int {
		for _, s := range segs {
			if s.kind == "heading" && strings.Contains(s.text, substr) {
				return s.lvl
			}
		}
		return 0
	}
	if lvl("5、") != 2 {
		t.Fatalf("Q5 want h2, got %d", lvl("5、"))
	}
	if lvl("三查三整”的整党") != 3 {
		t.Fatalf("section 三查三整 want h3, got %d", lvl("三查三整”的整党"))
	}
	if a, b := lvl("(1)整党运动"), lvl("(2)“三查三整”的方法"); a != 4 || b != 4 {
		t.Fatalf("(1)(2) under 三查三整 want h4, got %d %d", a, b)
	}
	if lvl("政策、纪律") != 3 {
		t.Fatalf("政策 section want h3 sibling, got %d", lvl("政策、纪律"))
	}
	if a, b := lvl("(1)增强"), lvl("(2)克服"); a != 4 || b != 4 {
		t.Fatalf("(1)(2) under 政策 want h4, got %d %d", a, b)
	}
}

func TestTypeHintPlusPeer(t *testing.T) {
	pages := []Page{
		{Blocks: []Block{
			{Type: "doc_title", Content: "1、简述三次论战？", BBox: []float64{100, 40, 900, 100}},
			{Type: "text", Content: "答：五四运动前后改造社会。", BBox: []float64{100, 120, 900, 200}},
		}},
		{Blocks: []Block{
			{Type: "text", Content: "2、黄埔军校思想政治教育起点，简述经验？", BBox: []float64{100, 40, 900, 100}},
			{Type: "text", Content: "答：第一，建立政治工作。", BBox: []float64{100, 120, 900, 180}},
			{Type: "paragraph_title", Content: "3、试论述延安整风", BBox: []float64{100, 220, 700, 260}},
			{Type: "text", Content: strings.Repeat("整风运动论述要足够长作为正文块。", 10), BBox: []float64{100, 280, 900, 700}},
		}},
	}
	md := MarkdownFromPages(pages)
	// 三题同级
	for _, q := range []string{"1、简述", "2、黄埔", "3、试论述"} {
		if !strings.Contains(md, "# "+q) && !strings.Contains(md, "## "+q) && !strings.Contains(md, "### "+q) {
			t.Fatalf("missing heading for %s:\n%s", q, md)
		}
	}
	// 提取三题级别应相同
	lvl := func(prefix string) int {
		for _, ln := range strings.Split(md, "\n") {
			t := strings.TrimSpace(ln)
			if strings.Contains(t, prefix) && strings.HasPrefix(t, "#") {
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
	a, b, c := lvl("1、简述"), lvl("2、黄埔"), lvl("3、试论述")
	if a == 0 || a != b || b != c {
		t.Fatalf("peer levels want equal, got %d %d %d\n%s", a, b, c, md)
	}
}
