//go:build ignore

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"doc2md/internal/feat"
)

func main() {
	root, _ := os.Getwd()
	if filepath.Base(root) == "scripts" {
		root = filepath.Dir(root)
	}
	dir := filepath.Join(root, "testdata", "out", "oar_structure_json")
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	sort.Slice(files, func(i, j int) bool { return pnum(files[i]) < pnum(files[j]) })

	type row struct {
		page, order int
		typ, text   string
		em, boxH    float64
		n           int
		bodyEm      float64
		ratio       float64
	}
	var rows []row
	for _, f := range files {
		b, _ := os.ReadFile(f)
		p, err := feat.ParsePageJSON(b)
		if err != nil {
			panic(err)
		}
		page := pnum(f)
		// recompute body em same as package (exported helper)
		ems := []float64{}
		type it struct {
			em   float64
			text string
			boxH float64
			w    float64
		}
		items := []it{}
		for _, bl := range p.Blocks {
			t := strings.TrimSpace(bl.Content)
			if t == "" {
				continue
			}
			h := bl.Y1() - bl.Y0()
			if h < 1 {
				h = 1
			}
			w := bl.X1() - bl.X0()
			if w < 1 {
				w = 1
			}
			em := estEm(h, w, t)
			items = append(items, it{em: em, text: t, boxH: h, w: w})
			n := utf8.RuneCountInString(t)
			if n >= 36 && !strings.HasSuffix(t, "？") && !strings.HasSuffix(t, "?") {
				ems = append(ems, em)
			}
		}
		body := med(ems)
		if body < 1 {
			var all []float64
			for _, it := range items {
				all = append(all, it.em)
			}
			body = med(all)
		}
		for i, it := range items {
			n := utf8.RuneCountInString(it.text)
			r := 0.0
			if body > 0 {
				r = it.em / body
			}
			typ := ""
			if i < len(p.Blocks) {
				// find matching
			}
			_ = typ
			rows = append(rows, row{page: page, order: i + 1, text: it.text, em: it.em, boxH: it.boxH, n: n, bodyEm: body, ratio: r})
		}
	}

	fmt.Printf("page\tn\tratio\tem\tbody\tboxH\ttext\n")
	for _, r := range rows {
		mark := ""
		if r.ratio >= 1.18 && r.n <= 48 {
			mark = "H?"
		}
		if r.n > 72 {
			mark = "long"
		}
		fmt.Printf("p%d\t%d\t%.2f\t%.1f\t%.1f\t%.0f\t%s\t%s\n", r.page, r.n, r.ratio, r.em, r.bodyEm, r.boxH, mark, trunc(r.text, 36))
	}
}

func estEm(boxH, boxW float64, text string) float64 {
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

func med(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[len(c)/2]
}

func pnum(path string) int {
	base := filepath.Base(path)
	i := strings.LastIndex(base, "_p")
	if i < 0 {
		return 0
	}
	rest := strings.TrimSuffix(base[i+2:], filepath.Ext(base))
	n, _ := strconv.Atoi(rest)
	return n
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
