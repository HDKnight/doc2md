package oarx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"doc2md/engine/pipeline"
	"doc2md/internal/ocr"
	"doc2md/internal/pdf"
)

// Stats mirrors convert OCR timing for UI.
type Stats struct {
	Used         bool
	Model        string
	Pages        int
	TotalMs      int64
	AvgMsPerPage float64
}

// structurePageBatchSize：每次送 worker 的页数。
// 默认 0=整本一次 IPC（worker 内部仍按页推理，峰值与分批接近，少开关 PDF/少往返）。
// 内存紧时设 DOC2MD_OAR_STRUCTURE_BATCH=2。
func structurePageBatchSize() int {
	v := strings.TrimSpace(os.Getenv("DOC2MD_OAR_STRUCTURE_BATCH"))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// RunStructureJSON runs oar structure --format json and returns pages + plain text.
func RunStructureJSON(ctx context.Context, pdfData []byte, dpi int) (pages []pipeline.Page, plain string, stats *Stats, err error) {
	dpi = ocr.ResolveStructureDPI(dpi)
	batchSize := structurePageBatchSize()

	tmpPDF, err := os.CreateTemp("", "doc2md-engine-*.pdf")
	if err != nil {
		return nil, "", nil, err
	}
	pdfPath := tmpPDF.Name()
	defer os.Remove(pdfPath)
	if _, err := tmpPDF.Write(pdfData); err != nil {
		tmpPDF.Close()
		return nil, "", nil, err
	}
	if err := tmpPDF.Close(); err != nil {
		return nil, "", nil, err
	}

	outDir, err := os.MkdirTemp("", "doc2md-engine-out-*")
	if err != nil {
		return nil, "", nil, err
	}
	defer os.RemoveAll(outDir)

	t0 := time.Now()
	_ = ocr.DefaultPool().Close()

	dev := ocr.OarStructureDevice()
	devLabel := dev
	if devLabel == "" {
		devLabel = "auto"
	}
	if note := ocr.OarDeviceNote(); note != "" {
		devLabel = devLabel + ";" + note
	}

	mode := "cli"
	warm := ocr.DefaultStructureWarm()
	// 多路 CPU：oar CLI 按副本并行翻页；warm worker 单会话串行，同机实测更慢（~26s vs ~15s/11p）。
	useWarm := warm.Available() && preferStructureWarm(dev)

	nHint := pdf.PageCountFile(pdfPath, pdfData)
	replicas := 1
	if useWarm && structureUsesGPUPool(dev) {
		if batchSize == 0 {
			replicas = ocr.GPUStructureReplicas(nHint)
		}
		ocr.DefaultStructurePool().ReleaseAbove(replicas)
	}
	if replicas > 1 {
		fmt.Fprintf(os.Stderr, "doc2md: structure GPU replicas=%d pages=%d\n", replicas, nHint)
		threads := ocr.StructureIntraOpThreads(replicas)
		pool := ocr.DefaultStructurePool()
		for i := 0; i < replicas; i++ {
			pool.Slot(i).Prepare(threads)
		}
		if err := runWarmParallel(ctx, pool, pdfPath, outDir, dpi, nHint, replicas); err != nil {
			fmt.Fprintf(os.Stderr, "doc2md/engine: structure parallel failed (%v); fallback CLI\n", err)
			_ = removeOutJSON(outDir)
			if err := runCLIStructure(ctx, pdfPath, outDir, dpi, dev, nil); err != nil {
				return nil, "", nil, err
			}
			mode = "cli"
		} else {
			mode = fmt.Sprintf("warmx%d", replicas)
		}
	} else {
		batches := pageBatches(nHint, batchSize)
		for bi, batch := range batches {
			select {
			case <-ctx.Done():
				return nil, "", nil, ctx.Err()
			default:
			}
			batchOut := outDir
			if len(batches) > 1 {
				batchOut = filepath.Join(outDir, fmt.Sprintf("b%d", bi))
				if err := os.MkdirAll(batchOut, 0o755); err != nil {
					return nil, "", nil, err
				}
			}
			usedWarm := false
			if useWarm {
				var werr error
				if len(batch) == 0 {
					_, werr = warm.RunStructure(ctx, pdfPath, batchOut, dpi, "json")
				} else {
					_, werr = warm.RunStructurePages(ctx, pdfPath, batchOut, dpi, "json", batch)
				}
				if werr != nil {
					fmt.Fprintf(os.Stderr, "doc2md/engine: structure warm batch %v failed (%v); fallback CLI\n", batch, werr)
					_ = os.RemoveAll(batchOut)
					_ = os.MkdirAll(batchOut, 0o755)
				} else {
					usedWarm = true
					mode = "warm"
				}
			}
			if !usedWarm {
				if err := runCLIStructure(ctx, pdfPath, batchOut, dpi, dev, batch); err != nil {
					return nil, "", nil, err
				}
				mode = "cli"
			}
			if batchOut != outDir {
				if err := mergeJSONDir(batchOut, outDir); err != nil {
					return nil, "", nil, err
				}
			}
		}
	}

	pages, plain, n, err := readJSONPages(outDir)
	if err != nil {
		return nil, "", nil, err
	}
	batchLabel := batchSize
	if batchLabel <= 0 {
		batchLabel = n
		if batchLabel < 1 {
			batchLabel = 0
		}
	}
	st := &Stats{
		Used:  true,
		Model: fmt.Sprintf("engine-json;dpi=%d;%s;%s;batch=%d", dpi, devLabel, mode, batchLabel),
		Pages: n,
	}
	st.TotalMs = time.Since(t0).Milliseconds()
	if st.Pages > 0 {
		st.AvgMsPerPage = float64(st.TotalMs) / float64(st.Pages)
	}
	return pages, plain, st, nil
}

// preferStructureWarm：GPU/单路或显式强制时用常驻 worker；多路 CPU 默认走 CLI 并行。
// DOC2MD_OAR_STRUCTURE_WARM=1/0 可强制开/关。
func preferStructureWarm(dev string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DOC2MD_OAR_STRUCTURE_WARM"))) {
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}
	n := 0
	for _, p := range strings.Split(dev, ",") {
		if strings.EqualFold(strings.TrimSpace(p), "cpu") {
			n++
		}
	}
	return n < 2
}

// structureUsesGPUPool：独显单路（DirectML auto 或 cuda:N）才在进程外再拆页。
// cpu×N 已经由 oar CLI 自己并行；逗号设备列表也交给 CLI。
func structureUsesGPUPool(dev string) bool {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return true
	}
	if strings.Contains(dev, ",") {
		return false
	}
	low := strings.ToLower(dev)
	if strings.Contains(low, "cpu") {
		return false
	}
	return strings.HasPrefix(low, "cuda:")
}

func splitPages(n, replicas int) [][]int {
	if n < 1 || replicas < 1 {
		return nil
	}
	if replicas > n {
		replicas = n
	}
	base := n / replicas
	extra := n % replicas
	out := make([][]int, 0, replicas)
	page := 1
	for i := 0; i < replicas; i++ {
		size := base
		if i < extra {
			size++
		}
		group := make([]int, 0, size)
		for j := 0; j < size; j++ {
			group = append(group, page)
			page++
		}
		out = append(out, group)
	}
	return out
}

func runWarmParallel(ctx context.Context, pool *ocr.StructurePool, pdfPath, outDir string, dpi, nPages, replicas int) error {
	groups := splitPages(nPages, replicas)
	if len(groups) < 2 {
		return fmt.Errorf("not enough pages to split (%d)", nPages)
	}
	gctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(groups))
	for i, group := range groups {
		wg.Add(1)
		go func(i int, group []int) {
			defer wg.Done()
			sub := filepath.Join(outDir, fmt.Sprintf("w%d", i))
			if err := os.MkdirAll(sub, 0o755); err != nil {
				errCh <- err
				cancel()
				return
			}
			if _, err := pool.Slot(i).RunStructurePages(gctx, pdfPath, sub, dpi, "json", group); err != nil {
				errCh <- err
				cancel()
				return
			}
			if err := mergeJSONDir(sub, outDir); err != nil {
				errCh <- err
				cancel()
			}
		}(i, group)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func removeOutJSON(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	return nil
}

func pageBatches(pageCount, batch int) [][]int {
	if pageCount <= 0 || batch <= 0 || batch >= pageCount {
		// 未知页数 / 整本一次：nil → worker/CLI 自扫全部页
		return [][]int{nil}
	}
	var out [][]int
	for i := 1; i <= pageCount; i += batch {
		end := i + batch - 1
		if end > pageCount {
			end = pageCount
		}
		var b []int
		for p := i; p <= end; p++ {
			b = append(b, p)
		}
		out = append(out, b)
	}
	return out
}

func runCLIStructure(ctx context.Context, pdfPath, outDir string, dpi int, dev string, pages []int) error {
	bin := ocr.ResolveOarBin()
	if bin == "" {
		return fmt.Errorf("oar CLI not found (need `oar` on PATH or DOC2MD_OCR_OAR)")
	}
	args := []string{
		"structure", pdfPath,
		"--dpi", strconv.Itoa(dpi),
		"--format", "json",
		"-o", outDir,
	}
	if len(pages) > 0 {
		args = append(args, "--pages", formatPageRanges(pages))
	}
	if dev != "" {
		args = append(args, "--device", dev)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = ocr.OarStructureEnviron(dev)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("oar structure: %s", msg)
	}
	return nil
}

func formatPageRanges(pages []int) string {
	if len(pages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(pages))
	for _, p := range pages {
		parts = append(parts, strconv.Itoa(p))
	}
	return strings.Join(parts, ",")
}

func mergeJSONDir(src, dst string) error {
	files, err := filepath.Glob(filepath.Join(src, "*.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		base := filepath.Base(f)
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, base), raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readJSONPages(outDir string) ([]pipeline.Page, string, int, error) {
	files, err := filepath.Glob(filepath.Join(outDir, "*.json"))
	if err != nil {
		return nil, "", 0, err
	}
	sort.Slice(files, func(i, j int) bool {
		return pageNum(files[i]) < pageNum(files[j])
	})
	var pages []pipeline.Page
	var plainB strings.Builder
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, "", 0, err
		}
		pg, err := ParsePageJSON(raw)
		if err != nil {
			return nil, "", 0, fmt.Errorf("parse %s: %w", filepath.Base(f), err)
		}
		pages = append(pages, pg)
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
	return pages, plainB.String(), len(files), nil
}

// pageNum：从 oar 产物名 `{stem}_p{N}.json` 取页码。
// 不能扫文件名里「第一串数字」——临时 PDF stem（doc2md-engine-123）会把所有页都解析成同一个数，
// 再叠加 Glob 字典序变成 p1,p10,p11,p2… 乱序。
func pageNum(path string) int {
	base := filepath.Base(path)
	if i := strings.LastIndex(base, "_p"); i >= 0 {
		n := 0
		for _, r := range base[i+2:] {
			if r >= '0' && r <= '9' {
				n = n*10 + int(r-'0')
			} else {
				break
			}
		}
		if n > 0 {
			return n
		}
	}
	return 0
}
