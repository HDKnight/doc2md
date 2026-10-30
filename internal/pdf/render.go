package pdf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func pdftoppmBin() string {
	return popplerBin("DOC2MD_PDFTOPPM", "pdftoppm")
}

// RenderPagePNG renders a single 1-based page to PNG bytes (200 DPI default for diagrams).
func RenderPagePNG(ctx context.Context, pdfPath string, page, dpi int) ([]byte, error) {
	if dpi <= 0 {
		dpi = 200
	}
	bin := pdftoppmBin()
	if err := lookPoppler(bin); err != nil {
		return nil, fmt.Errorf("pdf renderer not found (%s): put Poppler in tools/poppler or set DOC2MD_PDFTOPPM", bin)
	}
	workDir, err := os.MkdirTemp("", "doc2md-page-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)
	prefix := filepath.Join(workDir, "page")
	cmd := exec.CommandContext(ctx, bin,
		"-png", "-r", strconv.Itoa(dpi),
		"-f", strconv.Itoa(page), "-l", strconv.Itoa(page),
		pdfPath, prefix,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pdftoppm page %d: %w: %s", page, err, strings.TrimSpace(string(out)))
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			return os.ReadFile(filepath.Join(workDir, e.Name()))
		}
	}
	return nil, fmt.Errorf("pdftoppm produced no png for page %d", page)
}

// RenderPagesToPNG renders PDF pages to PNG files using pdftoppm when available.
// Returns PNG file paths in page order. Caller should remove workDir when done.
// Env DOC2MD_PDFTOPPM overrides binary name/path (default: pdftoppm).
func RenderPagesToPNG(ctx context.Context, pdfPath string, dpi int) (workDir string, pngPaths []string, err error) {
	return renderPages(ctx, pdfPath, dpi, "png", false)
}

// RenderPagesForOCR renders grayscale JPEG pages for fast OCR (much quicker than PNG).
func RenderPagesForOCR(ctx context.Context, pdfPath string, dpi int) (workDir string, imgPaths []string, err error) {
	return renderPages(ctx, pdfPath, dpi, "jpeg", true)
}

func renderPages(ctx context.Context, pdfPath string, dpi int, format string, gray bool) (workDir string, paths []string, err error) {
	if dpi <= 0 {
		dpi = 150
	}
	bin := pdftoppmBin()
	if lookErr := lookPoppler(bin); lookErr != nil {
		return "", nil, fmt.Errorf("pdf renderer not found (%s): run start.bat to auto-install Poppler into tools/poppler, or set DOC2MD_PDFTOPPM", bin)
	}
	workDir, err = os.MkdirTemp("", "doc2md-render-*")
	if err != nil {
		return "", nil, err
	}
	prefix := filepath.Join(workDir, "page")
	args := []string{"-r", strconv.Itoa(dpi)}
	if gray {
		args = append(args, "-gray")
	}
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		args = append(args, "-jpeg")
	default:
		args = append(args, "-png")
	}
	args = append(args, pdfPath, prefix)
	cmd := exec.CommandContext(ctx, bin, args...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		_ = os.RemoveAll(workDir)
		return "", nil, fmt.Errorf("pdftoppm: %w: %s", runErr, strings.TrimSpace(string(out)))
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return "", nil, err
	}
	exts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if exts[ext] {
			paths = append(paths, filepath.Join(workDir, e.Name()))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		_ = os.RemoveAll(workDir)
		return "", nil, fmt.Errorf("pdftoppm produced no image pages")
	}
	return workDir, paths, nil
}
