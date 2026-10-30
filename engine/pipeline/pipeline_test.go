package pipeline

import (
	"strings"
	"testing"
)

func TestKeepsPeriodInsideStem(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "54、毛泽东在《论人民民主专政》一文中，提出了人民内部思想政治教育的原则方法。它是指（A ）。", BBox: []float64{40, 100, 500, 120}},
			{Type: "text", Content: "A、民主的方法", BBox: []float64{40, 130, 200, 150}},
			{Type: "text", Content: "B、专政的方法", BBox: []float64{40, 160, 200, 180}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "原则方法。它是指") {
		t.Fatalf("must not split stem at 。:\n%s", md)
	}
	if strings.Contains(md, "\n它是指") || strings.Contains(md, "\n\n它是指") {
		t.Fatalf("stem continuation became separate block:\n%s", md)
	}
	if !strings.Contains(md, "#") || !strings.Contains(md, "54、") {
		t.Fatalf("ordinal stem should be heading:\n%s", md)
	}
}

func TestSoftWrapBrokenStem(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "18、党在（ A）的思想政治工作是中国军事史上军队政治工作的开始，是党", BBox: []float64{50, 100, 470, 114}},
			{Type: "text", Content: "在军队中从事思想政治工作的开端。", BBox: []float64{50, 116, 330, 130}},
			{Type: "text", Content: "A、黄埔军校", BBox: []float64{50, 140, 150, 154}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "是党在军队中从事") {
		t.Fatalf("broken stem not rejoined:\n%s", md)
	}
}

func TestLongAndShortOrdinalsBothHeadings(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "30、短题（D ）。", BBox: []float64{40, 10, 200, 24}},
			{Type: "text", Content: "A、洛川", BBox: []float64{40, 30, 100, 44}},
			{Type: "text", Content: "31、1937年8月，中国共产党在（B ）召开会议，提出了党在抗日战争统一战线中的领导权问题和加强党对抗日战争的政治领导的主张。", BBox: []float64{40, 50, 520, 64}},
			{Type: "text", Content: "A、瓦窑堡", BBox: []float64{40, 70, 100, 84}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "30、") || !strings.Contains(md, "31、") {
		t.Fatalf("both ordinals should appear:\n%s", md)
	}
	// both should be headings (lines starting with #)
	for _, stem := range []string{"30、", "31、"} {
		ok := false
		for _, line := range strings.Split(md, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, stem) {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("%s should be heading:\n%s", stem, md)
		}
	}
}

func TestOptionNotHeading(t *testing.T) {
	md := MarkdownFromLines([]string{
		"一、单项选择题",
		"1、实践是认识的基础",
		"A、正确",
		"B、错误",
	})
	for _, line := range strings.Split(md, "\n") {
		tline := strings.TrimSpace(line)
		if strings.HasPrefix(tline, "#") && strings.HasPrefix(strings.TrimLeft(tline, "# "), "A、") {
			t.Fatalf("option must not be heading: %q", tline)
		}
	}
}

func TestSplitGluedOptionsOnly(t *testing.T) {
	md := MarkdownFromLines([]string{
		"3、开端事件是（D)。A、新文化运动B、五四运动",
	})
	if !strings.Contains(md, "A、新文化运动") || !strings.Contains(md, "B、五四运动") {
		t.Fatalf("glued options not split:\n%s", md)
	}
	if !strings.Contains(md, "3、开端事件") {
		t.Fatalf("stem lost:\n%s", md)
	}
}

func TestSoftWrapStemContinuationRelaxedGap(t *testing.T) {
	// 间距略大：soft-wrap 或定题后的 headingContinues 都应拼上
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "18、党在（ A）的思想政治工作是中国军事史上军队政治工作的开始，是党", BBox: []float64{50, 100, 470, 114}},
			{Type: "text", Content: "在军队中从事思想政治工作的开端。", BBox: []float64{50, 130, 330, 144}},
			{Type: "text", Content: "A、黄埔军校", BBox: []float64{50, 160, 150, 174}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "是党在军队中从事") {
		t.Fatalf("heading fragment join failed:\n%s", md)
	}
}

func TestJoinHeadingFragment_LargeGapStillJoins(t *testing.T) {
	// 几何几乎不挨着：仍靠定题后文本续拼（旧 headingLineContinues）
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "18、党在（ A）的思想政治工作是中国军事史上军队政治工作的开始，是党", BBox: []float64{50, 100, 470, 114}},
			{Type: "text", Content: "在军队中从事思想政治工作的开端。", BBox: []float64{50, 200, 330, 214}},
			{Type: "text", Content: "A、黄埔军校", BBox: []float64{50, 230, 150, 244}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "是党在军队中从事") {
		t.Fatalf("text-level heading continue failed:\n%s", md)
	}
}

func TestParenShortTitlePeriodSplit(t *testing.T) {
	md := MarkdownFromLines([]string{
		"(1) 增强政策和策略观念。解放战争期间，党中央制定了新民主主义革命的总路线。",
		"(2) 克服无纪律和无政府状态。为了克服无纪律和无政府状态，党中央着重加强党的建设。",
		"54、毛泽东提出了原则方法。它是指（A ）。",
	})
	for _, line := range strings.Split(md, "\n") {
		tline := strings.TrimSpace(line)
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "解放战争期间") {
			t.Fatalf("body stuck in (1) heading: %q", tline)
		}
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "为了克服") {
			t.Fatalf("body stuck in (2) heading: %q", tline)
		}
	}
	if !strings.Contains(md, "原则方法。它是指") {
		t.Fatalf("MCQ stem must not split at 。:\n%s", md)
	}
}

func TestUnheraldedBreakJoinsUnderOrdinal(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "(2) 克服无纪律和无政府状态。", BBox: []float64{40, 40, 300, 56}},
			{Type: "text", Content: "为了克服无纪律和无政府状态，党中央着重从思想上、组织上、制度上加强党的建设。首先，进行政治理论教育，解决思", BBox: []float64{40, 70, 500, 100}},
			{Type: "text", Content: "想认识问题。其次，建立请示报告制度，保证党的集中统一。", BBox: []float64{40, 200, 500, 230}},
			{Type: "text", Content: "第一，从思想理论上武装全党。", BBox: []float64{40, 260, 400, 276}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "解决思想认识问题") {
		t.Fatalf("unheralded mid-break must join 思|想:\n%s", md)
	}
	idxSi := strings.Index(md, "解决思想认识问题")
	idxDi := strings.Index(md, "第一，从思想理论")
	if idxSi < 0 || idxDi < 0 || idxDi < idxSi {
		t.Fatalf("order broken:\n%s", md)
	}
	if strings.Contains(md, "集中统一。第一，") {
		t.Fatalf("第一， should stay separate block:\n%s", md)
	}
}

func TestHardStopKeepsNextParagraph(t *testing.T) {
	md := MarkdownFromLines([]string{
		"八、明确中国特色社会主义最本质的特征是中国共产党领导，确保党领导一切。",
		"我们要深刻领会新时代中国特色社会主义思想的精神实质和丰富的理论内涵，在各项工作中全面准确贯彻落实。",
	})
	if strings.Contains(md, "一切。我们要深刻领会") {
		t.Fatalf("paragraph after 。 must not join:\n%s", md)
	}
	if !strings.Contains(md, "确保党领导一切。") || !strings.Contains(md, "我们要深刻领会") {
		t.Fatalf("lost text:\n%s", md)
	}
}

func TestExpandedProseBlockNotHeading(t *testing.T) {
	md := MarkdownFromLines([]string{
		"一、明确坚持和发展中国特色社会主义，总任务是实现社会主义现代化。在全面建成小康社会的基础上，分两步走。",
		"二、明确新时代我国社会主要矛盾是人民日益增长的美好生活需要和不平衡不充分的发展之间的矛盾。必须坚持以人民为中心。",
		"一、单项选择题",
		"54、毛泽东提出了原则方法。它是指（A ）。",
	})
	for _, line := range strings.Split(md, "\n") {
		tline := strings.TrimSpace(line)
		if !strings.HasPrefix(tline, "#") {
			continue
		}
		if strings.Contains(tline, "明确坚持") || strings.Contains(tline, "明确新时代") {
			t.Fatalf("expanded prose must not be heading: %q", tline)
		}
	}
	hasChoice, has54 := false, false
	for _, line := range strings.Split(md, "\n") {
		tline := strings.TrimSpace(line)
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "单项选择题") {
			hasChoice = true
		}
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "54、") {
			has54 = true
		}
	}
	if !hasChoice || !has54 {
		t.Fatalf("short 一、 and MCQ stem should stay headings:\n%s", md)
	}

	// 短 OCR title 仍置信；多句 OCR title / 跨页拼成的多句序号标题要降
	pages := []Page{{
		Blocks: []Block{
			{Type: "paragraph_title", Content: "3.新时代中国特色社会主义思想。", BBox: []float64{40, 20, 300, 36}},
			{Type: "text", Content: "一、明确坚持和发展中国特色社会主义，总任务是实现社会主义现代化和中", BBox: []float64{40, 50, 500, 66}, Order: 1},
			{Type: "text", Content: "华民族伟大复兴。为了实现这一梦想，我们党提出两个一百年奋斗目标。", BBox: []float64{40, 80, 500, 110}, Order: 2},
			{Type: "paragraph_title", Content: "“三查三整”的整党运动", BBox: []float64{40, 140, 300, 156}},
			{Type: "paragraph_title", Content: "明确全面深化改革总目标。改革是决定当代中国命运的关键一招。", BBox: []float64{40, 180, 500, 210}},
		},
	}}
	md2 := MarkdownFromPages(pages)
	shortOCR, shortSan, joinedYi, multiOCR := false, false, false, false
	for _, line := range strings.Split(md2, "\n") {
		tline := strings.TrimSpace(line)
		if !strings.HasPrefix(tline, "#") {
			continue
		}
		if strings.Contains(tline, "3.新时代") {
			shortOCR = true
		}
		if strings.Contains(tline, "三查三整") {
			shortSan = true
		}
		if strings.Contains(tline, "明确坚持") {
			joinedYi = true
		}
		if strings.Contains(tline, "全面深化改革总目标") {
			multiOCR = true
		}
	}
	if !shortOCR || !shortSan {
		t.Fatalf("short OCR titles must stay:\n%s", md2)
	}
	if joinedYi {
		t.Fatalf("cross-join 一、 essay must not stay heading:\n%s", md2)
	}
	if multiOCR {
		t.Fatalf("multi-sentence OCR title must demote:\n%s", md2)
	}
}

func TestUnnumberedTitlesSameLevel_ParenNests(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "5、解放战争时期开展了哪些教育？", BBox: []float64{40, 40, 400, 56}},
			{Type: "paragraph_title", Content: "“三查三整”的整党运动", BBox: []float64{40, 80, 300, 96}},
			{Type: "text", Content: "1947年10月至1949年春，结合土地改革运动的开展。", BBox: []float64{40, 110, 480, 140}},
			{Type: "text", Content: "(1)整党运动的背景原因", BBox: []float64{40, 160, 280, 176}},
			{Type: "text", Content: "第一，保证土地改革运动的顺利进行。", BBox: []float64{40, 190, 400, 206}},
			{Type: "paragraph_title", Content: "(2)“三查三整”的方法步骤", BBox: []float64{40, 230, 320, 246}},
			{Type: "text", Content: "这次整党是结合土地改革运动进行的。", BBox: []float64{40, 260, 400, 276}},
			{Type: "paragraph_title", Content: "政策、纪律和理论教育", BBox: []float64{40, 300, 280, 316}},
			{Type: "text", Content: "在解放战争特别是在人民解放军转入进攻阶段以后。", BBox: []float64{40, 330, 480, 360}},
		},
	}}
	md := MarkdownFromPages(pages)
	lvl := func(substr string) int {
		for _, line := range strings.Split(md, "\n") {
			t := strings.TrimSpace(line)
			if !strings.HasPrefix(t, "#") || !strings.Contains(t, substr) {
				continue
			}
			n := 0
			for _, r := range t {
				if r == '#' {
					n++
				} else {
					break
				}
			}
			return n
		}
		return -1
	}
	a, b := lvl("三查三整”的整党运动"), lvl("政策、纪律和理论教育")
	if a < 0 || b < 0 || a != b {
		t.Fatalf("unnumbered titles should share level: 三查=%d 政策=%d\n%s", a, b, md)
	}
	p1, p2 := lvl("(1)整党运动的背景原因"), lvl("(2)“三查三整”的方法步骤")
	if p1 != a+1 || p2 != a+1 {
		t.Fatalf("(1)(2) should nest under 三查: 三查=%d (1)=%d (2)=%d\n%s", a, p1, p2, md)
	}
}

func TestOCRTitleNotMergedWithBody(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			{Type: "paragraph_title", Content: "“三查三整”的整党运动", BBox: []float64{100, 200, 400, 220}},
			{Type: "text", Content: "1947 年10月至1949年春，结合土地改革运动的开展，中国共产党内开展了以查阶级、查思想、查作风和整顿组织、整顿思想、整顿作风(简称“三查三整”)为内容的整党运动，其主要目的是解决党内不纯和官僚主义问题，实质上是一场马克思列宁主义的阶级教育运动。", BBox: []float64{100, 230, 500, 320}},
		},
	}}
	md := MarkdownFromPages(pages)
	for _, line := range strings.Split(md, "\n") {
		tline := strings.TrimSpace(line)
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "1947") {
			t.Fatalf("OCR title swallowed body:\n%s", md)
		}
	}
	if !strings.Contains(md, "三查三整”的整党运动") || !strings.Contains(md, "1947") {
		t.Fatalf("title/body lost:\n%s", md)
	}
}

func TestDottedDateNotOrdinal(t *testing.T) {
	md := MarkdownFromLines([]string{
		"5、思想政治教育工作是生命线是何时提出的:1932.7.21",
		"6、抗战时期党的思想政治教育围绕的主题",
		"12、1949.3召开七届二中全会，加强党的思想建设和作风建设。",
		"1.思政是什么:思想政治教育是社会实践活动。",
		"2.灌输论:灌输论实际上是一种理念。",
	})
	for _, line := range strings.Split(md, "\n") {
		tline := strings.TrimSpace(line)
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "1932") {
			t.Fatalf("date must not be heading: %q\n%s", tline, md)
		}
	}
	if !strings.Contains(md, "1932.7.21") {
		t.Fatalf("date lost:\n%s", md)
	}
	if !strings.Contains(md, "1949.3召开") {
		t.Fatalf("in-stem date lost:\n%s", md)
	}
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, "1949.3") && !strings.Contains(line, "12、") {
			t.Fatalf("date inside stem split into its own heading: %q\n%s", line, md)
		}
	}
	has5, has1 := false, false
	for _, line := range strings.Split(md, "\n") {
		tline := strings.TrimSpace(line)
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "5、") {
			has5 = true
		}
		if strings.HasPrefix(tline, "#") && strings.Contains(tline, "1.思政") {
			has1 = true
		}
	}
	if !has5 || !has1 {
		t.Fatalf("real ordinals should stay headings:\n%s", md)
	}
}

func TestShortLabelColonSplitsBody(t *testing.T) {
	md := MarkdownFromLines([]string{
		"6、三湾改编：1927年秋收起义中，毛泽东决定对部队进行整顿。",
	})
	if !strings.Contains(md, "三湾改编：") {
		t.Fatalf("label heading missing:\n%s", md)
	}
	// 冒号后应落在正文，不整段塞进同一标题行
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, "1927年秋收") {
			t.Fatalf("body stuck in heading: %q", line)
		}
	}
	if !strings.Contains(md, "1927年秋收") {
		t.Fatalf("body lost:\n%s", md)
	}
}
