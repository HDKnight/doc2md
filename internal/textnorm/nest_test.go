package textnorm

import (
	"strings"
	"testing"

	"doc2md/internal/markdown"
)

func renderHeads(blocks []markdown.Block) string {
	var md strings.Builder
	for _, b := range blocks {
		if b.Type != "heading" {
			continue
		}
		md.WriteString(strings.Repeat("#", b.Level))
		md.WriteByte(' ')
		md.WriteString(b.Text)
		md.WriteByte('\n')
	}
	return md.String()
}

func TestNestNewFamilyUnderParent(t *testing.T) {
	// pdf-md: test_new_family_nests_under_the_open_parent
	lines := []string{
		"一、总",
		"（一）子",
		"（二）丑",
		"二、分",
		"（一）寅",
	}
	out := renderHeads(BuildOutline(lines))
	if !strings.Contains(out, "## 一、总") || !strings.Contains(out, "### （一）子") {
		t.Fatalf("nest failed:\n%s", out)
	}
	if !strings.Contains(out, "## 二、分") {
		t.Fatalf("missing 二、分:\n%s", out)
	}
	if strings.Contains(out, "### （一）寅") || strings.Contains(out, "## （一）寅") {
		t.Fatalf("singleton nested heading:\n%s", out)
	}
	// Peer markers stay at the same depth (no last_level+1 ratchet).
	if !strings.Contains(out, "### （二）丑") {
		t.Fatalf("(二) should stay peer of (一):\n%s", out)
	}
}

func TestOutlineHintForcesTitle(t *testing.T) {
	lines := []string{
		"简介",
		"正文段落下不应把长题干抬成标题。",
		"一、单项选择题",
		"1. 短题干甲 ( )",
		"2. 短题干乙 ( )",
	}
	out := renderHeads(BuildOutlineOpt(lines, []OutlineHint{
		{Title: "简介", Level: 1},
		{Title: "一、单项选择题", Level: 2},
	}))
	if !strings.Contains(out, "## 简介") && !strings.Contains(out, "# 简介") {
		t.Fatalf("hint title missing:\n%s", out)
	}
	if !strings.Contains(out, "一、单项选择题") {
		t.Fatalf("section hint missing:\n%s", out)
	}
}

func TestPeerCnParenSameLevel(t *testing.T) {
	lines := []string{
		"第三章 理想",
		"（一）方向甲",
		"（二）方向乙",
		"（三）方向丙",
		"（一）财富甲",
		"（二）财富乙",
	}
	out := renderHeads(BuildOutline(lines))
	// All （一）（二）（三） peers share one depth under 章.
	depthYi := headingDepth(out, "（一）方向甲")
	depthEr := headingDepth(out, "（二）方向乙")
	depthNew := headingDepth(out, "（一）财富甲")
	if depthYi == 0 || depthEr == 0 {
		t.Fatalf("missing cn_paren heads:\n%s", out)
	}
	if depthYi != depthEr {
		t.Fatalf("peer depth drift %d vs %d:\n%s", depthYi, depthEr, out)
	}
	if depthNew != 0 && depthNew != depthYi {
		t.Fatalf("restarted （一） should match peer depth %d got %d:\n%s", depthYi, depthNew, out)
	}
}

func headingDepth(md, title string) int {
	for _, line := range strings.Split(md, "\n") {
		if !strings.Contains(line, title) {
			continue
		}
		n := 0
		for _, c := range line {
			if c == '#' {
				n++
				continue
			}
			break
		}
		return n
	}
	return 0
}

func TestNestDotUnderCnDun(t *testing.T) {
	// pdf-md: test_dot_number_nests_under_the_open_parent_not_the_dun_number
	lines := []string{
		"2、黄埔军校思想政治教育的基本经验",
		"第一，建立政治工作。",
		"第二，确立革命观念。",
		"3、试论述延安整风运动的方针及基本经验",
		"一、延安整风运动是思想教育运动。",
		"二、方针是惩前毖后，治病救人。",
		"三、基本经验：",
		"1.坚持实事求是的思想路线",
		"2.加强党员干部的党性修养",
	}
	out := renderHeads(BuildOutline(lines))
	if !strings.Contains(out, "## 2、黄埔军校思想政治教育的基本经验") {
		t.Fatalf("missing parent:\n%s", out)
	}
	if !strings.Contains(out, "### 三、基本经验：") {
		t.Fatalf("cn_dun not nested:\n%s", out)
	}
	if !strings.Contains(out, "#### 1.坚持实事求是的思想路线") {
		t.Fatalf("dot not nested under cn_dun:\n%s", out)
	}
}

func TestNotesLongTopLevelStaysBody(t *testing.T) {
	lines := []string{
		"74",
		"1.马克思主义的基本立场 是马克思主义观察、分析和解决问题的根本立足点和出发点。这就是始终站在无产阶级和人民大众的立场上，以无产阶级的解放和全人类的解放为己任。",
		"2.马克思主义的基本观点 是关于自然、社会和人类思维发展一般规律的科学认识，是对人类思想成果和社会实践经验的科学总结。",
		"3.马克思主义的基本方法 是建立在辩证唯物主义和历史唯物主义世界观和方法论基础上。",
		"1.观察当代世界变化的认识工具。",
		"2.指引当代中国发展的行动指南。",
	}
	blocks := BuildOutline(lines)
	for _, b := range blocks {
		if b.Type == "heading" && b.Text == "74" {
			t.Fatalf("page number became heading: %q", b.Text)
		}
	}
	out := renderHeads(blocks)
	longD := headingDepth(out, "1.马克思主义的基本立场")
	shortD := headingDepth(out, "1.观察当代世界变化的认识工具")
	if longD == 0 || shortD == 0 || longD != shortD {
		t.Fatalf("same integer_dot family should share one level (long=%d short=%d):\n%s", longD, shortD, out)
	}
}

func TestNestQuestionsUnderSection(t *testing.T) {
	// pdf-md: numbered questions under 题型 are headings
	lines := []string{
		"一、单项选择题",
		"1、1945年毛泽东称其为总司令。",
		"2、（A）是传播马克思主义的第一人。",
		"4、1915年杂志创办标志着兴起。",
		"二、名词解释",
	}
	out := renderHeads(BuildOutline(lines))
	if !strings.Contains(out, "### 1、1945年毛泽东称其为总司令。") {
		t.Fatalf("questions should nest as headings:\n%s", out)
	}
	if !strings.Contains(out, "### 2、（A）是传播马克思主义的第一人。") {
		t.Fatalf("missing q2 heading:\n%s", out)
	}
}
