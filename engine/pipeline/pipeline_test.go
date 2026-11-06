package pipeline

import (
	"strings"
	"testing"
)

func TestWrapJoinFollowsColumnEdge(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			// 两行写到同一右缘，栏才成立。满行、紧挨着、未到句末 → 拼。
			{Type: "text", Content: "它公开申明自己是无产阶级的世", BBox: []float64{35, 100, 555, 113}},
			{Type: "text", Content: "界观和方法论，不带有任何的宗派性。", BBox: []float64{35, 115, 400, 128}},
			// 满行但已是句末 → 不拼。
			{Type: "text", Content: "反映的是马克思主义的精神实质。", BBox: []float64{35, 140, 555, 153}},
			{Type: "text", Content: "下一句另起。", BBox: []float64{35, 155, 200, 168}},
			// 没写到右缘 → 不拼，即使下一行就在下面。
			{Type: "text", Content: "D．用马克思主义基本理论指导中国革命和建设", BBox: []float64{35, 200, 392, 213}},
			{Type: "text", Content: "【答案】D", BBox: []float64{30, 215, 80, 228}},
			// 同一基线上左右相邻 → 拼回同一行。
			{Type: "text", Content: "表2", BBox: []float64{250, 300, 262, 313}},
			{Type: "text", Content: "马克思主义创立的条件", BBox: []float64{270, 300, 345, 313}},
			// 满行但垂直间距明显大于一行 → 不拼。
			{Type: "text", Content: "上一段写到栏的右缘还没结束", BBox: []float64{35, 400, 555, 413}},
			{Type: "text", Content: "下一段离得远。", BBox: []float64{35, 460, 200, 473}},
		},
	}, {
		// 没有两行共同到达的右缘，最长的一行也不当栏宽。
		Blocks: []Block{
			{Type: "text", Content: "导论", BBox: []float64{272, 55, 322, 77}},
			{Type: "text", Content: "0.1 复习笔记", BBox: []float64{253, 110, 342, 129}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "无产阶级的世界观和方法论") {
		t.Fatalf("full line wrap should join:\n%s", md)
	}
	if strings.Contains(md, "精神实质。下一句") {
		t.Fatalf("sentence end must not join:\n%s", md)
	}
	if strings.Contains(md, "建设【答案】") {
		t.Fatalf("short line must not join:\n%s", md)
	}
	if !strings.Contains(md, "表2马克思主义创立的条件") {
		t.Fatalf("same-baseline fragments should join:\n%s", md)
	}
	if strings.Contains(md, "还没结束下一段") {
		t.Fatalf("large gap must not join:\n%s", md)
	}
	if strings.Contains(md, "导论0.1") {
		t.Fatalf("title-only page must not join its longest line:\n%s", md)
	}
	cross := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "这行写满栏宽还没有句号所以要接下去", BBox: []float64{35, 100, 555, 113}},
			{Type: "text", Content: "另一行也写满作为右缘", BBox: []float64{35, 115, 555, 128}},
			{Type: "text", Content: "短的收尾", BBox: []float64{35, 130, 120, 143}},
		},
	}, {
		Blocks: []Block{
			{Type: "text", Content: "【例】下一页", BBox: []float64{35, 40, 200, 53}},
		},
	}}
	crossed := MarkdownFromPages(cross)
	if strings.Contains(crossed, "收尾【例】") {
		t.Fatalf("short tail must not join the next page:\n%s", crossed)
	}
	if !strings.Contains(crossed, "接下去另一行也写满作为右缘短的收尾") {
		t.Fatalf("in-page wrap should still join:\n%s", crossed)
	}
	plain := MarkdownFromLines([]string{"考点二：马克思主义的创立", "1 马克思主义创立的条件"})
	if strings.Contains(plain, "创立1") {
		t.Fatalf("lines without boxes must not join:\n%s", plain)
	}
}

func TestShortOptionDoesNotSwallowNextQuestion(t *testing.T) {
	// 选项框被拉到栏右缘时，也不能把下一题题干或右侧纯数字拼上来。
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "1. 1945年毛泽东在中共七大预备会议上称（A）为“五四运动时期的总司令”。", BBox: []float64{40, 40, 520, 54}},
			{Type: "text", Content: "A. 陈独秀", BBox: []float64{40, 60, 520, 74}},
			{Type: "text", Content: "D. 钱玄同", BBox: []float64{40, 108, 520, 122}},
			{Type: "text", Content: "2、（A）是比较系统地、正确地传播马克思主义的第一人。", BBox: []float64{40, 124, 520, 138}},
			{Type: "text", Content: "A. 李大钊", BBox: []float64{40, 140, 160, 154}},
			{Type: "text", Content: "1", BBox: []float64{170, 140, 182, 154}},
			{Type: "text", Content: "栏右缘参照行写满用来定栏。", BBox: []float64{40, 180, 520, 194}},
		},
	}}
	md := MarkdownFromPages(pages)
	if strings.Contains(md, "钱玄同2、") || strings.Contains(md, "钱玄同2") {
		t.Fatalf("option must not swallow the next stem:\n%s", md)
	}
	if strings.Contains(md, "李大钊1") {
		t.Fatalf("option must not swallow a stray number:\n%s", md)
	}
	if !strings.Contains(md, "2、（A）是比较系统") {
		t.Fatalf("next question lost:\n%s", md)
	}
	if !strings.Contains(md, "A. 李大钊") {
		t.Fatalf("option lost:\n%s", md)
	}
}

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
			{Type: "text", Content: "栏右缘参照行。", BBox: []float64{50, 180, 470, 194}},
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
	// 行距明显超过一行：即使写到栏右缘也不拼。
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "18、党在（ A）的思想政治工作是中国军事史上军队政治工作的开始，是党", BBox: []float64{50, 100, 470, 114}},
			{Type: "text", Content: "在军队中从事思想政治工作的开端。", BBox: []float64{50, 130, 330, 144}},
			{Type: "text", Content: "栏右缘参照行。", BBox: []float64{50, 180, 470, 194}},
		},
	}}
	md := MarkdownFromPages(pages)
	if strings.Contains(md, "是党在军队中从事") {
		t.Fatalf("gap past one line must not join:\n%s", md)
	}
}

func TestLargeGapDoesNotJoin(t *testing.T) {
	// 垂直方向隔开：不再按文本续拼。
	pages := []Page{{
		Blocks: []Block{
			{Type: "text", Content: "18、党在（ A）的思想政治工作是中国军事史上军队政治工作的开始，是党", BBox: []float64{50, 100, 470, 114}},
			{Type: "text", Content: "在军队中从事思想政治工作的开端。", BBox: []float64{50, 200, 330, 214}},
			{Type: "text", Content: "栏右缘参照行。", BBox: []float64{50, 240, 470, 254}},
		},
	}}
	md := MarkdownFromPages(pages)
	if strings.Contains(md, "是党在军队中从事") {
		t.Fatalf("large gap must not join:\n%s", md)
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
			{Type: "text", Content: "(2) 克服无纪律和无政府状态。", BBox: []float64{40, 40, 300, 54}},
			{Type: "text", Content: "为了克服无纪律和无政府状态，党中央着重从思想上、组织上、制度上加强党的建设。首先，进行政治理论教育，解决思", BBox: []float64{40, 70, 500, 84}},
			{Type: "text", Content: "想认识问题。其次，建立请示报告制度，保证党的集中统一。", BBox: []float64{40, 86, 420, 100}},
			{Type: "text", Content: "第一，从思想理论上武装全党。", BBox: []float64{40, 116, 400, 130}},
			{Type: "text", Content: "栏右缘参照。", BBox: []float64{40, 160, 500, 174}},
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

func TestQuotedSentenceEndDoesNotSwallowTitle(t *testing.T) {
	pages := []Page{{
		Blocks: []Block{
			// 引号前不是句号：仍是折行，要拼。
			{Type: "text", Content: "最重要的是在党内展开批评和自我批评”", BBox: []float64{40, 40, 520, 54}},
			{Type: "text", Content: "，说服教育，治病救人。", BBox: []float64{40, 56, 280, 70}},
			{Type: "text", Content: "栏右缘参照行写满不参与这句。", BBox: []float64{40, 90, 520, 104}},
			// 句号后只有收引号，下一行是标题：不拼。
			{Type: "text", Content: "使党的面貌焕然一新。”", BBox: []float64{40, 120, 520, 134}},
			{Type: "paragraph_title", Content: "政策、纪律和理论教育", BBox: []float64{40, 136, 220, 150}},
			// 句号本身仍是收句。
			{Type: "text", Content: "反映的是马克思主义的精神实质。", BBox: []float64{40, 170, 520, 184}},
			{Type: "text", Content: "下一句另起。", BBox: []float64{40, 186, 180, 200}},
		},
	}}
	md := MarkdownFromPages(pages)
	if !strings.Contains(md, "自我批评”，说服教育") {
		t.Fatalf("quote without a sentence end must still join:\n%s", md)
	}
	if strings.Contains(md, "焕然一新。”政策、纪律和理论教育") {
		t.Fatalf("title after a quoted sentence must stay apart:\n%s", md)
	}
	if !strings.Contains(md, "政策、纪律和理论教育") {
		t.Fatalf("title lost:\n%s", md)
	}
	if strings.Contains(md, "精神实质。下一句") {
		t.Fatalf("period must still end the line:\n%s", md)
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
	lvl := headingLevel(md)
	if lvl("明确坚持") < 2 || lvl("明确坚持") != lvl("明确新时代") {
		t.Fatalf("multi-sentence ordinals stay with their series:\n%s", md)
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

	// 短 OCR title 仍置信；无编号的多句 OCR title 要降；有编号的多句保持标题
	pages := []Page{{
		Blocks: []Block{
			{Type: "paragraph_title", Content: "3.新时代中国特色社会主义思想。", BBox: []float64{40, 20, 300, 36}},
			{Type: "text", Content: "一、明确坚持和发展中国特色社会主义，总任务是实现社会主义现代化和中", BBox: []float64{40, 50, 500, 66}, Order: 1},
			{Type: "text", Content: "华民族伟大复兴。为了实现这一梦想，我们党提出两个一百年奋斗目标。", BBox: []float64{40, 68, 500, 98}, Order: 2},
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
	if !joinedYi {
		t.Fatalf("numbered 一、 stays a heading after wrap:\n%s", md2)
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

func TestMultiSentenceOrdinalStaysWithPeers(t *testing.T) {
	md := MarkdownFromLines([]string{
		"(3)1848年2月,马克思和恩格斯《共产党宣言》的发表,标志着马克思主义的公开问世,这是第一部全面阐述科学社会主义的伟大著作。",
		"(4)1867年,马克思撰写的《资本论》第一卷出版,系统阐述了剩余价值学说,揭示了资本主义生产关系的秘密。唯物史观和剩余价值学说是马克思一生的两个伟大发现。",
		"(5)1876—1878年,恩格斯写出了《反杜林论》,全面阐述了马克思主义理论体系。",
	})
	lvl := headingLevel(md)
	if lvl("(3)") < 2 || lvl("(3)") != lvl("(4)") || lvl("(4)") != lvl("(5)") {
		t.Fatalf("multi-sentence ordinal stays with its series:\n%s", md)
	}
}

func TestExtendedNumberSeries(t *testing.T) {
	md := MarkdownFromLines([]string{
		"第一章 总论",
		"第一节 导入",
		"1. 总则",
		"1。另写",
		"1.1 范围",
		"1.2 引用",
		"（一）原则",
		"（1）细则",
		"1）补充",
		"1 科学性",
		"2 人民性",
		"a.甲点",
		"b.乙点",
		"A、正确",
		"①第一条",
		"②第二条",
		"第二章 分论",
	})
	lvl := headingLevel(md)
	same := func(a, b string) {
		t.Helper()
		la, lb := lvl(a), lvl(b)
		if la < 2 || la != lb {
			t.Fatalf("%s and %s should share a level, got %d and %d\n%s", a, b, la, lb, md)
		}
	}
	deeper := func(child, parent string) {
		t.Helper()
		lc, lp := lvl(child), lvl(parent)
		if lc != lp+1 {
			t.Fatalf("%s should sit one level under %s, got %d and %d\n%s", child, parent, lc, lp, md)
		}
	}
	same("第一章", "第二章")
	deeper("第一节", "第一章")
	deeper("1.1", "1. 总则")
	same("1.1", "1.2")
	deeper("（1）", "（一）")
	deeper("1）补充", "（1）")
	same("1 科学性", "2 人民性")
	deeper("a.甲点", "1 科学性")
	same("a.甲点", "b.乙点")
	same("①第一条", "②第二条")
	if lvl("1。另写") > 0 {
		t.Fatalf("ideographic period is not a list dot:\n%s", md)
	}
	if lvl("A、正确") > 0 {
		t.Fatalf("uppercase option must not be a heading:\n%s", md)
	}
	short := MarkdownFromLines([]string{"a.甲点", "b.乙点", "①第一条", "②第二条", "第一篇 上", "第一章 总"})
	sl := headingLevel(short)
	if sl("a.甲点") < 2 || sl("a.甲点") != sl("b.乙点") || sl("①第一条") == sl("a.甲点") || sl("①第一条") != sl("②第二条") {
		t.Fatalf("letter dot and circle are different types:\n%s", short)
	}
	if sl("第一篇") < 2 || sl("第一章") == sl("第一篇") {
		t.Fatalf("pian and chapter are different types:\n%s", short)
	}
	lone := MarkdownFromLines([]string{"3 种看法", "后文"})
	if headingLevel(lone)("3 种看法") > 0 {
		t.Fatalf("a single digit-space line is not a heading:\n%s", lone)
	}
}

func headingLevel(md string) func(string) int {
	return func(substr string) int {
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
}
