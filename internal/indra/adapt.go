// Package indra adapts indra-ocr JSON into feat.Page for the feat pipeline branch.
// It does not modify oar or feat heading logic.
package indra

import (
	"encoding/json"
	"sort"
	"strings"

	"doc2md/internal/feat"
)

// rawBlock is one layout block from indra --format json.
type rawBlock struct {
	Label string    `json:"label"`
	Text  string    `json:"text"`
	BBox  []float64 `json:"bbox"`
}

// rawLine is one OCR line (reading order).
type rawLine struct {
	Text string    `json:"text"`
	Box  []float64 `json:"box"`
	Conf float64   `json:"conf"`
}

// rawPage is one JSONL line from indra --format json.
type rawPage struct {
	Page   int        `json:"page"` // 0-based in CLI output
	Blocks []rawBlock `json:"blocks"`
	Lines  []rawLine  `json:"lines"`
}

// MapLabel maps indra layout labels to oar-like feat types.
func MapLabel(label string) (typ string, drop bool) {
	switch strings.TrimSpace(label) {
	case "Page-Header", "Page-Footer":
		return "", true
	case "Section-Header", "Table-Of-Contents":
		return "paragraph_title", false
	// 选择题选项栏常被误判成 Table；当 text 交给 feat，避免假表格毁掉版面
	case "Table", "Text", "List-Group", "Caption", "Footnote", "Form":
		return "text", false
	default:
		return "text", false
	}
}

// ToFeatPages converts indra JSONL into feat pages.
// 优先用 lines（阅读序、避免假 Table）；无 lines 时再退回 blocks。
func ToFeatPages(jsonl []byte) ([]feat.Page, error) {
	rawLines := splitNonEmptyLines(jsonl)
	var raws []rawPage
	for _, line := range rawLines {
		var p rawPage
		if err := json.Unmarshal(line, &p); err != nil {
			return nil, err
		}
		raws = append(raws, p)
	}
	sort.Slice(raws, func(i, j int) bool { return raws[i].Page < raws[j].Page })
	out := make([]feat.Page, 0, len(raws))
	for _, rp := range raws {
		if len(rp.Lines) > 0 {
			out = append(out, pageFromLines(rp.Lines))
			continue
		}
		out = append(out, pageFromBlocks(rp.Blocks))
	}
	return out, nil
}

func pageFromLines(lines []rawLine) feat.Page {
	// OrderNet 常对双栏选项走「先左列再右列」→ A,C,B,D；改按几何行带重排（同行左→右，再下行）。
	// 同一行带合并成一块，避免「2、（ A」与「）是…」被拆开后 feat 把半截抬成标题。
	lines = sortLinesRowMajor(lines)
	merged := mergeRowBands(lines)
	var blocks []feat.Block
	order := 0
	for _, ln := range merged {
		content := strings.TrimSpace(ln.Text)
		if content == "" {
			continue
		}
		if isPageNumberOnly(content) {
			continue
		}
		order++
		blocks = append(blocks, feat.Block{
			Type:    "text",
			Content: content,
			Order:   order,
			BBox:    append([]float64(nil), ln.Box...),
		})
	}
	return feat.Page{Blocks: blocks}
}

// mergeRowBands 把 sortLinesRowMajor 后的同行片段拼成一行文本。
func mergeRowBands(lines []rawLine) []rawLine {
	if len(lines) < 2 {
		return lines
	}
	tol := rowBandTol(lines)
	var out []rawLine
	var band []rawLine
	flush := func() {
		if len(band) == 0 {
			return
		}
		out = append(out, joinLineBand(band))
		band = nil
	}
	for _, ln := range lines {
		if len(band) == 0 {
			band = []rawLine{ln}
			continue
		}
		if lineY0(ln)-lineY0(band[0]) <= tol {
			band = append(band, ln)
			continue
		}
		flush()
		band = []rawLine{ln}
	}
	flush()
	return out
}

func joinLineBand(band []rawLine) rawLine {
	if len(band) == 1 {
		return band[0]
	}
	var b strings.Builder
	var x0, y0, x1, y1 float64
	var confSum float64
	for i, ln := range band {
		t := strings.TrimSpace(ln.Text)
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			prev, _ := utf8Last(b.String())
			next, _ := utf8First(t)
			if !joinNoSpace(prev, next) {
				b.WriteByte(' ')
			}
		}
		b.WriteString(t)
		if len(ln.Box) >= 4 {
			if i == 0 || ln.Box[0] < x0 {
				x0 = ln.Box[0]
			}
			if i == 0 || ln.Box[1] < y0 {
				y0 = ln.Box[1]
			}
			if ln.Box[2] > x1 {
				x1 = ln.Box[2]
			}
			if ln.Box[3] > y1 {
				y1 = ln.Box[3]
			}
		}
		confSum += ln.Conf
	}
	return rawLine{
		Text: b.String(),
		Box:  []float64{x0, y0, x1, y1},
		Conf: confSum / float64(len(band)),
	}
}

func utf8First(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

func utf8Last(s string) (rune, bool) {
	var last rune
	ok := false
	for _, r := range s {
		last, ok = r, true
	}
	return last, ok
}

func joinNoSpace(prev, next rune) bool {
	if prev == 0 || next == 0 {
		return true
	}
	// 半截括号粘回「（ A）」；连续 CJK 不插空格；选项「李大钊」+「B、」仍留空格
	if prev == '（' || prev == '(' || next == '）' || next == ')' {
		return true
	}
	if isCJK(prev) && isCJK(next) {
		return true
	}
	return false
}

func isCJK(r rune) bool {
	return r >= 0x3000 && r <= 0x9FFF || r >= 0xF900 && r <= 0xFAFF || r >= 0xFF00 && r <= 0xFFEF
}

// sortLinesRowMajor: 按 y 分「行带」，带内按 x。修复双栏 A/B / C/D 被读成 A/C/B/D。
func sortLinesRowMajor(lines []rawLine) []rawLine {
	if len(lines) < 2 {
		return lines
	}
	out := append([]rawLine(nil), lines...)
	sort.SliceStable(out, func(i, j int) bool {
		return lineY0(out[i]) < lineY0(out[j])
	})
	tol := rowBandTol(out)
	var bands [][]rawLine
	for _, ln := range out {
		if len(bands) == 0 {
			bands = append(bands, []rawLine{ln})
			continue
		}
		last := bands[len(bands)-1]
		refY := lineY0(last[0])
		if lineY0(ln)-refY <= tol {
			bands[len(bands)-1] = append(last, ln)
			continue
		}
		bands = append(bands, []rawLine{ln})
	}
	flat := make([]rawLine, 0, len(out))
	for _, band := range bands {
		sort.SliceStable(band, func(i, j int) bool {
			return lineX0(band[i]) < lineX0(band[j])
		})
		flat = append(flat, band...)
	}
	return flat
}

func lineY0(ln rawLine) float64 {
	if len(ln.Box) >= 2 {
		return ln.Box[1]
	}
	return 0
}

func lineX0(ln rawLine) float64 {
	if len(ln.Box) >= 1 {
		return ln.Box[0]
	}
	return 0
}

func lineH(ln rawLine) float64 {
	if len(ln.Box) >= 4 {
		h := ln.Box[3] - ln.Box[1]
		if h > 1 {
			return h
		}
	}
	return 20
}

func rowBandTol(lines []rawLine) float64 {
	if len(lines) == 0 {
		return 20
	}
	hs := make([]float64, len(lines))
	for i, ln := range lines {
		hs[i] = lineH(ln)
	}
	sort.Float64s(hs)
	med := hs[len(hs)/2]
	tol := med * 0.6
	if tol < 12 {
		tol = 12
	}
	if tol > 80 {
		tol = 80
	}
	return tol
}

func pageFromBlocks(in []rawBlock) feat.Page {
	var blocks []feat.Block
	order := 0
	for _, b := range in {
		typ, drop := MapLabel(b.Label)
		if drop {
			continue
		}
		content := strings.TrimSpace(b.Text)
		if content == "" {
			continue
		}
		order++
		blocks = append(blocks, feat.Block{
			Type:    typ,
			Content: content,
			Order:   order,
			BBox:    append([]float64(nil), b.BBox...),
		})
	}
	return feat.Page{Blocks: blocks}
}

func isPageNumberOnly(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func splitNonEmptyLines(b []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i <= len(b); i++ {
		if i == len(b) || b[i] == '\n' {
			line := b[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if len(strings.TrimSpace(string(line))) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	return lines
}
