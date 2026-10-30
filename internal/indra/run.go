package indra

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ResolveBin finds indra-ocr: DOC2MD_INDRA → .venv-indra/Scripts → PATH.
func ResolveBin() string {
	if p := strings.TrimSpace(os.Getenv("DOC2MD_INDRA")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	names := []string{"indra-ocr.exe", "indra-ocr"}
	for _, root := range searchRoots() {
		for _, n := range names {
			cands := []string{
				filepath.Join(root, ".venv-indra", "Scripts", n),
				filepath.Join(root, ".venv-indra", "bin", n),
				filepath.Join(root, "tools", n),
			}
			for _, p := range cands {
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					return p
				}
			}
		}
	}
	if p, err := exec.LookPath("indra-ocr"); err == nil {
		return p
	}
	if p, err := exec.LookPath("indra-ocr.exe"); err == nil {
		return p
	}
	return ""
}

// ResolveDPI: DOC2MD_INDRA_DPI → 否则 request（若在 72–300）→ 默认 200。
// 注意：convert 未传 DPI 时会塞 150（历史默认），对 indra 过糊；150 视为未指定，改用 200。
func ResolveDPI(request int) int {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_INDRA_DPI")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 72 && n <= 300 {
			return n
		}
	}
	if request >= 72 && request <= 300 && request != 150 {
		return request
	}
	return 200
}

// RunJSON writes PDF to a temp file, runs indra-ocr --format json --mode ocr, returns JSONL bytes.
func RunJSON(ctx context.Context, pdf []byte, dpi int) (jsonl []byte, err error) {
	bin := ResolveBin()
	if bin == "" {
		return nil, fmt.Errorf("indra-ocr not found (set DOC2MD_INDRA or install .venv-indra)")
	}
	dpi = ResolveDPI(dpi)

	tmpPDF, err := os.CreateTemp("", "doc2md-indra-*.pdf")
	if err != nil {
		return nil, err
	}
	pdfPath := tmpPDF.Name()
	defer os.Remove(pdfPath)
	if _, err := tmpPDF.Write(pdf); err != nil {
		tmpPDF.Close()
		return nil, err
	}
	if err := tmpPDF.Close(); err != nil {
		return nil, err
	}

	outDir, err := os.MkdirTemp("", "doc2md-indra-out-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outDir)

	args := []string{
		pdfPath,
		"--format", "json",
		"--mode", "ocr",
		"--dpi", strconv.Itoa(dpi),
		"--out", outDir,
	}
	// 默认拉 HF v3.1.1（含 rec_cjk）；包自带 v1.1.0 中文质量明显更差
	if md, err := ResolveModelDir(); err == nil && md != "" {
		args = append(args, "--model-dir", md)
	} else if err != nil && os.Getenv("DOC2MD_INDRA_ALLOW_DEFAULT_MODELS") != "1" {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("indra-ocr: %s", msg)
	}

	matches, err := filepath.Glob(filepath.Join(outDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("indra-ocr: no .jsonl in output")
	}
	return os.ReadFile(matches[0])
}

// ModelLabel for OCRStats（便于 UI 核对是否在用 CJK 模型）。
func ModelLabel() string {
	if _, err := ResolveModelDir(); err != nil {
		return "default"
	}
	if modelTag != "" {
		return modelTag
	}
	return PreferredHFRevision
}

func searchRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		roots = append(roots, p)
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		p := wd
		for i := 0; i < 6; i++ {
			add(p)
			parent := filepath.Dir(p)
			if parent == p {
				break
			}
			p = parent
		}
	}
	return roots
}
