package pdf

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

var pdfInfoPagesRe = regexp.MustCompile(`(?m)^Pages:\s+(\d+)\s*$`)

// PageCountFile returns the page count of a PDF on disk.
// pdfinfo is used when Poppler is present; otherwise the bytes are scanned for /Count.
func PageCountFile(path string, data []byte) int {
	if n := pdfInfoPages(path); n > 0 {
		return n
	}
	return PageCountHint(data)
}

func pdfInfoPages(path string) int {
	if path == "" {
		return 0
	}
	bin := popplerBin("DOC2MD_PDFINFO", "pdfinfo")
	if lookPoppler(bin) != nil && !fileExists(bin) {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, path).Output()
	if err != nil {
		return 0
	}
	out = bytes.ReplaceAll(out, []byte("\r"), nil)
	m := pdfInfoPagesRe.FindSubmatch(out)
	if len(m) != 2 {
		return 0
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
