package ocr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StructureIdleSec returns idle seconds before killing the warm structure worker.
// DOC2MD_OAR_STRUCTURE_IDLE_SEC: default 60; 0 = never auto-kill.
func StructureIdleSec() int {
	v := strings.TrimSpace(os.Getenv("DOC2MD_OAR_STRUCTURE_IDLE_SEC"))
	if v == "" {
		return 60
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 60
	}
	return n
}

// ResolveStructureWorkerBin finds oar_structure_worker.
func ResolveStructureWorkerBin() string {
	if p := strings.TrimSpace(os.Getenv("DOC2MD_OCR_OAR_STRUCTURE_WORKER")); p != "" && fileExists(p) {
		return p
	}
	names := []string{"oar_structure_worker.exe", "oar_structure_worker"}
	for _, root := range searchRoots() {
		for _, n := range names {
			for _, p := range []string{
				filepath.Join(root, "tools", n),
				filepath.Join(root, "tools", "oar_structure_worker", n),
				filepath.Join(root, "tools", "oar_structure_worker", "target", "release", n),
			} {
				if fileExists(p) {
					return p
				}
			}
		}
	}
	if p, err := exec.LookPath("oar_structure_worker"); err == nil {
		return p
	}
	if p, err := exec.LookPath("oar_structure_worker.exe"); err == nil {
		return p
	}
	return ""
}

type structureWarmReply struct {
	OK    bool   `json:"ok"`
	Pages int    `json:"pages"`
	MS    int64  `json:"ms"`
	Error string `json:"error"`
}

// StructureWarm holds one long-lived oar_structure_worker; idle → Kill.
type StructureWarm struct {
	mu       sync.Mutex
	bin      string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	sc       *bufio.Scanner
	lastUsed time.Time
	idleSec  int
	watchOnce sync.Once
}

var defaultStructureWarm = &StructureWarm{}

// DefaultStructureWarm is the process-wide warm structure sidecar.
func DefaultStructureWarm() *StructureWarm { return defaultStructureWarm }

// Available reports whether a structure worker binary exists.
func (w *StructureWarm) Available() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.bin == "" {
		w.bin = ResolveStructureWorkerBin()
	}
	return w.bin != ""
}

// RunStructure sends one PDF job to the warm worker (starts it if needed).
// pageNums nil/empty = all pages；建议由调用方按小批次传入以降内存峰值。
// outDir must be empty or exclusive for this job (worker create_new style via fresh dir).
func (w *StructureWarm) RunStructure(ctx context.Context, pdfPath, outDir string, dpi int, format string) (pages int, err error) {
	return w.RunStructurePages(ctx, pdfPath, outDir, dpi, format, nil)
}

// RunStructurePages 同 RunStructure，可指定 1-based 页码列表。
func (w *StructureWarm) RunStructurePages(ctx context.Context, pdfPath, outDir string, dpi int, format string, pageNums []int) (pages int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.bin == "" {
		w.bin = ResolveStructureWorkerBin()
	}
	if w.bin == "" {
		return 0, fmt.Errorf("oar_structure_worker not found")
	}
	w.idleSec = StructureIdleSec()
	w.watchOnce.Do(func() { go w.idleLoop() })

	if err := w.ensureLocked(ctx); err != nil {
		return 0, err
	}

	if format != "json" && format != "markdown" {
		format = "json"
	}
	if dpi <= 0 {
		dpi = DefaultStructureDPI()
	}
	payload := map[string]any{
		"op":     "structure",
		"pdf":    pdfPath,
		"out":    outDir,
		"dpi":    float64(dpi),
		"format": format,
	}
	if len(pageNums) > 0 {
		payload["pages"] = pageNums
	}
	req, _ := json.Marshal(payload)
	req = append(req, '\n')

	type result struct {
		pages int
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		if _, e := w.stdin.Write(req); e != nil {
			ch <- result{err: fmt.Errorf("structure worker write: %w", e)}
			return
		}
		if !w.sc.Scan() {
			err := w.sc.Err()
			if err == nil {
				err = io.EOF
			}
			ch <- result{err: fmt.Errorf("structure worker read: %w", err)}
			return
		}
		var rep structureWarmReply
		if e := json.Unmarshal(w.sc.Bytes(), &rep); e != nil {
			ch <- result{err: fmt.Errorf("structure worker bad reply: %w (%s)", e, w.sc.Text())}
			return
		}
		if !rep.OK {
			msg := rep.Error
			if msg == "" {
				msg = "structure failed"
			}
			ch <- result{err: fmt.Errorf("structure worker: %s", msg)}
			return
		}
		ch <- result{pages: rep.Pages}
	}()

	select {
	case <-ctx.Done():
		_ = w.killLocked()
		return 0, ctx.Err()
	case r := <-ch:
		w.lastUsed = time.Now()
		if r.err != nil {
			_ = w.killLocked()
			return 0, r.err
		}
		return r.pages, nil
	}
}

// Close kills the warm worker if running.
func (w *StructureWarm) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.killLocked()
}

func (w *StructureWarm) ensureLocked(ctx context.Context) error {
	if w.cmd != nil && w.cmd.Process != nil {
		return nil
	}
	cmd := exec.CommandContext(context.Background(), w.bin, "--worker")
	cmd.Env = OarStructureEnviron("")
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
		return fmt.Errorf("start structure worker: %w", err)
	}
	sc := bufio.NewScanner(stdout)
	// structure replies are small; pages can be large jobs but reply is one line
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 4*1024*1024)

	w.cmd = cmd
	w.stdin = stdin
	w.sc = sc
	w.lastUsed = time.Now()

	// Wait until models load: first stderr "ready" is async; block on first ping?
	// Worker only answers after load, before reading stdin — Start returns after spawn.
	// First real job pays load cost inside RunStructure while waiting for reply.
	_ = ctx
	return nil
}

func (w *StructureWarm) killLocked() error {
	if w.stdin != nil {
		_, _ = io.WriteString(w.stdin, "EXIT\n")
		_ = w.stdin.Close()
		w.stdin = nil
	}
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
		_, _ = w.cmd.Process.Wait()
	}
	w.cmd = nil
	w.sc = nil
	return nil
}

func (w *StructureWarm) idleLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		w.mu.Lock()
		idle := w.idleSec
		if idle <= 0 || w.cmd == nil {
			w.mu.Unlock()
			continue
		}
		if time.Since(w.lastUsed) >= time.Duration(idle)*time.Second {
			fmt.Fprintf(os.Stderr, "doc2md: structure worker idle %ds — killing to free VRAM/RAM\n", idle)
			_ = w.killLocked()
		}
		w.mu.Unlock()
	}
}
