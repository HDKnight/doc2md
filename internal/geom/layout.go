package geom

import (
	"sort"
	"strings"
)

// Clusters groups lines by horizontal center gaps (> 20% page width).
func Clusters(lines []Line) [][]Line {
	if len(lines) == 0 {
		return nil
	}
	ordered := append([]Line(nil), lines...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].XCenter() < ordered[j].XCenter()
	})
	width := ordered[0].PageWidth
	groups := [][]Line{{ordered[0]}}
	for _, item := range ordered[1:] {
		last := groups[len(groups)-1]
		if item.XCenter()-last[len(last)-1].XCenter() > width*0.2 {
			groups = append(groups, []Line{item})
		} else {
			groups[len(groups)-1] = append(groups[len(groups)-1], item)
		}
	}
	return groups
}

// ColumnGroups returns left/right columns when clearly dual-column, else nil.
func ColumnGroups(lines []Line) [][]Line {
	groups := Clusters(lines)
	if len(groups) != 2 {
		return nil
	}
	for _, g := range groups {
		if len(g) < 5 {
			return nil
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return minX(groups[i]) < minX(groups[j])
	})
	left, right := groups[0], groups[1]
	if minX(right) < maxX(left) {
		return nil
	}
	return [][]Line{sortByY(left), sortByY(right)}
}

// ReadingOrder returns left-then-right for dual columns, else top-to-bottom.
func ReadingOrder(lines []Line) []Line {
	cols := ColumnGroups(lines)
	if cols != nil {
		out := make([]Line, 0, len(lines))
		out = append(out, cols[0]...)
		out = append(out, cols[1]...)
		return out
	}
	ordered := append([]Line(nil), lines...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Y0 == ordered[j].Y0 {
			return ordered[i].X0 < ordered[j].X0
		}
		return ordered[i].Y0 < ordered[j].Y0
	})
	return ordered
}

// IsDiagram detects mind-map style pages (pdf-md rules).
func IsDiagram(lines []Line) bool {
	if len(lines) < 8 {
		return false
	}
	short := 0
	for _, item := range lines {
		if len([]rune(strings.TrimSpace(item.Text))) <= 8 {
			short++
		}
	}
	if short*2 < len(lines) {
		return false
	}
	for _, g := range Clusters(lines) {
		if len(g)*2 >= len(lines) {
			return false
		}
	}
	return true
}

func minX(lines []Line) float64 {
	m := lines[0].X0
	for _, l := range lines[1:] {
		if l.X0 < m {
			m = l.X0
		}
	}
	return m
}

func maxX(lines []Line) float64 {
	m := lines[0].X1
	for _, l := range lines[1:] {
		if l.X1 > m {
			m = l.X1
		}
	}
	return m
}

func sortByY(lines []Line) []Line {
	out := append([]Line(nil), lines...)
	sort.Slice(out, func(i, j int) bool { return out[i].Y0 < out[j].Y0 })
	return out
}
