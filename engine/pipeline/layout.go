package pipeline

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

func (b Block) x0() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[0]
}
func (b Block) y0() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[1]
}
func (b Block) x1() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[2]
}
func (b Block) y1() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[3]
}
func (b Block) height() float64 {
	h := b.y1() - b.y0()
	if h < 1 {
		return 1
	}
	return h
}
func (b Block) width() float64 {
	w := b.x1() - b.x0()
	if w < 1 {
		return 1
	}
	return w
}

func estimateEm(boxH, boxW float64, text string) float64 {
	n := float64(utf8.RuneCountInString(strings.ReplaceAll(text, "\n", "")))
	if boxH < 1 {
		return 1
	}
	if n < 1 {
		return boxH
	}
	const pitch = 1.25
	const aspect = 0.88
	if n*aspect*boxH <= boxW*1.25 {
		return boxH
	}
	best := boxH / pitch
	for lines := 2; lines <= 60; lines++ {
		em := boxH / (pitch * float64(lines))
		if em < 4 {
			break
		}
		cpl := math.Max(1, boxW/(aspect*em))
		need := int(math.Ceil(n / cpl))
		if need <= lines {
			return em
		}
		best = em
	}
	return best
}

func pageItems(blocks []Block, page int) []item {
	items := make([]item, 0, len(blocks))
	for _, b := range blocks {
		t := strings.TrimSpace(b.Content)
		if t == "" {
			continue
		}
		h, w := b.height(), b.width()
		items = append(items, item{
			text: t,
			typ:  strings.ToLower(strings.TrimSpace(b.Type)),
			em:   estimateEm(h, w, t),
			w:    w,
			x0:   b.x0(), y0: b.y0(), y1: b.y1(),
			page: page,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y0 == items[j].y0 {
			return items[i].x0 < items[j].x0
		}
		return items[i].y0 < items[j].y0
	})
	return items
}

func medianBodyEm(items []item) float64 {
	var vals []float64
	for _, it := range items {
		if typeIsTitle(it.typ) || isOptionLine(it.text) || isStructuralLabel(it.text) {
			continue
		}
		if it.em > 0 {
			vals = append(vals, it.em)
		}
	}
	if len(vals) == 0 {
		for _, it := range items {
			if it.em > 0 {
				vals = append(vals, it.em)
			}
		}
	}
	return medianFloat(vals)
}

func medianFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 12
	}
	sort.Float64s(vals)
	return vals[len(vals)/2]
}
