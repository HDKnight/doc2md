package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OarCLI runs the oar-ocr CLI (https://github.com/GreatV/oar-ocr) as a one-shot
// subprocess: `oar ocr --format json --size <tiny|small|medium> <image>`.
//
// Each call may reload models; warm up once before timing. Default production
// path remains faster-paddle via NewFromEnv(); use DOC2MD_OCR_ENGINE=oar or
// NewOarFromEnv for trials.
type OarCLI struct {
	Bin     string
	Timeout time.Duration
	size    string // tiny|small|medium
}

// ConfigureOCR maps doc2md model names to oar --size (tiny/small; medium via env).
func (o *OarCLI) ConfigureOCR(model string, _ bool) {
	if o == nil {
		return
	}
	o.size = normalizeOarSize(model)
}

func normalizeOarSize(model string) string {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_OCR_OAR_SIZE")); v != "" {
		switch strings.ToLower(v) {
		case "tiny", "small", "medium":
			return strings.ToLower(v)
		}
	}
	switch NormalizeModel(model) {
	case "small":
		return "small"
	default:
		return "tiny"
	}
}

func (o *OarCLI) sizeOrDefault() string {
	if o != nil && o.size != "" {
		return o.size
	}
	return normalizeOarSize("")
}

func (o *OarCLI) timeout() time.Duration {
	if o != nil && o.Timeout > 0 {
		return o.Timeout
	}
	return 180 * time.Second
}

// Close is a no-op (no persistent worker yet).
func (o *OarCLI) Close() error { return nil }

// OCRImage writes a temp image and runs oar.
func (o *OarCLI) OCRImage(ctx context.Context, page int, image []byte) ([]Line, error) {
	if o == nil || o.Bin == "" {
		return nil, fmt.Errorf("oar cli: empty binary")
	}
	ext := ".png"
	if len(image) >= 3 && image[0] == 0xff && image[1] == 0xd8 {
		ext = ".jpg"
	}
	f, err := os.CreateTemp("", "doc2md-oar-*"+ext)
	if err != nil {
		return nil, err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.Write(image); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return o.OCRFile(ctx, page, path)
}

// OCRFile runs oar on an on-disk image path.
func (o *OarCLI) OCRFile(ctx context.Context, page int, path string) ([]Line, error) {
	if o == nil || o.Bin == "" {
		return nil, fmt.Errorf("oar cli: empty binary")
	}
	ctx, cancel := context.WithTimeout(ctx, o.timeout())
	defer cancel()

	args := []string{"ocr", "--format", "json", "--size", o.sizeOrDefault(), path}
	cmd := exec.CommandContext(ctx, o.Bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("oar ocr: %s", msg)
	}
	lines, err := parseOarOCRJSON(stdout.String(), page)
	if err != nil {
		return nil, err
	}
	return lines, nil
}

// OCRFiles runs one `oar ocr` with all image paths so models load once (fairer vs warm paddle worker).
func (o *OarCLI) OCRFiles(ctx context.Context, paths []string) ([][]Line, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if len(paths) == 1 {
		lines, err := o.OCRFile(ctx, 1, paths[0])
		if err != nil {
			return nil, err
		}
		return [][]Line{lines}, nil
	}
	if o == nil || o.Bin == "" {
		return nil, fmt.Errorf("oar cli: empty binary")
	}
	// Multi-page: allow longer wall time (model load + N pages).
	to := o.timeout() * time.Duration(1+len(paths)/4)
	if to < o.timeout() {
		to = o.timeout()
	}
	ctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	args := append([]string{"ocr", "--format", "json", "--size", o.sizeOrDefault()}, paths...)
	cmd := exec.CommandContext(ctx, o.Bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("oar ocr batch: %s", msg)
	}
	pages, err := parseOarOCRJSONMulti(stdout.String(), len(paths))
	if err != nil {
		return nil, err
	}
	return pages, nil
}

// NewOarFromEnv prefers the warm oar_worker sidecar (same protocol as ocr_cli.py);
// falls back to one-shot `oar` CLI if the worker binary is missing.
func NewOarFromEnv() (Engine, error) {
	if w := resolveOarWorkerBin(); w != "" {
		return &Subprocess{Cmd: quoteArg(w), Timeout: 180 * time.Second}, nil
	}
	bin := resolveOarBin()
	if bin == "" {
		return nil, fmt.Errorf("oar not found: build tools/oar_worker, or `cargo install oar-ocr-cli`, or set DOC2MD_OCR_OAR")
	}
	return &OarCLI{Bin: bin, Timeout: 180 * time.Second, size: normalizeOarSize("")}, nil
}

// resolveOarWorkerBin finds the warm worker: DOC2MD_OCR_OAR_WORKER → tools/oar_worker → PATH.
func resolveOarWorkerBin() string {
	if p := strings.TrimSpace(os.Getenv("DOC2MD_OCR_OAR_WORKER")); p != "" && fileExists(p) {
		return p
	}
	names := []string{"oar_worker.exe", "oar_worker"}
	for _, root := range searchRoots() {
		for _, n := range names {
			for _, p := range []string{
				filepath.Join(root, n),
				filepath.Join(root, "tools", n),
				filepath.Join(root, "tools", "oar_worker", n),
				filepath.Join(root, "tools", "oar_worker", "target", "release", n),
			} {
				if fileExists(p) {
					return p
				}
			}
		}
	}
	if p, err := exec.LookPath("oar_worker"); err == nil {
		return p
	}
	if p, err := exec.LookPath("oar_worker.exe"); err == nil {
		return p
	}
	return ""
}

// ResolveOarBin finds the one-shot oar CLI (DOC2MD_OCR_OAR → tools/ → PATH).
func ResolveOarBin() string { return resolveOarBin() }

// resolveOarBin finds one-shot oar CLI: DOC2MD_OCR_OAR → tools/ → PATH / ~/.cargo/bin.
func resolveOarBin() string {
	if p := strings.TrimSpace(os.Getenv("DOC2MD_OCR_OAR")); p != "" && fileExists(p) {
		return p
	}
	names := []string{"oar.exe", "oar"}
	for _, root := range searchRoots() {
		for _, n := range names {
			for _, p := range []string{
				filepath.Join(root, n),
				filepath.Join(root, "tools", n),
				filepath.Join(root, "tools", "ocr", n),
			} {
				if fileExists(p) {
					return p
				}
			}
		}
	}
	if p, err := exec.LookPath("oar"); err == nil {
		return p
	}
	if p, err := exec.LookPath("oar.exe"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		for _, p := range []string{
			filepath.Join(home, ".cargo", "bin", "oar.exe"),
			filepath.Join(home, ".cargo", "bin", "oar"),
		} {
			if fileExists(p) {
				return p
			}
		}
	}
	return ""
}

// parseOarOCRJSON maps `oar ocr --format json` output to []Line.
// Single image → object; multi-image → array (first page only — use parseOarOCRJSONMulti for batches).
func parseOarOCRJSON(raw string, page int) ([]Line, error) {
	pages, err := parseOarOCRJSONMulti(raw, 0)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, nil
	}
	out := pages[0]
	for i := range out {
		out[i].Page = page
	}
	return out, nil
}

// parseOarOCRJSONMulti parses one or many page objects; expect>0 enforces page count.
func parseOarOCRJSONMulti(raw string, expect int) ([][]Line, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("oar ocr: empty stdout")
	}
	if i := strings.IndexAny(raw, "{["); i > 0 {
		raw = raw[i:]
	}
	var arr []json.RawMessage
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return nil, fmt.Errorf("oar ocr json array: %w", err)
		}
	} else {
		arr = []json.RawMessage{json.RawMessage(raw)}
	}
	if expect > 0 && len(arr) != expect {
		return nil, fmt.Errorf("oar ocr: got %d pages, want %d", len(arr), expect)
	}
	out := make([][]Line, len(arr))
	for i, rawPage := range arr {
		lines, err := linesFromOarPage(rawPage, i+1)
		if err != nil {
			return nil, fmt.Errorf("oar page %d: %w", i+1, err)
		}
		out[i] = lines
	}
	return out, nil
}

func linesFromOarPage(raw json.RawMessage, page int) ([]Line, error) {
	var obj struct {
		TextRegions []oarTextRegion `json:"text_regions"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("oar ocr json: %w", err)
	}
	out := make([]Line, 0, len(obj.TextRegions))
	for _, r := range obj.TextRegions {
		text := strings.TrimSpace(r.Text.String())
		if text == "" {
			continue
		}
		ln := Line{Text: text, Page: page}
		if x0, y0, x1, y1, ok := r.BoundingBox.rect(); ok {
			ln.X0, ln.Y0, ln.X1, ln.Y1 = x0, y0, x1, y1
			ln.HasBBox = true
		}
		out = append(out, ln)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].HasBBox && out[j].HasBBox {
			if out[i].Y0 != out[j].Y0 {
				return out[i].Y0 < out[j].Y0
			}
			return out[i].X0 < out[j].X0
		}
		return false
	})
	return out, nil
}

type oarTextRegion struct {
	BoundingBox oarBBox     `json:"bounding_box"`
	Text        oarFlexText `json:"text"`
	Confidence  *float64    `json:"confidence"`
}

// oarFlexText accepts string or null.
type oarFlexText string

func (t *oarFlexText) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*t = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*t = oarFlexText(s)
		return nil
	}
	return fmt.Errorf("oar text: unexpected %s", string(b))
}

func (t oarFlexText) String() string { return string(t) }

// oarBBox accepts {"points":[{"x","y"},...]} or nested point arrays.
type oarBBox struct {
	Points []oarPoint `json:"points"`
}

type oarPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (b *oarBBox) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	// Object with points
	var obj struct {
		Points json.RawMessage `json:"points"`
	}
	if err := json.Unmarshal(data, &obj); err == nil && len(obj.Points) > 0 {
		pts, err := parseOarPoints(obj.Points)
		if err != nil {
			return err
		}
		b.Points = pts
		return nil
	}
	// Bare array of points
	if data[0] == '[' {
		pts, err := parseOarPoints(data)
		if err != nil {
			return err
		}
		b.Points = pts
		return nil
	}
	return nil
}

func parseOarPoints(raw json.RawMessage) ([]oarPoint, error) {
	var asObj []oarPoint
	if err := json.Unmarshal(raw, &asObj); err == nil && len(asObj) > 0 {
		return asObj, nil
	}
	var asArr [][]float64
	if err := json.Unmarshal(raw, &asArr); err == nil {
		out := make([]oarPoint, 0, len(asArr))
		for _, p := range asArr {
			if len(p) >= 2 {
				out = append(out, oarPoint{X: p[0], Y: p[1]})
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("oar bbox points: %s", string(raw))
}

func (b oarBBox) rect() (x0, y0, x1, y1 float64, ok bool) {
	if len(b.Points) == 0 {
		return 0, 0, 0, 0, false
	}
	x0, y0 = math.Inf(1), math.Inf(1)
	x1, y1 = math.Inf(-1), math.Inf(-1)
	for _, p := range b.Points {
		if p.X < x0 {
			x0 = p.X
		}
		if p.Y < y0 {
			y0 = p.Y
		}
		if p.X > x1 {
			x1 = p.X
		}
		if p.Y > y1 {
			y1 = p.Y
		}
	}
	if math.IsInf(x0, 0) || math.IsInf(y0, 0) {
		return 0, 0, 0, 0, false
	}
	return x0, y0, x1, y1, true
}
