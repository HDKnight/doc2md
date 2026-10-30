package pipeline

import (
	"math"
	"sort"
)

// sortReadingOrder：对齐旧 geom.ReadingOrder ——
// 按行心 x 聚成栏（栏心间距 > 版心 20%），双栏则左栏整栏再右栏；否则纯 y/x。
func sortReadingOrder(items []item) []item {
	if len(items) < 2 {
		return items
	}
	cols := columnGroups(items)
	if cols == nil {
		out := append([]item(nil), items...)
		sort.SliceStable(out, func(i, j int) bool {
			if almostEq(out[i].y0, out[j].y0, 3) {
				return out[i].x0 < out[j].x0
			}
			return out[i].y0 < out[j].y0
		})
		return out
	}
	out := make([]item, 0, len(items))
	for _, col := range cols {
		sort.SliceStable(col, func(i, j int) bool { return col[i].y0 < col[j].y0 })
		out = append(out, col...)
	}
	return out
}

func almostEq(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func xCenter(it item) float64 {
	if it.w > 0 {
		return it.x0 + it.w/2
	}
	return it.x0
}

func pageWidthOf(items []item) float64 {
	var max float64
	for _, it := range items {
		r := it.x0 + it.w
		if r > max {
			max = r
		}
		if it.x0 > max {
			max = it.x0
		}
	}
	if max < 1 {
		return 1000
	}
	return max
}

func columnGroups(items []item) [][]item {
	if len(items) < 10 {
		return nil
	}
	ordered := append([]item(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return xCenter(ordered[i]) < xCenter(ordered[j])
	})
	width := pageWidthOf(items)
	groups := [][]item{{ordered[0]}}
	for _, it := range ordered[1:] {
		last := groups[len(groups)-1]
		if xCenter(it)-xCenter(last[len(last)-1]) > width*0.2 {
			groups = append(groups, []item{it})
		} else {
			groups[len(groups)-1] = append(groups[len(groups)-1], it)
		}
	}
	if len(groups) != 2 {
		return nil
	}
	for _, g := range groups {
		if len(g) < 5 {
			return nil
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return minX0(groups[i]) < minX0(groups[j])
	})
	left, right := groups[0], groups[1]
	if minX0(right) < maxRight(left) {
		return nil
	}
	return [][]item{left, right}
}

func minX0(items []item) float64 {
	m := items[0].x0
	for _, it := range items[1:] {
		if it.x0 < m {
			m = it.x0
		}
	}
	return m
}

func maxRight(items []item) float64 {
	m := items[0].x0 + items[0].w
	for _, it := range items[1:] {
		r := it.x0 + it.w
		if r > m {
			m = r
		}
	}
	return m
}
