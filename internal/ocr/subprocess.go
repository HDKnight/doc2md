package ocr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Subprocess runs an external OCR CLI that accepts an image path and prints JSON or plain text.
// Compatible with RapidOCR-json style: <cmd> … <imagePath>
// Output: plain text, or JSON {"data":[{"text":"..."}]} / {"text":"..."}.
//
// When Cmd points at tools/ocr_cli.py, a long-lived --worker process is reused so
// PP-OCRv6 models stay warm across pages.
type Subprocess struct {
	Cmd     string
	Timeout time.Duration

	mu       sync.Mutex
	model    string // small|tiny for current worker
	useCls   bool
	worker   *exec.Cmd
	workerIn io.WriteCloser
	workerSc *bufio.Scanner
	workDir  string
}

// ConfigureOCR updates model settings; restarts the worker if they change.
func (s *Subprocess) ConfigureOCR(model string, useCls bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	model = NormalizeModel(model)
	if s.worker != nil && (s.model != model || s.useCls != useCls) {
		_ = s.stopWorkerLocked()
	}
	s.model = model
	s.useCls = useCls
}

// NewFromEnv builds OCR engine:
//
//	DOC2MD_OCR_CMD → 任意旁路 CLI
//	DOC2MD_OCR_ENGINE=oar → oar-ocr CLI（试验）
//	默认 → tools/ocr_cli.py(faster-paddle) → RapidOCR-json → Stub
func NewFromEnv() Engine {
	if cmd := strings.TrimSpace(os.Getenv("DOC2MD_OCR_CMD")); cmd != "" {
		return &Subprocess{Cmd: cmd, Timeout: 120 * time.Second}
	}
	engine := strings.ToLower(strings.TrimSpace(os.Getenv("DOC2MD_OCR_ENGINE")))
	if engine == "oar" || engine == "oar-ocr" || engine == "oar_ocr" {
		if o, err := NewOarFromEnv(); err == nil {
			return o
		}
		// Requested oar but missing → fall through to faster-paddle if present.
	}
	if cmd := resolveBundledOCR(); cmd != "" {
		return &Subprocess{Cmd: cmd, Timeout: 120 * time.Second}
	}
	return Stub{}
}

// NewFasterPaddleFromEnv returns the bundled faster-paddle sidecar, ignoring DOC2MD_OCR_ENGINE=oar.
func NewFasterPaddleFromEnv() Engine {
	if cmd := resolveBundledOCR(); cmd != "" {
		return &Subprocess{Cmd: cmd, Timeout: 120 * time.Second}
	}
	return Stub{}
}

// resolveBundledOCR finds OCR without env:
//  1. tools/ocr_cli.py + python（faster-paddle，优先 .venv-fpaddle）
//  2. RapidOCR-json.exe beside doc2md / tools/ocr/（兼容旧旁路）
func resolveBundledOCR() string {
	roots := searchRoots()
	cli := ""
	for _, root := range roots {
		cand := filepath.Join(root, "tools", "ocr_cli.py")
		if fileExists(cand) {
			cli = cand
			break
		}
	}
	if cli != "" {
		if py := findPythonWithFasterPaddle(); py != "" {
			return quoteArg(py) + " " + quoteArg(cli)
		}
	}

	names := []string{
		"RapidOCR-json.exe", "rapidocr-json.exe", "RapidOCR-json", "rapidocr-json",
	}
	for _, root := range roots {
		for _, n := range names {
			for _, p := range []string{
				filepath.Join(root, n),
				filepath.Join(root, "tools", n),
				filepath.Join(root, "tools", "ocr", n),
			} {
				if fileExists(p) {
					return quoteArg(p)
				}
			}
		}
	}
	if p, err := exec.LookPath("RapidOCR-json"); err == nil {
		return quoteArg(p)
	}
	if p, err := exec.LookPath("rapidocr-json"); err == nil {
		return quoteArg(p)
	}
	return ""
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
	addAncestors := func(start string) {
		p := filepath.Clean(start)
		for i := 0; i < 6; i++ {
			add(p)
			parent := filepath.Dir(p)
			if parent == p {
				break
			}
			p = parent
		}
	}
	if exe, err := os.Executable(); err == nil {
		addAncestors(filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		addAncestors(wd)
	}
	return roots
}

func findPythonWithFasterPaddle() string {
	cands := []string{}
	if p := strings.TrimSpace(os.Getenv("DOC2MD_OCR_PYTHON")); p != "" && fileExists(p) {
		cands = append(cands, p)
	}
	for _, root := range searchRoots() {
		for _, rel := range []string{
			filepath.Join(".venv-fpaddle", "Scripts", "python.exe"),
			filepath.Join(".venv-fpaddle", "bin", "python"),
			filepath.Join(".venv-fpaddle", "bin", "python3"),
		} {
			p := filepath.Join(root, rel)
			if fileExists(p) {
				cands = append(cands, p)
			}
		}
	}
	for _, n := range []string{"python", "python3", "py"} {
		if p, err := exec.LookPath(n); err == nil {
			cands = append(cands, p)
		}
	}
	for _, p := range []string{
		`D:\Python314\python.exe`,
		`C:\Python314\python.exe`,
		`C:\Python312\python.exe`,
		`C:\Python311\python.exe`,
	} {
		if fileExists(p) {
			cands = append(cands, p)
		}
	}
	seen := map[string]struct{}{}
	for _, py := range cands {
		py = filepath.Clean(py)
		if _, ok := seen[py]; ok {
			continue
		}
		seen[py] = struct{}{}
		cmd := exec.Command(py, "-c", "import faster_paddle")
		if err := cmd.Run(); err == nil {
			return py
		}
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func quoteArg(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsAny(s, " \t\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func (s *Subprocess) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopWorkerLocked()
}

func (s *Subprocess) stopWorkerLocked() error {
	if s.workerIn != nil {
		_, _ = io.WriteString(s.workerIn, "EXIT\n")
		_ = s.workerIn.Close()
		s.workerIn = nil
	}
	if s.worker != nil {
		done := make(chan struct{})
		go func(cmd *exec.Cmd) {
			_ = cmd.Wait()
			close(done)
		}(s.worker)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			if s.worker.Process != nil {
				_ = s.worker.Process.Kill()
			}
			<-done
		}
		s.worker = nil
	}
	s.workerSc = nil
	if s.workDir != "" {
		_ = os.RemoveAll(s.workDir)
		s.workDir = ""
	}
	return nil
}

func usesOcrCli(cmd string) bool {
	c := strings.ToLower(cmd)
	return strings.Contains(c, "ocr_cli.py") || strings.Contains(c, "oar_worker")
}

func ocrWorkerEnv(model string, useCls bool) []string {
	threads := strings.TrimSpace(os.Getenv("DOC2MD_OCR_THREADS"))
	if threads == "" {
		n := runtime.NumCPU()
		if n > 8 {
			n = 8
		}
		if n < 2 {
			n = 2
		}
		threads = strconv.Itoa(n)
	}
	cls := "0"
	if useCls {
		cls = "1"
	}
	overrides := map[string]string{
		"PYTHONIOENCODING":         "utf-8",
		"PYTHONUTF8":               "1",
		"DOC2MD_OCR_MODEL":         model,
		"DOC2MD_OCR_USE_CLS":       cls,
		"DOC2MD_OCR_THREADS":       threads,
		"OMP_NUM_THREADS":          threads,
		"ORT_INTRA_OP_NUM_THREADS": threads,
		"ORT_INTER_OP_NUM_THREADS": "1",
	}
	base := os.Environ()
	out := make([]string, 0, len(base)+len(overrides))
	seen := map[string]bool{}
	for _, e := range base {
		key := e
		if i := strings.IndexByte(e, '='); i > 0 {
			key = e[:i]
		}
		uk := strings.ToUpper(key)
		if _, ok := overrides[key]; ok {
			continue
		}
		// Windows env keys are case-insensitive
		skip := false
		for ok := range overrides {
			if strings.EqualFold(ok, key) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		_ = uk
		out = append(out, e)
		seen[strings.ToUpper(key)] = true
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

func (s *Subprocess) OCRImage(ctx context.Context, page int, image []byte) ([]Line, error) {
	if s.Cmd == "" {
		return nil, fmt.Errorf("ocr subprocess cmd empty")
	}
	if usesOcrCli(s.Cmd) {
		return s.ocrViaWorker(ctx, page, image)
	}
	return s.ocrOneShot(ctx, page, image)
}

// OCRFile runs OCR on an existing image path (preferred for warm worker).
func (s *Subprocess) OCRFile(ctx context.Context, page int, path string) ([]Line, error) {
	if s.Cmd == "" {
		return nil, fmt.Errorf("ocr subprocess cmd empty")
	}
	if usesOcrCli(s.Cmd) {
		return s.ocrViaWorkerPath(ctx, page, path)
	}
	img, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return s.ocrOneShot(ctx, page, img)
}

// OCRFiles runs batch OCR via worker BATCH protocol (faster-paddle ocr_batch).
// Large docs are chunked to avoid worker OOM / crash ("ocr worker closed").
func (s *Subprocess) OCRFiles(ctx context.Context, paths []string) ([][]Line, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if s.Cmd == "" {
		return nil, fmt.Errorf("ocr subprocess cmd empty")
	}
	if !usesOcrCli(s.Cmd) {
		out := make([][]Line, len(paths))
		for i, p := range paths {
			lines, err := s.OCRFile(ctx, i+1, p)
			if err != nil {
				return nil, err
			}
			out[i] = lines
		}
		return out, nil
	}

	chunk := batchChunkSize()
	out := make([][]Line, 0, len(paths))
	for start := 0; start < len(paths); start += chunk {
		end := start + chunk
		if end > len(paths) {
			end = len(paths)
		}
		part, err := s.ocrBatchChunk(ctx, paths[start:end])
		if err != nil {
			// one restart+retry for crashed worker
			_ = s.Close()
			part, err = s.ocrBatchChunk(ctx, paths[start:end])
			if err != nil {
				return nil, fmt.Errorf("ocr pages %d-%d: %w", start+1, end, err)
			}
		}
		for i := range part {
			page := start + i + 1
			for j := range part[i] {
				part[i][j].Page = page
			}
		}
		out = append(out, part...)
	}
	return out, nil
}

func batchChunkSize() int {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_OCR_BATCH")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 1 && n <= 32 {
			return n
		}
	}
	return 4
}

func (s *Subprocess) ocrBatchChunk(ctx context.Context, paths []string) ([][]Line, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	if d := time.Duration(len(paths)) * 20 * time.Second; d > timeout {
		timeout = d
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureWorkerLocked(ctx); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(s.workerIn, "BATCH "+string(payload)+"\n"); err != nil {
		_ = s.stopWorkerLocked()
		return nil, fmt.Errorf("ocr worker write: %w", err)
	}

	type result struct {
		line string
		ok   bool
	}
	ch := make(chan result, 1)
	go func() {
		ok := s.workerSc.Scan()
		if ok {
			ch <- result{line: s.workerSc.Text(), ok: true}
			return
		}
		ch <- result{ok: false}
	}()

	select {
	case <-ctx.Done():
		_ = s.stopWorkerLocked()
		return nil, ctx.Err()
	case r := <-ch:
		if !r.ok {
			err := s.workerSc.Err()
			_ = s.stopWorkerLocked()
			if err != nil {
				return nil, fmt.Errorf("ocr worker read: %w", err)
			}
			return nil, fmt.Errorf("ocr worker closed")
		}
		return parseOCRBatchJSON(r.line, len(paths))
	}
}

func (s *Subprocess) ensureWorkerLocked(ctx context.Context) error {
	if s.worker != nil && s.workerIn != nil && s.workerSc != nil {
		return nil
	}
	_ = s.stopWorkerLocked()

	parts := splitCommand(s.Cmd)
	if len(parts) == 0 {
		return fmt.Errorf("ocr subprocess cmd empty")
	}
	args := append(append([]string{}, parts[1:]...), "--worker")
	cmd := exec.CommandContext(context.Background(), parts[0], args...)
	model := s.model
	if model == "" {
		model = NormalizeModel(os.Getenv("DOC2MD_OCR_MODEL"))
		s.model = model
	}
	cmd.Env = ocrWorkerEnv(model, s.useCls)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	sc := bufio.NewScanner(stdout)
	// 多页 BATCH JSON 可能很大（题库页）
	sc.Buffer(make([]byte, 0, 256*1024), 64*1024*1024)

	dir, err := os.MkdirTemp("", "doc2md-ocr-work-*")
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	s.worker = cmd
	s.workerIn = stdin
	s.workerSc = sc
	s.workDir = dir
	_ = ctx
	return nil
}

func (s *Subprocess) ocrViaWorker(ctx context.Context, page int, image []byte) ([]Line, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureWorkerLocked(ctx); err != nil {
		return nil, err
	}

	ext := ".png"
	if len(image) >= 2 && image[0] == 0xff && image[1] == 0xd8 {
		ext = ".jpg"
	}
	imgPath := filepath.Join(s.workDir, fmt.Sprintf("page-%d-%d%s", page, time.Now().UnixNano(), ext))
	if err := os.WriteFile(imgPath, image, 0o600); err != nil {
		return nil, err
	}
	defer os.Remove(imgPath)

	return s.ocrWorkerExchangeLocked(ctx, page, imgPath)
}

func (s *Subprocess) ocrViaWorkerPath(ctx context.Context, page int, path string) ([]Line, error) {
	lines, err := s.ocrViaWorkerPathOnce(ctx, page, path)
	if err != nil && (strings.Contains(err.Error(), "worker closed") || strings.Contains(err.Error(), "worker read")) {
		_ = s.Close()
		return s.ocrViaWorkerPathOnce(ctx, page, path)
	}
	return lines, err
}

func (s *Subprocess) ocrViaWorkerPathOnce(ctx context.Context, page int, path string) ([]Line, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureWorkerLocked(ctx); err != nil {
		return nil, err
	}
	return s.ocrWorkerExchangeLocked(ctx, page, path)
}

// ocrWorkerExchangeLocked requires s.mu held.
func (s *Subprocess) ocrWorkerExchangeLocked(ctx context.Context, page int, imgPath string) ([]Line, error) {
	if _, err := io.WriteString(s.workerIn, imgPath+"\n"); err != nil {
		_ = s.stopWorkerLocked()
		return nil, fmt.Errorf("ocr worker write: %w", err)
	}

	type result struct {
		line string
		ok   bool
	}
	ch := make(chan result, 1)
	go func() {
		ok := s.workerSc.Scan()
		if ok {
			ch <- result{line: s.workerSc.Text(), ok: true}
			return
		}
		ch <- result{ok: false}
	}()

	select {
	case <-ctx.Done():
		_ = s.stopWorkerLocked()
		return nil, ctx.Err()
	case r := <-ch:
		if !r.ok {
			err := s.workerSc.Err()
			_ = s.stopWorkerLocked()
			if err != nil {
				return nil, fmt.Errorf("ocr worker read: %w", err)
			}
			return nil, fmt.Errorf("ocr worker closed")
		}
		text := strings.TrimSpace(r.line)
		if lines, ok := parseOCRJSON(text, page); ok {
			return lines, nil
		}
		if text == "" {
			return nil, nil
		}
		return []Line{{Text: text, Page: page}}, nil
	}
}

func (s *Subprocess) ocrOneShot(ctx context.Context, page int, image []byte) ([]Line, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.mu.Lock()
	model := s.model
	useClsOn := s.useCls
	s.mu.Unlock()
	if model == "" {
		model = NormalizeModel(os.Getenv("DOC2MD_OCR_MODEL"))
	}

	dir, err := os.MkdirTemp("", "doc2md-ocr-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	imgPath := filepath.Join(dir, fmt.Sprintf("page-%d.png", page))
	if err := os.WriteFile(imgPath, image, 0o600); err != nil {
		return nil, err
	}

	parts := splitCommand(s.Cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("ocr subprocess cmd empty")
	}
	args := append(append([]string{}, parts[1:]...), imgPath)
	cmd := exec.CommandContext(ctx, parts[0], args...)
	cmd.Env = ocrWorkerEnv(model, useClsOn)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ocr exec: %w: %s", err, strings.TrimSpace(decodeOCRBytes(stderr.Bytes())))
	}
	text := strings.TrimSpace(decodeOCRBytes(stdout.Bytes()))
	if lines, ok := parseOCRJSON(text, page); ok {
		return lines, nil
	}
	if i := strings.Index(text, "{"); i >= 0 {
		if lines, ok := parseOCRJSON(text[i:], page); ok {
			return lines, nil
		}
	}
	if text == "" {
		return nil, nil
	}
	return []Line{{Text: text, Page: page}}, nil
}

func splitCommand(s string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// decodeOCRBytes 优先 UTF-8；若大量替换符或非法序列，再试 GBK（Windows 控制台乱码回退）。
func decodeOCRBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) && countRune(b, '\uFFFD') == 0 {
		s := string(b)
		if looksMostlyCJKOrASCII(s) || !looksLikeMojibake(s) {
			return s
		}
	}
	if gbk, err := decodeGBK(b); err == nil && gbk != "" && countRune([]byte(gbk), '\uFFFD') == 0 {
		if looksMostlyCJKOrASCII(gbk) || utf8.RuneCountInString(gbk) > countRune(b, '\uFFFD') {
			return gbk
		}
	}
	if utf8.Valid(b) {
		return string(b)
	}
	return string(b)
}

func decodeGBK(b []byte) (string, error) {
	r := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GBK.NewDecoder())
	out, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func countRune(b []byte, want rune) int {
	n := 0
	for _, r := range string(b) {
		if r == want {
			n++
		}
	}
	return n
}

func looksMostlyCJKOrASCII(s string) bool {
	cjk, other := 0, 0
	for _, r := range s {
		switch {
		case r <= 0x7f, r >= 0x4e00 && r <= 0x9fff, r >= 0x3000 && r <= 0x303f:
			cjk++
		case r == '\uFFFD':
			other += 3
		default:
			if r > 0xff {
				other++
			}
		}
	}
	return cjk >= other
}

func looksLikeMojibake(s string) bool {
	n := 0
	for _, r := range s {
		if r == '\uFFFD' {
			n++
		}
	}
	return n > 8 || (len([]rune(s)) > 20 && float64(n)/float64(len([]rune(s))) > 0.15)
}

type ocrDataItem struct {
	Text string   `json:"text"`
	X0   *float64 `json:"x0"`
	Y0   *float64 `json:"y0"`
	X1   *float64 `json:"x1"`
	Y1   *float64 `json:"y1"`
}

func lineFromDataItem(it ocrDataItem, page int) (Line, bool) {
	t := strings.TrimSpace(it.Text)
	if t == "" {
		return Line{}, false
	}
	ln := Line{Text: t, Page: page}
	if it.X0 != nil && it.Y0 != nil && it.X1 != nil && it.Y1 != nil {
		ln.X0, ln.Y0, ln.X1, ln.Y1 = *it.X0, *it.Y0, *it.X1, *it.Y1
		ln.HasBBox = true
	}
	return ln, true
}

func parseOCRJSON(raw string, page int) ([]Line, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[") {
		return nil, false
	}
	if i := strings.LastIndex(raw, "}"); i > 0 {
		raw = raw[:i+1]
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		if data, ok := obj["data"]; ok {
			var items []ocrDataItem
			if json.Unmarshal(data, &items) == nil && len(items) > 0 {
				out := make([]Line, 0, len(items))
				for _, it := range items {
					if ln, ok := lineFromDataItem(it, page); ok {
						out = append(out, ln)
					}
				}
				return out, true
			}
			if json.Unmarshal(data, &items) == nil {
				return nil, true
			}
		}
		if tRaw, ok := obj["text"]; ok {
			var t string
			if json.Unmarshal(tRaw, &t) == nil && strings.TrimSpace(t) != "" {
				return []Line{{Text: strings.TrimSpace(t), Page: page}}, true
			}
		}
	}
	return nil, false
}

func parseOCRBatchJSON(raw string, expect int) ([][]Line, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.LastIndex(raw, "}"); i > 0 {
		raw = raw[:i+1]
	}
	var obj struct {
		Pages []struct {
			Data []ocrDataItem `json:"data"`
		} `json:"pages"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, fmt.Errorf("ocr batch json: %w", err)
	}
	if obj.Error != "" {
		return nil, fmt.Errorf("ocr batch: %s", obj.Error)
	}
	if expect > 0 && len(obj.Pages) != expect {
		return nil, fmt.Errorf("ocr batch: got %d pages, want %d", len(obj.Pages), expect)
	}
	out := make([][]Line, len(obj.Pages))
	for i, p := range obj.Pages {
		lines := make([]Line, 0, len(p.Data))
		for _, it := range p.Data {
			if ln, ok := lineFromDataItem(it, i+1); ok {
				lines = append(lines, ln)
			}
		}
		out[i] = lines
	}
	return out, nil
}
