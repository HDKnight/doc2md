package textnorm

import (
	"strings"
	"testing"
)

func TestJoinWrappedClosingParen(t *testing.T) {
	in := "论断的提出者是(B\n)。\nA、《实践论》"
	out := JoinWrappedLines(Normalize(in))
	if !strings.Contains(out, "(B)。") {
		t.Fatalf("got %q", out)
	}
	in2 := "明确提出了“(B\n)”的方针"
	out2 := JoinWrappedLines(Normalize(in2))
	if !strings.Contains(out2, "(B)”") && !strings.Contains(out2, "“(B)”") {
		t.Fatalf("quote paren: %q", out2)
	}
	// 题号行也会被当成短标题，仍须拼回闭括号
	in3 := "34、1937年8月,毛泽东写的(B\n),阐释了唯物辩证法的最根本的法则,"
	out3 := JoinWrappedLines(Normalize(in3))
	if !strings.Contains(out3, "(B),阐释") {
		t.Fatalf("numbered stem: %q", out3)
	}
	in4 := "30、1935年12月,中国共产党在(\nD)召开政治局扩大会议,制定了抗日民族统一战线的新政策。"
	out4 := JoinWrappedLines(Normalize(in4))
	if !strings.Contains(out4, "在(D)召开") {
		t.Fatalf("filled blank split: %q", out4)
	}
}

func TestChoiceBlankRegex(t *testing.T) {
	in := "" +
		"26、 中国共产党从幼年走向成熟的标志是指(\nA)。\n\nA、 遵义会议的召开\n\nB、 “生命线”论断的提出\n\n" +
		"27、 1936年10月9日,红一、二、四方面军三大主力部队在甘肃(B)\n会师,胜利地结束了长征。\n\nA、 瓦窑堡\n\nB、 会宁\n\n" +
		"28、 1937年7月7日(\nB)的爆发标志着中国进入全面抗战时期。\n\nA、 九一八事变\n\n" +
		"30、 1935年12月,中国共产党在(\nD)召开政治局扩大会议,制定了抗日民族统一战线的新政策。\n\nA、 洛川\n"
	out := fixChoiceBlankBreaks(in)
	for _, want := range []string{"指(A)。", "甘肃(B)会师", "日(B)的爆发", "在(D)召开"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s\n%s", want, out)
		}
	}
	if strings.Contains(out, "召开A、") || strings.Contains(out, "(A)A、") || strings.Contains(out, "时期。A、") {
		t.Fatalf("option swallowed\n%s", out)
	}
	// 未闭合 (D 后接选项：不得吞掉后续换行
	orphan := fixChoiceBlankBreaks("59、中心环节是(D\nA、政治整顿\n60、下一题\n")
	if !strings.Contains(orphan, "(D\nA、") && !strings.Contains(orphan, "(D\nA、政治") {
		if strings.Contains(orphan, "(DA、") || strings.Contains(orphan, "整顿60、") {
			t.Fatalf("orphan paren ate newlines:\n%s", orphan)
		}
	}
	if strings.Contains(orphan, "整顿60、") {
		t.Fatalf("orphan paren glued next question:\n%s", orphan)
	}
	headed := FormatMarkdownOrdinals("#### 26、 中国共产党从幼年走向成熟的标志是指(\n\nA)。\n\nA、 遵义会议的召开\n")
	if !strings.Contains(headed, "指(A)。") {
		t.Fatalf("heading:\n%s", headed)
	}
	if strings.Contains(headed, "指(A)。A、") {
		t.Fatalf("heading ate option:\n%s", headed)
	}
}

func TestDateHasNoLineBreak(t *testing.T) {
	out := finishLayout("1938年\n5月,毛泽东在延安作了演讲。\n1936年10月\n9日会师。\n一九三七年\n八月\n七日。\n")
	for _, want := range []string{"1938年5月", "1936年10月9日", "一九三七年八月七日"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s\n%s", want, out)
		}
	}
	if strings.Contains(out, "年\n") || strings.Contains(out, "月\n9") || strings.Contains(out, "月\n七") {
		t.Fatalf("date still broken:\n%s", out)
	}
}

func TestGeneralParenNewlineAndEmphasis(t *testing.T) {
	out := finishLayout("会后在（\n说明）\n如下，不再换行。\n【要点】注意\n参见[2018研]题\n[text](http://a)\n见[1]脚注\n")
	if !strings.Contains(out, "（说明）如下") {
		t.Fatalf("paren newline:\n%s", out)
	}
	if strings.Contains(out, "说明）\n如下") {
		t.Fatalf("tail newline kept:\n%s", out)
	}
	kept := finishLayout("（说明）。\n下一段仍然分开。\n")
	if !strings.Contains(kept, "（说明）。\n下一段") {
		t.Fatalf("punct should keep break:\n%s", kept)
	}
	if !strings.Contains(out, "**【要点】** 注意") {
		t.Fatalf("corner label:\n%s", out)
	}
	if !strings.Contains(out, "**[2018研]** 题") {
		t.Fatalf("square label:\n%s", out)
	}
	if strings.Contains(out, "**[text]**") {
		t.Fatalf("link was bolded:\n%s", out)
	}
	if strings.Contains(out, "**[1]**") {
		t.Fatalf("numeric footnote bolded:\n%s", out)
	}
	plain := finishLayout("他说“完成了”，\n并且离开。\n")
	if !strings.Contains(plain, "完成了”，并且离开。") {
		t.Fatalf("general fragment:\n%s", plain)
	}
}

func TestColonMayBreakLine(t *testing.T) {
	in := "（一）历史意义：\n原理和中国革命斗争的实际情况。\n"
	out := FormatMarkdownOrdinals(in)
	if strings.Contains(out, "意义：原理") {
		t.Fatalf("colon newline should stay:\n%s", out)
	}
	if !strings.Contains(out, "意义：") || !strings.Contains(out, "原理和") {
		t.Fatalf("missing parts:\n%s", out)
	}
}

func TestShortQuotedTitleDoesNotSwallowBody(t *testing.T) {
	in := "### （2） “三查三整”的方法步骤\n\n这次整党是结合土地改革运动进行的,整顿的重点对象是农村基层党组织和党员。\n"
	out := FormatMarkdownOrdinals(in)
	if strings.Contains(out, "方法步骤这次整党") {
		t.Fatalf("short quoted title swallowed body:\n%s", out)
	}
	if !strings.Contains(out, "方法步骤") || !strings.Contains(out, "这次整党是结合") {
		t.Fatalf("missing parts:\n%s", out)
	}
}

func TestStemJoinsYearAfterCong(t *testing.T) {
	// 标题折行：…第一个阶段,从 + 2020年到2035年
	in := "#### 一、 明确坚持和发展中国特色社会主义,总任务是实现社会主义现代化和中华民族伟大复兴。" +
		"第一个阶段,从\n\n2020年到2035年,基本实现社会主义现代化;第二个阶段,从2035年到本世纪中叶。\n"
	out := FormatMarkdownOrdinals(in)
	if !strings.Contains(out, "阶段,从2020年到2035年") {
		t.Fatalf("从|2020 not joined:\n%s", out)
	}
	// 词中折行：推进国|家治理
	guo := FormatMarkdownOrdinals("#### 四、 明确全面深化改革总目标是完善和发展中国特色社会主义制度、推进国\n\n家治理体系和治理能力现代化。\n")
	if !strings.Contains(guo, "推进国家治理体系") {
		t.Fatalf("国|家 not joined:\n%s", guo)
	}
}

func TestHeadingMidPhraseWrap(t *testing.T) {
	in := "##### (一) 历史意义:“政治工作是红军的生命线”论断符合马克思主义的基本\n\n原理和中国革命斗争的实际情况,它的提出明确了政治工作是保证我军生命的必要要素。\n"
	out := FormatMarkdownOrdinals(in)
	if !strings.Contains(out, "基本原理和中国革命斗争") {
		t.Fatalf("heading mid-wrap not joined:\n%s", out)
	}
	// 纯小标题无「已展开正文」信号，即使很短/很长标题名也不吞下行
	kept := FormatMarkdownOrdinals("##### (一) 历史意义\n\n原理和中国革命斗争的实际情况。\n")
	if strings.Contains(kept, "历史意义原理") {
		t.Fatalf("label heading swallowed body:\n%s", kept)
	}
	kept2 := FormatMarkdownOrdinals("##### (一) 这是一个没有冒号和逗号的超长小节标题名称用于回归\n\n原理和中国革命斗争的实际情况。\n")
	if strings.Contains(kept2, "回归原理") || strings.Contains(kept2, "名称用于回归原理") {
		t.Fatalf("long label without prose cue swallowed body:\n%s", kept2)
	}
}

func TestStemFragmentRejoin(t *testing.T) {
	in := "" +
		"32、 1938年5月,毛泽东在延安作了(A)的演讲,驳斥了“亡国论”\n和“速胜论”。\n\nA、 《抗日游击战的战略问题》\n\n" +
		"#### 34、 1937年8月,毛泽东写的(B),阐释了唯物辩证法的最根本的法则,\n\n为思想政治教育奠定了世界观和方法论基础。\n\nA、 《实践论》\n"
	out := FormatMarkdownOrdinals(in)
	if !strings.Contains(out, "“亡国论”和“速胜论”。") {
		t.Fatalf("quote fragment:\n%s", out)
	}
	if !strings.Contains(out, "法则,为思想政治教育奠定了世界观和方法论基础。") {
		t.Fatalf("comma fragment:\n%s", out)
	}
	if strings.Contains(out, "论”。A、") || strings.Contains(out, "基础。A、") {
		t.Fatalf("option swallowed:\n%s", out)
	}
	// 「第一次…」不得被 第X章 误判为结构行（复习题 65 断章）
	q65 := FormatMarkdownOrdinals(
		"#### 65、 1967年5月 18 日《人民日报》、《红旗》杂志发表文章《伟大的历史文件》，\n\n" +
			"第一次明确提出“(    )”这一概念。\n\nA、 以阶级斗争为纲\n",
	)
	if !strings.Contains(q65, "文件》，第一次明确提出") {
		t.Fatalf("diyi fragment not joined:\n%s", q65)
	}
	if strings.Contains(q65, "概念。A、") {
		t.Fatalf("option swallowed:\n%s", q65)
	}
	kept := FormatMarkdownOrdinals("1、什么是马克思主义\n马克思主义是关于无产阶级解放的科学。\n")
	if strings.Contains(kept, "主义马克思主义") {
		t.Fatalf("short title joined:\n%s", kept)
	}
}

func TestUnclosedBlankSurvivesHeading(t *testing.T) {
	in := "#### 28、 1937年7月7日(\n\nB)的爆发标志着中国进入全面抗战时期。\n\nA、 九一八事变\n\nB、 卢沟桥事变"
	out := FormatMarkdownOrdinals(in)
	if !strings.Contains(out, "日(B)的爆发") {
		t.Fatalf("heading blank not closed:\n%s", out)
	}
	if strings.Contains(out, "日(B)A、") || strings.Contains(out, "时期。A、") {
		t.Fatalf("option swallowed:\n%s", out)
	}
	plain := "28、 1937年7月7日(\nB)的爆发标志着中国进入全面抗战时期。\nA、九一八事变"
	got := FormatMarkdownOrdinals(plain)
	if !strings.Contains(got, "日(B)的爆发") {
		t.Fatalf("plain blank not closed:\n%s", got)
	}
	if strings.Contains(got, "爆发A、") {
		t.Fatalf("option glued:\n%s", got)
	}
}

func TestQuizStemRejoinsBlankAndKeepsOrdinalHeading(t *testing.T) {
	// 断行括号拼回；特殊序号题干仍为标题
	text := "" +
		"30、1935年12月,中国共产党在(\nD)召开政治局扩大会议,制定了抗日民族统一战线的新政策。\n" +
		"A、洛川\nB、直罗镇\nC、延安\nD、瓦窑堡\n" +
		"31、1937年8月,中国共产党在(B)召开会议,提出了党在抗日战争统一战线中的领导权问题和加强党对抗日战争的政治领导的主张。\n" +
		"A、瓦窑堡\nB、洛川\nC、黄陵\nD、枣园\n"
	blocks := ToBlocks(text)
	var heads []string
	for _, b := range blocks {
		if b.Type == "heading" {
			heads = append(heads, b.Text)
		}
	}
	joined := strings.Join(heads, "\n")
	if !strings.Contains(joined, "在(D)召开") {
		t.Fatalf("30 blank not rejoined into heading:\n%s", joined)
	}
	if !strings.Contains(joined, "31、") && !strings.Contains(joined, "31、 ") {
		t.Fatalf("31 ordinal should be heading:\n%s", joined)
	}
}
