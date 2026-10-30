package textnorm

import (
	"strings"
	"testing"
)

func TestWeakIntegerNeedsConsecutivePeers(t *testing.T) {
	// 单独 / 不成连续：不当标题
	lone := renderHeads(BuildOutline([]string{
		"一、概述",
		"1坚持了唯物主义一元论，这是很长的论述正文。",
		"其他说明文字。",
	}))
	if headingDepth(lone, "1坚持了") != 0 && headingDepth(lone, "1 坚持了") != 0 {
		t.Fatalf("lone weak integer became heading:\n%s", lone)
	}
	scattered := renderHeads(BuildOutline([]string{
		"一、概述",
		"3种基本形式需要掌握。",
		"7个主要环节如下说明。",
		"19个具体要求写在后面。",
	}))
	if headingDepth(scattered, "3种") != 0 || headingDepth(scattered, "3 种") != 0 ||
		headingDepth(scattered, "7个") != 0 || headingDepth(scattered, "7 个") != 0 {
		t.Fatalf("scattered weak integers became headings:\n%s", scattered)
	}
	// 成族且连续：升标题
	run := renderHeads(BuildOutline([]string{
		"一、要点",
		"1 核心要义与内涵",
		"2 核心内容与要求",
		"3 基本方略与路径",
	}))
	if headingDepth(run, "1 核心要义与内涵") == 0 ||
		headingDepth(run, "2 核心内容与要求") == 0 ||
		headingDepth(run, "3 基本方略与路径") == 0 {
		t.Fatalf("consecutive weak integers should be headings:\n%s", run)
	}
}

func TestOrdinalRunStable(t *testing.T) {
	long := "中央农民运动讲习所:全称“中国国民党中央农民运动讲习所”。是1927年1月,根据毛泽东的倡议在武昌创办的,由邓演达负责,毛泽东主持,周以粟担任教务长。"
	lines := []string{
		"二、 名词解释",
		"3、 " + long,
		"4、 " + long,
		"17、 新式整军运动:",
		"（二） 现实意义:",
		"1 政治工作为经济工作和其他一切工作和建设提供了正确的政治导向。",
		"2 政治工作能为全面建设小康社会目标的实现提供强大的精神动力。",
		"3 政治工作能够通过营建安定团结的政治局面和稳定的社会秩序,为经济工作和其他工作提供健康发展的保障环境。",
	}
	out := renderHeads(BuildOutline(lines))
	d3 := headingDepth(out, "3、 中央农民运动讲习所")
	d4 := headingDepth(out, "4、 中央农民运动讲习所")
	d17 := headingDepth(out, "17、 新式整军运动:")
	if d3 == 0 || d3 != d4 || d4 != d17 {
		t.Fatalf("same integer_dun not one level (3=%d 4=%d 17=%d):\n%s", d3, d4, d17, out)
	}
	s1 := headingDepth(out, "1 政治工作为经济工作")
	s2 := headingDepth(out, "2 政治工作能为全面建设")
	s3 := headingDepth(out, "3 政治工作能够通过营建")
	if s1 == 0 || s1 != s2 || s2 != s3 || s1 != d3+1 {
		t.Fatalf("space ordinals should share one deeper level (dun=%d 1=%d 2=%d 3=%d):\n%s", d3, s1, s2, s3, out)
	}
}

func TestNestLongMarkersAnswerBlock(t *testing.T) {
	lines := []string{
		"【答案】导语。",
		"（1）习近平新时代中国特色社会主义思想的内容",
		"1 核心要义与内涵",
		"坚持和发展中国特色社会主义,是鲜明主题。",
		"2 核心内容:“八个明确”",
		"a.明确坚持和发展中国特色社会主义,总任务是实现社会主义现代化和中华民族伟大复兴。",
		"b.明确新时代我国社会主要矛盾是人民日益增长的美好生活需要和不平衡不充分的发展之间的矛盾。",
		"3 基本方略:“十四个坚持”",
		"坚持党对一切工作的领导。",
		"（2）习近平新时代中国特色社会主义思想的意义",
		"1 开辟了马克思主义新境界。习近平新时代中国特色社会主义思想对马克思主义在21世纪的发展做出了重大原创性贡献。",
		"2 开辟了中国特色社会主义新境界。以习近平同志为核心的党中央坚持和发展中国特色社会主义。",
	}
	out := renderHeads(BuildOutlineWith(lines, OutlineConfig{KeepExamStemsBody: true}))
	if !strings.Contains(out, "（1）习近平新时代中国特色社会主义思想的内容") {
		t.Fatalf("missing (1):\n%s", out)
	}
	if !strings.Contains(out, "1 核心要义与内涵") {
		t.Fatalf("missing short 1:\n%s", out)
	}
	if !strings.Contains(out, "a.明确坚持和发展中国特色社会主义") {
		t.Fatalf("long latin should nest as heading:\n%s", out)
	}
	if !strings.Contains(out, "1 开辟了马克思主义新境界") {
		t.Fatalf("long integer answer item should nest:\n%s", out)
	}
	d1 := headingDepth(out, "1 核心要义与内涵")
	da := headingDepth(out, "a.明确坚持和发展中国特色社会主义")
	if d1 == 0 || da == 0 || da <= d1 {
		t.Fatalf("a. should nest deeper than 1 (got 1=%d a=%d):\n%s", d1, da, out)
	}
}
