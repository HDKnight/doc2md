package textnorm

import (
	"strings"
	"testing"

	"doc2md/internal/markdown"
)

func TestSplitGluedShortTitlesYanAn(t *testing.T) {
	lines := []string{
		"1. 坚持实事求是的思想路线",
		"延安时期党的思想路线是实事求是。",
		"2. 加强党员干部的党性修养",
		"整风运动既是一次深刻的马克思主义教育运动。",
		"3. 贯彻治病救人的方针政策党内不同思想的对立和斗争是经常发生的,这是社会的阶级矛盾和新旧事物矛盾在党内的反映。",
	}
	out := ExpandGluedLines(lines)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "3. 贯彻治病救人的方针政策\n党内不同思想") {
		t.Fatalf("glued title not split:\n%s", joined)
	}
	if strings.Contains(joined, "方针政策党内") {
		t.Fatalf("still glued:\n%s", joined)
	}
	// OCR 常在标题与引语正文间无换行：方针政策“党内…
	quoted := ExpandGluedLines([]string{
		"1. 坚持实事求是的思想路线",
		"正文甲。",
		"2. 加强党员干部的党性修养",
		"正文乙。",
		"3. 贯彻治病救人的方针政策“党内不同思想的对立和斗争是经常发生的,这是社会的阶级矛盾。",
	})
	qj := strings.Join(quoted, "\n")
	if !strings.Contains(qj, "3. 贯彻治病救人的方针政策\n“党内不同思想") {
		t.Fatalf("quoted glue not split:\n%s", qj)
	}
}

func TestSplitGluedShortTitlesSkipsLongStems(t *testing.T) {
	lines := []string{
		"1. 1844年8月底，马克思和恩格斯在巴黎会面。这次会面使他们发现彼此的基本观点完全一致。",
		"2. 马克思主义是关于无产阶级和人类解放的科学，是科学的世界观和方法论。",
	}
	out := ExpandGluedLines(lines)
	joined := strings.Join(out, "\n")
	if strings.Count(joined, "\n") > strings.Count(strings.Join(lines, "\n"), "\n") {
		t.Fatalf("long stems should not be split:\n%s", joined)
	}
}

func TestToBlocksSplitsGluedShortTitle(t *testing.T) {
	text := "" +
		"1. 坚持实事求是的思想路线\n" +
		"延安时期党的思想路线是实事求是。\n" +
		"2. 加强党员干部的党性修养\n" +
		"整风运动既是一次深刻的马克思主义教育运动。\n" +
		"3. 贯彻治病救人的方针政策党内不同思想的对立和斗争是经常发生的,这是社会的阶级矛盾和新旧事物矛盾在党内的反映。\n"
	blocks := ToBlocks(text)
	var heads []string
	for _, b := range blocks {
		if b.Type == "heading" {
			heads = append(heads, b.Text)
		}
	}
	joined := strings.Join(heads, "\n")
	if !strings.Contains(joined, "贯彻治病救人的方针政策") {
		t.Fatalf("title missing from headings:\n%s", joined)
	}
	for _, h := range heads {
		if strings.Contains(h, "方针政策党内") {
			t.Fatalf("heading still contains body:\n%s", h)
		}
	}
}

func TestFormatOrdinalsKeepsSplitQuotedBody(t *testing.T) {
	// 全链路：拆粘连后不得因下行「“」被 soft-join 粘回标题
	in := "" +
		"1. 坚持实事求是的思想路线\n延安时期。\n" +
		"2. 加强党员干部的党性修养\n整风运动。\n" +
		"3. 贯彻治病救人的方针政策“党内不同思想的对立和斗争是经常发生的,这是社会的阶级矛盾和新旧事物矛盾在党内的反映。\n"
	blocks := ToBlocks(in)
	md := FinishMarkdown(markdown.Render(blocks))
	if strings.Contains(md, "方针政策“党内") || strings.Contains(md, "方针政策党内") {
		t.Fatalf("rejoined after format:\n%s", md)
	}
	if !strings.Contains(md, "贯彻治病救人的方针政策") || !strings.Contains(md, "“党内不同思想") {
		t.Fatalf("missing parts:\n%s", md)
	}
}
