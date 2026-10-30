package ocr

import (
	"math"
	"sort"
	"strings"

	"doc2md/internal/textnorm"
)

// GeometryPlain merges OCR lines using detector boxes when available.
// Large vertical gaps and short-wide→full-width transitions keep paragraph breaks;
// tight gaps within a column are joined as wrapped prose.
// Falls back to one text line per OCR line when boxes are missing.
func GeometryPlain(lines []Line) string {
	if len(lines) == 0 {
		return ""
	}
	boxed := 0
	for _, ln := range lines {
		if ln.HasBBox {
			boxed++
		}
	}
	if boxed*2 < len(lines) {
		return plainByLines(lines)
	}

	sorted := append([]Line(nil), lines...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Page != sorted[j].Page {
			return sorted[i].Page < sorted[j].Page
		}
		if sorted[i].Y0 != sorted[j].Y0 {
			return sorted[i].Y0 < sorted[j].Y0
		}
		return sorted[i].X0 < sorted[j].X0
	})

	byPage := map[int][]float64{}
	var widths []float64
	for _, ln := range sorted {
		if !ln.HasBBox {
			continue
		}
		h := ln.Y1 - ln.Y0
		if h > 1 {
			byPage[ln.Page] = append(byPage[ln.Page], h)
		}
		w := ln.X1 - ln.X0
		if w > 1 {
			widths = append(widths, w)
		}
	}
	medW := median(widths)
	if medW <= 0 {
		medW = 1000
	}

	var b strings.Builder
	var buf string
	flush := func() {
		t := strings.TrimSpace(buf)
		buf = ""
		if t == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(t)
	}

	var prev Line
	havePrev := false
	for _, ln := range sorted {
		t := strings.TrimSpace(ln.Text)
		if t == "" {
			continue
		}
		if !havePrev {
			buf = t
			prev = ln
			havePrev = true
			continue
		}
		if shouldGeomJoin(prev, ln, medW, byPage) {
			if needsSpace(buf, t) {
				buf += " " + t
			} else {
				buf += t
			}
			// keep left edge / top of run; extend bottom-right
			if ln.HasBBox && prev.HasBBox {
				prev.X1 = math.Max(prev.X1, ln.X1)
				prev.Y1 = math.Max(prev.Y1, ln.Y1)
				if ln.X0 < prev.X0 {
					prev.X0 = ln.X0
				}
			}
			prev.Text = buf
			continue
		}
		flush()
		buf = t
		prev = ln
	}
	flush()
	return b.String()
}

func plainByLines(lines []Line) string {
	var b strings.Builder
	for _, ln := range lines {
		t := strings.TrimSpace(ln.Text)
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(t)
	}
	return b.String()
}

func shouldGeomJoin(prev, next Line, medW float64, heightsByPage map[int][]float64) bool {
	left := strings.TrimSpace(prev.Text)
	right := strings.TrimSpace(next.Text)
	if left == "" || right == "" {
		return false
	}
	// Cross-page: geometry unreliable; only mid-phrase / unclosed joins.
	if prev.Page != next.Page {
		return textnorm.LineContinuesUnclosed(left, right) || softCrossPageJoin(left, right)
	}
	if !prev.HasBBox || !next.HasBBox {
		return false
	}
	if textnorm.IsShortStandaloneTitle(left) {
		return false
	}
	if textnorm.StartsWithMarker(right) && !textnorm.LineContinuesUnclosed(left, right) {
		return false
	}
	if textnorm.IsAnswerLabel(right) {
		return false
	}

	medH := median(heightsByPage[prev.Page])
	if medH <= 0 {
		medH = math.Max(8, prev.Y1-prev.Y0)
	}
	gap := next.Y0 - prev.Y1
	// Same visual row (multi-column) — do not glue horizontally here.
	if gap < -0.35*medH {
		return false
	}
	prevW := prev.X1 - prev.X0
	nextW := next.X1 - next.X0
	rPrev, _ := lastRune(left)
	rNext, _ := firstRune(right)
	openTail := strings.ContainsRune("的一了是不和与及以把被在为有其并从到按向对给让使", rPrev) ||
		rPrev == '《' || rPrev == '“' || rPrev == '"' || rPrev == '「'

	// Short line → full-width line: typical heading then body（虚词折行除外）
	if medW > 0 && prevW > 0 && nextW > 0 && !openTail && !textnorm.LineContinuesUnclosed(left, right) {
		if prevW < 0.55*medW && nextW > 0.75*medW {
			return false
		}
	}

	if textnorm.LineContinuesUnclosed(left, right) {
		return true
	}
	if strings.ContainsRune("。！？；：.!?;:", rPrev) {
		return false
	}

	// 大间隙默认分段；行末虚词/开符号 + 汉字/年份 仍视为折行
	if gap >= 2.5*medH {
		if openTail && (isCJK(rNext) || isASCIILetterOrDigit(rNext)) {
			return true
		}
		return false
	}

	// Tight gap: wrapped prose
	if rPrev == '《' || rPrev == '“' || rPrev == '"' || rPrev == '「' {
		return isCJK(rNext) || isASCIILetterOrDigit(rNext)
	}
	if isCJK(rPrev) && (isCJK(rNext) || isASCIILetterOrDigit(rNext) || rNext == '“' || rNext == '"' || rNext == '《') {
		return true
	}
	if isASCIILetterOrDigit(rPrev) && (isASCIILetterOrDigit(rNext) || isCJK(rNext)) {
		return true
	}
	return false
}

func softCrossPageJoin(left, right string) bool {
	if textnorm.IsShortStandaloneTitle(left) || textnorm.StartsWithMarker(right) {
		return false
	}
	rPrev, _ := lastRune(left)
	if strings.ContainsRune("。！？；：.!?;:", rPrev) {
		return false
	}
	rNext, _ := firstRune(right)
	if isCJK(rPrev) && (isCJK(rNext) || isASCIILetterOrDigit(rNext) || rNext == '“' || rNext == '"') {
		return true
	}
	return isASCIILetterOrDigit(rPrev) && (isCJK(rNext) || isASCIILetterOrDigit(rNext))
}

func needsSpace(left, right string) bool {
	rPrev, _ := lastRune(left)
	rNext, _ := firstRune(right)
	if rPrev == 0 || rNext == 0 {
		return false
	}
	return isASCIILetterOrDigit(rPrev) && isASCIILetterOrDigit(rNext)
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

func lastRune(s string) (rune, int) {
	s = strings.TrimRight(s, " \t")
	if s == "" {
		return 0, 0
	}
	r := []rune(s)
	return r[len(r)-1], len(r)
}

func firstRune(s string) (rune, int) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return 0, 0
	}
	r := []rune(s)
	return r[0], len(r)
}

func isCJK(r rune) bool {
	return r >= 0x4e00 && r <= 0x9fff
}

func isASCIILetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
