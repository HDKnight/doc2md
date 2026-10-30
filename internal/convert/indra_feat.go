package convert

import (
	"context"
	"fmt"
	"strings"
	"time"

	"doc2md/internal/indra"
	"doc2md/internal/ocr"
)

// runIndraFeatPDF: indra-ocr JSON → adapt → 保守 Markdown（不跑 feat 标题启发式，避免误抬标题）。
func runIndraFeatPDF(ctx context.Context, data []byte, dpi int) (md, plain string, stats *OCRStats, err error) {
	_ = ocr.DefaultPool().Close()

	t0 := time.Now()
	dpi = indra.ResolveDPI(dpi)
	jsonl, err := indra.RunJSON(ctx, data, dpi)
	if err != nil {
		return "", "", nil, err
	}
	pages, err := indra.ToFeatPages(jsonl)
	if err != nil {
		return "", "", nil, fmt.Errorf("indra adapt: %w", err)
	}
	// 不用 feat.MarkdownFromPages：行级块极易被序号/短行启发式抬成标题，护栏兜不住。
	md = indra.MarkdownFromPages(pages)
	var plainB strings.Builder
	for _, pg := range pages {
		for _, b := range pg.Blocks {
			t := strings.TrimSpace(b.Content)
			if t == "" {
				continue
			}
			if plainB.Len() > 0 {
				plainB.WriteByte('\n')
			}
			plainB.WriteString(t)
		}
	}
	plain = plainB.String()
	st := &OCRStats{
		Used:  true,
		Model: fmt.Sprintf("indra;dpi=%d;models=%s;conservative", dpi, indra.ModelLabel()),
		Pages: len(pages),
	}
	st.TotalMs = time.Since(t0).Milliseconds()
	if st.Pages > 0 {
		st.AvgMsPerPage = float64(st.TotalMs) / float64(st.Pages)
	}
	return md, plain, st, nil
}
