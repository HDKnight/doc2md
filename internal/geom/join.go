package geom

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"doc2md/internal/textnorm"
)

// Faithful port of pdf_md/join.py — decide whether two lines are one sentence.

var (
	reBareOCRNumber = regexp.MustCompile(`^\d{1,3}。$`)
	reDate          = regexp.MustCompile(`\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日|\d{4}\s*年\s*\d{1,2}\s*月|\d{1,2}\s*月\s*\d{1,2}\s*日|\d{4}\s*年`)
	reUnbreakable   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._%+-]{3,}`)
	sentenceStop    = "。！？；："
)

func endsSentence(text string) bool {
	stripped := strings.TrimSpace(text)
	if textnorm.IsMarkerOnly(stripped) {
		return false
	}
	if stripped == "" {
		return false
	}
	for _, c := range sentenceStop {
		if strings.HasSuffix(stripped, string(c)) {
			return true
		}
	}
	return false
}

func dateCrosses(left, right string) bool {
	// Match Python: suffix/prefix by rune count, regex indices on UTF-8 bytes.
	ls := []rune(strings.TrimRight(left, " \t"))
	rs := []rune(strings.TrimLeft(right, " \t"))
	if len(ls) > 16 {
		ls = ls[len(ls)-16:]
	}
	if len(rs) > 16 {
		rs = rs[:16]
	}
	suffix := string(ls)
	prefix := string(rs)
	combined := suffix + prefix
	boundary := len(suffix) // byte index into combined
	for _, m := range reDate.FindAllStringIndex(combined, -1) {
		if m[0] < boundary && boundary < m[1] {
			return true
		}
	}
	return false
}

func canJoinText(left, right string) bool {
	if textnorm.LineContinuesUnclosed(left, right) {
		return true
	}
	if endsSentence(left) {
		return false
	}
	// 短标题/小节标签不吞下行（简介、（1）短题、一、概念题…）
	if textnorm.IsShortStandaloneTitle(left) {
		return false
	}
	if textnorm.IsAnswerLabel(right) {
		return false
	}
	if textnorm.StartsWithMarker(right) && !dateCrosses(left, right) {
		return false
	}
	return true
}

// wordCrosses mirrors pdf-md._word_crosses.
// Without jieba, pdf-md returns True (allow merge when gap > body).
func wordCrosses(left, right string) bool {
	return true
}

func sameRow(left, right Line) bool {
	overlap := math.Min(left.Y1, right.Y1) - math.Max(left.Y0, right.Y0)
	if overlap <= 0 {
		return false
	}
	shorter := math.Min(left.Y1-left.Y0, right.Y1-right.Y0)
	return shorter > 0 && overlap >= shorter*0.5
}

func withSpace(left, right Line) Line {
	size := left.Em()
	gap := right.X0 - left.X1
	space := ""
	if gap > math.Max(2.0, size*0.34) {
		space = " "
	}
	return MergeBBox(left, right, strings.TrimRight(left.Text, " \t")+space+strings.TrimLeft(right.Text, " \t"))
}

func collapseRows(lines []Line) []Line {
	ordered := append([]Line(nil), lines...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Y0 == ordered[j].Y0 {
			return ordered[i].X0 < ordered[j].X0
		}
		return ordered[i].Y0 < ordered[j].Y0
	})
	var collapsed []Line
	for _, item := range ordered {
		if len(collapsed) > 0 && sameRow(collapsed[len(collapsed)-1], item) {
			a, b := collapsed[len(collapsed)-1], item
			if b.X0 < a.X0 {
				a, b = b, a
			}
			collapsed[len(collapsed)-1] = withSpace(a, b)
			continue
		}
		collapsed = append(collapsed, item)
	}
	return collapsed
}

func mergeText(left, right Line) Line {
	return MergeBBox(left, right, strings.TrimRight(left.Text, " \t")+strings.TrimLeft(right.Text, " \t"))
}

func columnRight(lines []Line) float64 {
	edges := make([]float64, len(lines))
	for i, item := range lines {
		edges[i] = item.X1
	}
	sort.Float64s(edges)
	idx := int(math.Round(float64(len(edges)-1) * 0.8))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(edges) {
		idx = len(edges) - 1
	}
	return edges[idx]
}

func atRightMargin(prev Line, right, em float64) bool {
	s := strings.TrimSpace(prev.Text)
	if textnorm.IsMarkerOnly(s) || s == "【答案】" || s == "【解析】" {
		return true
	}
	return right-prev.X1 <= em*2
}

func longTokenCouldNotFit(prev, nxt Line, right, em float64) bool {
	m := reUnbreakable.FindString(strings.TrimLeft(nxt.Text, " \t"))
	if m == "" || em <= 0 {
		return false
	}
	return float64(len(m))*em*0.55 > right-prev.X1
}

// JoinSeam merges column/page seam lines when text rules allow (ignores vertical gap).
func JoinSeam(left, right Line) *Line {
	stripped := strings.TrimSpace(left.Text)
	if textnorm.IsMarkerOnly(stripped) && !reBareOCRNumber.MatchString(stripped) {
		return nil
	}
	if canJoinText(left.Text, right.Text) {
		m := mergeText(left, right)
		return &m
	}
	return nil
}

// JoinLines merges wrapped lines within a column (pdf_md.join.join_lines).
func JoinLines(lines []Line) []Line {
	lines = collapseRows(lines)
	if len(lines) == 0 {
		return nil
	}
	var positive []float64
	for i := 0; i < len(lines)-1; i++ {
		gap := lines[i+1].Y0 - lines[i].Y1
		if gap > 0 {
			positive = append(positive, gap)
		}
	}
	median := 14.0
	if len(positive) > 0 {
		median = medianFloat(positive)
	}
	var heights []float64
	for _, item := range lines {
		if h := item.Height(); h > 0 {
			heights = append(heights, h)
		}
	}
	pitch := median
	if len(heights) > 0 {
		pitch = medianFloat(heights)
	}
	// A page of tight options pulls the median gap down; body wraps still sit about one line apart.
	body := math.Max(median, pitch*1.35)
	limit := math.Max(median*1.4, pitch*1.8)
	right := columnRight(lines)

	merged := []Line{lines[0]}
	for _, item := range lines[1:] {
		prev := merged[len(merged)-1]
		gap := item.Y0 - prev.Y1
		em := prev.FontSize
		if em <= 0 {
			em = prev.Height()
		}
		if em <= 0 {
			em = 12
		}
		indented := item.X0-prev.X0 > em
		ps := strings.TrimSpace(prev.Text)
		if ps == "【答案】" || ps == "【解析】" {
			indented = false
		}
		if !canJoinText(prev.Text, item.Text) || indented || gap > limit {
			merged = append(merged, item)
			continue
		}
		if !atRightMargin(prev, right, pitch) && !longTokenCouldNotFit(prev, item, right, pitch) {
			merged = append(merged, item)
			continue
		}
		if gap > body && !wordCrosses(prev.Text, item.Text) {
			merged = append(merged, item)
			continue
		}
		merged[len(merged)-1] = mergeText(prev, item)
	}
	return merged
}

func medianFloat(vals []float64) float64 {
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
