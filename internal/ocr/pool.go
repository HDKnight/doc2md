package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultWorkers returns parallel OCR worker count (env DOC2MD_OCR_WORKERS, default 1).
// 单进程 + ORT 多线程通常最快；多进程易抢核变慢。
func DefaultWorkers() int {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_OCR_WORKERS")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 1 {
			return n
		}
	}
	_ = runtime.NumCPU()
	return 1
}

// DefaultOCRDPI is the render DPI used for OCR pages (env DOC2MD_OCR_DPI, default 72).
func DefaultOCRDPI() int {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_OCR_DPI")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 72 && n <= 300 {
			return n
		}
	}
	return 72
}

// DefaultStructureDPI is PDF render DPI for oar structure / feat（env DOC2MD_OAR_STRUCTURE_DPI，默认 96）。
// 纯 CPU 下 150/144 过重；96 在速度与版面质量间更合适。
func DefaultStructureDPI() int {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_OAR_STRUCTURE_DPI")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 72 && n <= 300 {
			return n
		}
	}
	return 96
}

// ResolveStructureDPI picks DPI for oar structure：显式合理值保留；未设或 ≥150 时改用 DefaultStructureDPI。
func ResolveStructureDPI(dpi int) int {
	if dpi <= 0 || dpi >= 150 {
		return DefaultStructureDPI()
	}
	return dpi
}

// oarDeviceNote is set by OarStructureDevice (e.g. "gpu-fallback").
var oarDeviceNote string

// cudaProbeFn is the GPU hardware check; overridden in tests.
var cudaProbeFn = defaultCUDAProbe

// oarCUDAProbeFn checks whether the bundled oar binary was built with --features cuda.
var oarCUDAProbeFn = defaultOarCUDAProbe

var (
	cudaProbeOnce    sync.Once
	cudaProbeOK      bool
	oarCUDAProbeOnce sync.Once
	oarCUDAProbeOK   bool
)

// SetCUDAProbeForTest replaces the CUDA hardware probe; pass nil to restore default.
func SetCUDAProbeForTest(fn func() bool) {
	if fn == nil {
		cudaProbeFn = defaultCUDAProbe
	} else {
		cudaProbeFn = fn
	}
	cudaProbeOnce = sync.Once{}
	cudaProbeOK = false
	oarDeviceNote = ""
}

// SetOarCUDAProbeForTest replaces the oar-binary CUDA-feature probe; pass nil to restore.
func SetOarCUDAProbeForTest(fn func() bool) {
	if fn == nil {
		oarCUDAProbeFn = defaultOarCUDAProbe
	} else {
		oarCUDAProbeFn = fn
	}
	oarCUDAProbeOnce = sync.Once{}
	oarCUDAProbeOK = false
	oarDeviceNote = ""
}

// OarDeviceNote returns a side note from the last OarStructureDevice call
// (e.g. "gpu-fallback"). Empty if none.
func OarDeviceNote() string { return oarDeviceNote }

func cudaAvailable() bool {
	cudaProbeOnce.Do(func() {
		cudaProbeOK = cudaProbeFn()
	})
	return cudaProbeOK
}

func oarSupportsCUDA() bool {
	oarCUDAProbeOnce.Do(func() {
		oarCUDAProbeOK = oarCUDAProbeFn()
	})
	return oarCUDAProbeOK
}

// minimalProbePDF：合法空页 PDF，让 oar 在打开文件后再校验 --device cuda（缺文件时不会报 feature）。
const minimalProbePDF = `%PDF-1.1
1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj
2 0 obj<< /Type /Pages /Kids [3 0 R] /Count 1 >>endobj
3 0 obj<< /Type /Page /Parent 2 0 R /MediaBox [0 0 3 3] >>endobj
trailer<< /Size 4 /Root 1 0 R >>
%%EOF
`

// defaultOarCUDAProbe：用 --device cuda:0 试跑；stderr 含 "CUDA support is missing" 则无。
func defaultOarCUDAProbe() bool {
	bin := resolveOarBin()
	if bin == "" {
		return false
	}
	dir, err := os.MkdirTemp("", "doc2md-oar-cuda-probe-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	pdf := filepath.Join(dir, "probe.pdf")
	if err := os.WriteFile(pdf, []byte(minimalProbePDF), 0o644); err != nil {
		return false
	}
	out := filepath.Join(dir, "out")
	_ = os.MkdirAll(out, 0o755)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "structure", pdf, "--device", "cuda:0", "--dpi", "72", "--format", "json", "-o", out, "--pages", "1")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	_ = cmd.Run()
	msg := strings.ToLower(stderr.String())
	if strings.Contains(msg, "cuda support is missing") ||
		strings.Contains(msg, "cuda is not available") ||
		strings.Contains(msg, "not compiled") ||
		(strings.Contains(msg, "cuda") && strings.Contains(msg, "missing")) {
		return false
	}
	// 无 CUDA-missing 文案：视为二进制支持 CUDA（真正跑通或其它与 feature 无关的错误）
	return true
}

func defaultCUDAProbe() bool {
	if nvidiaSMIAvailable() {
		return true
	}
	return windowsDiscreteGPU()
}

func nvidiaSMIAvailable() bool {
	candidates := []string{}
	if p, err := exec.LookPath("nvidia-smi"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates,
		filepath.Join(os.Getenv("SystemRoot"), "System32", "nvidia-smi.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, path := range candidates {
		if path == "" || !fileExists(path) {
			continue
		}
		if exec.CommandContext(ctx, path, "-L").Run() == nil {
			return true
		}
	}
	return false
}

// windowsDiscreteGPU detects NVIDIA/AMD via WMI (works when nvidia-smi is not on PATH).
func windowsDiscreteGPU() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		`(Get-CimInstance Win32_VideoController -EA SilentlyContinue).Name -join ';'`)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	return strings.Contains(s, "nvidia") ||
		strings.Contains(s, "geforce") ||
		strings.Contains(s, "quadro") ||
		strings.Contains(s, "radeon") ||
		strings.Contains(s, "rx ")
}

func deviceRequestsCUDA(v string) bool {
	for _, p := range strings.Split(strings.ToLower(v), ",") {
		if strings.HasPrefix(strings.TrimSpace(p), "cuda:") {
			return true
		}
	}
	return false
}

// OarStructureDevice returns --device for oar structure。
//
//	DOC2MD_OAR_DEVICE=
//	  未设 / auto → 有独显且 oar 编了 CUDA → cuda:0；
//	                  有独显无 CUDA → 不传 --device（交 oar auto / DirectML）；无卡 → cpu×N
//	  auto-cpu → 强制 cpu×N（跳过 GPU）
//	  gpu / oar-auto / native → 不传 --device，交给 oar 默认 auto（DirectML/CUDA）
//	  cuda:0 → 硬件或 oar 不支持则回退 cpu×N
//	  cpu,cpu / 其它显式列表 → 原样传入
//	DOC2MD_OAR_CPU_REPLICAS=N → 固定 N 路（覆盖智能值，仍生成 cpu×N）
func OarStructureDevice() string {
	oarDeviceNote = ""
	v := strings.ReplaceAll(strings.TrimSpace(os.Getenv("DOC2MD_OAR_DEVICE")), " ", "")
	switch strings.ToLower(v) {
	case "oar-auto", "native", "gpu", "directml", "dml":
		oarDeviceNote = "oar-auto"
		return "" // 不传 --device
	case "auto-cpu":
		return cpuDeviceList(SmartCPUReplicas())
	case "", "auto":
		return resolveAutoDevice()
	default:
		if deviceRequestsCUDA(v) {
			if !cudaAvailable() {
				return fallbackCPU("gpu-fallback", v)
			}
			if !oarSupportsCUDA() {
				return fallbackCPU("gpu-no-cuda", v)
			}
		}
		return v
	}
}

func resolveAutoDevice() string {
	if !cudaAvailable() {
		return cpuDeviceList(SmartCPUReplicas())
	}
	if oarSupportsCUDA() {
		oarDeviceNote = "gpu-auto"
		return "cuda:0"
	}
	// 本机常无 DirectML 版 oar（无需 CUDA Toolkit 即可编译）；交 oar 选 EP。
	oarDeviceNote = "gpu-dml"
	fmt.Fprintf(os.Stderr, "doc2md: GPU detected; oar has no CUDA — using oar --device auto (DirectML if built-in)\n")
	return ""
}

func fallbackCPU(note, requested string) string {
	oarDeviceNote = note
	fb := cpuDeviceList(SmartCPUReplicas())
	fmt.Fprintf(os.Stderr, "doc2md: DOC2MD_OAR_DEVICE=%q unavailable (%s), fallback to %q\n", requested, note, fb)
	return fb
}

// SmartCPUReplicas：按「CPU」与「可用内存」取较小值。
//
// 实测：副本过多会更慢——每路 ONNX 自己还有 intra-op 线程，路数≈NumCPU/2 时严重抢核；
// 你这台 16 逻辑核上 cpu×4（≈15s）优于 cpu×5/8（可到 30s+）。
// 因此 CPU 上限用 NumCPU/4；内存只作「别 OOM」的天花板，不是越多越好。
//
//	DOC2MD_OAR_CPU_REPLICAS  强制路数 1–8
//	DOC2MD_OAR_REPLICA_MB    每路内存估值，默认 1400
//	DOC2MD_OAR_RESERVE_MB    预留，默认 max(3072, 总内存×15%)
func SmartCPUReplicas() int {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_OAR_CPU_REPLICAS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			if n > 8 {
				n = 8
			}
			return n
		}
	}

	byCPU := replicasByCPU(runtime.NumCPU())
	byMem := replicasByMemory()
	n := byCPU
	if byMem > 0 && byMem < n {
		n = byMem
	}
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	return n
}

func replicasByCPU(ncpu int) int {
	// 小核机器少开路：每路 ORT 已占多线程，再叠副本只会抢核变慢
	switch {
	case ncpu <= 4:
		return 1
	case ncpu <= 8:
		return 2
	default:
		// 较多核：外层副本 ≈ 逻辑核/4（16→4），上限 8
		n := ncpu / 4
		if n < 2 {
			n = 2
		}
		if n > 8 {
			n = 8
		}
		return n
	}
}

func replicasByMemory() int {
	total, avail, ok := systemMemory()
	if !ok || total == 0 {
		return 0 // 未知内存时不按内存卡路数
	}
	replicaMB := envInt("DOC2MD_OAR_REPLICA_MB", 1400)
	if replicaMB < 512 {
		replicaMB = 512
	}
	reserveMB := envInt("DOC2MD_OAR_RESERVE_MB", 0)
	if reserveMB <= 0 {
		reserveMB = 3072
		pct := int(total / 1024 / 1024 * 15 / 100)
		if pct > reserveMB {
			reserveMB = pct
		}
	}
	totalMB := int(total / 1024 / 1024)
	availMB := int(avail / 1024 / 1024)

	// 主依据用「装机内存」估能开几路。Windows AvailPhys 常把 standby 算成不可用，
	// 会把 16 核机器误压成 1 路（表现为又回到 30s+）。
	budget := totalMB - reserveMB
	if budget < replicaMB {
		return 1
	}
	n := budget / replicaMB

	// 仅在可用内存真的不够撑起单路时才强制 1（严重压力）；
	// 不要用 AvailPhys 去「按路数打折」，否则 standby 一高就掉回单路。
	if availMB > 0 && availMB < replicaMB {
		return 1
	}
	return n
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func cpuDeviceList(n int) string {
	if n < 1 {
		n = 1
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "cpu"
	}
	return strings.Join(parts, ",")
}

// StructureIntraOpThreads is ORT/OMP threads per structure replica.
// DOC2MD_OAR_ORT_THREADS overrides; else max(1, min(8, NumCPU/replicas)).
// Multi-replica without capping thrash the CPU (seen: ×4@15s vs ×5/8@30s+).
func StructureIntraOpThreads(replicas int) int {
	if v := strings.TrimSpace(os.Getenv("DOC2MD_OAR_ORT_THREADS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			if n > 32 {
				n = 32
			}
			return n
		}
	}
	if replicas < 1 {
		replicas = 1
	}
	n := runtime.NumCPU() / replicas
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	return n
}

func countCPUDeviceReplicas(device string) int {
	device = strings.TrimSpace(device)
	if device == "" {
		return 1
	}
	n := 0
	for _, p := range strings.Split(device, ",") {
		if strings.EqualFold(strings.TrimSpace(p), "cpu") {
			n++
		}
	}
	if n < 1 {
		return 1
	}
	return n
}

// OarStructureEnviron returns os.Environ with ORT/OMP capped for structure replicas.
func OarStructureEnviron(device string) []string {
	replicas := countCPUDeviceReplicas(device)
	threads := strconv.Itoa(StructureIntraOpThreads(replicas))
	overrides := map[string]string{
		"OMP_NUM_THREADS":          threads,
		"ORT_INTRA_OP_NUM_THREADS": threads,
		"ORT_INTER_OP_NUM_THREADS": "1",
	}
	base := os.Environ()
	out := make([]string, 0, len(base)+len(overrides))
	for _, e := range base {
		key := e
		if i := strings.IndexByte(e, '='); i > 0 {
			key = e[:i]
		}
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
		out = append(out, e)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

// SharedPool keeps warm OCR workers across requests (avoids reloading PP-OCR each time).
type SharedPool struct {
	mu      sync.Mutex
	model   string
	useCls  bool
	workers int
	engines []Engine
}

var defaultPool = &SharedPool{}

// DefaultPool returns the process-wide OCR worker pool.
func DefaultPool() *SharedPool { return defaultPool }

// Ensure prepares workers for model/useCls (restarts if settings change).
func (p *SharedPool) Ensure(model string, useCls bool, workers int) error {
	model = NormalizeModel(model)
	if workers < 1 {
		workers = DefaultWorkers()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.engines != nil && p.model == model && p.useCls == useCls && p.workers == workers {
		return nil
	}
	p.closeLocked()
	p.model = model
	p.useCls = useCls
	p.workers = workers
	p.engines = make([]Engine, workers)
	for i := 0; i < workers; i++ {
		eng := NewFromEnv()
		if cfg, ok := eng.(ModelConfigurer); ok {
			cfg.ConfigureOCR(model, useCls)
		}
		p.engines[i] = eng
	}
	return nil
}

func (p *SharedPool) closeLocked() {
	for _, eng := range p.engines {
		if eng != nil {
			_ = eng.Close()
		}
	}
	p.engines = nil
}

// Close shuts down all warm workers.
func (p *SharedPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
	return nil
}

func (p *SharedPool) snapshot() []Engine {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Engine, len(p.engines))
	copy(out, p.engines)
	return out
}

// OCRFiles recognizes on-disk page images with the warm pool.
func (p *SharedPool) OCRFiles(ctx context.Context, paths []string) ([][]Line, error) {
	n := len(paths)
	if n == 0 {
		return nil, nil
	}
	engines := p.snapshot()
	workers := len(engines)
	if workers == 0 {
		return nil, fmt.Errorf("ocr pool not initialized")
	}
	if workers > n {
		workers = n
	}

	out := make([][]Line, n)
	runOne := func(eng Engine, page int, path string) ([]Line, error) {
		if fe, ok := eng.(FileEngine); ok {
			return fe.OCRFile(ctx, page, path)
		}
		img, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return eng.OCRImage(ctx, page, img)
	}

	if workers == 1 {
		if be, ok := engines[0].(BatchFileEngine); ok {
			return be.OCRFiles(ctx, paths)
		}
		for i, path := range paths {
			lines, err := runOne(engines[0], i+1, path)
			if err != nil {
				return nil, err
			}
			out[i] = lines
		}
		return out, nil
	}

	type job struct {
		i    int
		path string
	}
	jobs := make(chan job, n)
	for i, path := range paths {
		jobs <- job{i: i, path: path}
	}
	close(jobs)

	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(eng Engine) {
			defer wg.Done()
			for j := range jobs {
				if cancelCtx.Err() != nil {
					return
				}
				lines, err := runOne(eng, j.i+1, j.path)
				if err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("ocr page %d: %w", j.i+1, err)
						cancel()
					})
					return
				}
				out[j.i] = lines
			}
		}(engines[w])
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// OCRPagesParallel recognizes page image paths via the shared warm pool.
func OCRPagesParallel(ctx context.Context, paths []string, model string, useCls bool, workers int, _ Engine) ([][]Line, error) {
	if workers < 1 {
		workers = DefaultWorkers()
	}
	if err := DefaultPool().Ensure(model, useCls, workers); err != nil {
		return nil, err
	}
	return DefaultPool().OCRFiles(ctx, paths)
}

// minimalPNG is a 1×1 white PNG used to warm ONNX models.
var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0,
	0x00, 0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xfe, 0xd4, 0xef, 0x00, 0x00, 0x00, 0x00,
	0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// WarmDefault starts the shared pool and loads models once.
func WarmDefault(model string, useCls bool) error {
	workers := DefaultWorkers()
	if err := DefaultPool().Ensure(model, useCls, workers); err != nil {
		return err
	}
	engines := DefaultPool().snapshot()
	for i, eng := range engines {
		if _, err := eng.OCRImage(context.Background(), i+1, minimalPNG); err != nil {
			return err
		}
	}
	return nil
}
