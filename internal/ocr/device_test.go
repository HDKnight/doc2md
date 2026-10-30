package ocr

import (
	"runtime"
	"strings"
	"testing"
)

func TestSmartCPUReplicas_Bounds(t *testing.T) {
	t.Setenv("DOC2MD_OAR_CPU_REPLICAS", "")
	n := SmartCPUReplicas()
	if n < 1 || n > 8 {
		t.Fatalf("smart replicas out of range: %d (NumCPU=%d)", n, runtime.NumCPU())
	}
}

func TestReplicasByCPU(t *testing.T) {
	if replicasByCPU(2) != 1 || replicasByCPU(4) != 1 {
		t.Fatalf("≤4 cpu → 1, got 2=%d 4=%d", replicasByCPU(2), replicasByCPU(4))
	}
	if replicasByCPU(8) != 2 {
		t.Fatalf("8 cpu → 2, got %d", replicasByCPU(8))
	}
	if replicasByCPU(16) != 4 {
		t.Fatalf("16 cpu → 4, got %d", replicasByCPU(16))
	}
}

func TestOarStructureDevice_AutoCPU(t *testing.T) {
	t.Cleanup(func() { SetCUDAProbeForTest(nil) })
	SetCUDAProbeForTest(func() bool { return false })
	t.Setenv("DOC2MD_OAR_DEVICE", "auto-cpu")
	t.Setenv("DOC2MD_OAR_CPU_REPLICAS", "")
	dev := OarStructureDevice()
	if !strings.HasPrefix(dev, "cpu") {
		t.Fatalf("auto-cpu device: %q", dev)
	}
	parts := strings.Split(dev, ",")
	if len(parts) != SmartCPUReplicas() {
		t.Fatalf("parts=%d want %d (%q)", len(parts), SmartCPUReplicas(), dev)
	}
}

func TestOarStructureDevice_ExplicitGPUAuto(t *testing.T) {
	t.Setenv("DOC2MD_OAR_DEVICE", "gpu")
	if got := OarStructureDevice(); got != "" {
		t.Fatalf("gpu should omit --device, got %q", got)
	}
	if note := OarDeviceNote(); note != "oar-auto" {
		t.Fatalf("note=%q", note)
	}
}

func TestOarStructureDevice_AutoGPUNoCUDABinary(t *testing.T) {
	t.Cleanup(func() {
		SetCUDAProbeForTest(nil)
		SetOarCUDAProbeForTest(nil)
	})
	SetCUDAProbeForTest(func() bool { return true })
	SetOarCUDAProbeForTest(func() bool { return false })
	t.Setenv("DOC2MD_OAR_DEVICE", "auto")
	t.Setenv("DOC2MD_OAR_CPU_REPLICAS", "2")
	if got := OarStructureDevice(); got != "" {
		t.Fatalf("GPU present but oar no CUDA → oar-auto (DirectML), got %q", got)
	}
	if note := OarDeviceNote(); note != "gpu-dml" {
		t.Fatalf("note=%q", note)
	}
}

func TestOarStructureDevice_AutoUsesCUDA(t *testing.T) {
	t.Cleanup(func() {
		SetCUDAProbeForTest(nil)
		SetOarCUDAProbeForTest(nil)
	})
	SetCUDAProbeForTest(func() bool { return true })
	SetOarCUDAProbeForTest(func() bool { return true })
	t.Setenv("DOC2MD_OAR_DEVICE", "auto")
	if got := OarStructureDevice(); got != "cuda:0" {
		t.Fatalf("auto+GPU+cuda oar → cuda:0, got %q", got)
	}
	if note := OarDeviceNote(); note != "gpu-auto" {
		t.Fatalf("note=%q", note)
	}
}

func TestOarStructureDevice_Explicit(t *testing.T) {
	t.Cleanup(func() {
		SetCUDAProbeForTest(nil)
		SetOarCUDAProbeForTest(nil)
	})
	SetCUDAProbeForTest(func() bool { return true })
	SetOarCUDAProbeForTest(func() bool { return true })
	t.Setenv("DOC2MD_OAR_DEVICE", "cuda:0")
	if got := OarStructureDevice(); got != "cuda:0" {
		t.Fatalf("got %q", got)
	}
	if note := OarDeviceNote(); note != "" {
		t.Fatalf("note=%q", note)
	}
	t.Setenv("DOC2MD_OAR_DEVICE", "oar-auto")
	if got := OarStructureDevice(); got != "" {
		t.Fatalf("oar-auto should omit device, got %q", got)
	}
}

func TestOarStructureDevice_CUDAFallback(t *testing.T) {
	t.Cleanup(func() {
		SetCUDAProbeForTest(nil)
		SetOarCUDAProbeForTest(nil)
	})
	SetCUDAProbeForTest(func() bool { return false })
	SetOarCUDAProbeForTest(func() bool { return true })
	t.Setenv("DOC2MD_OAR_DEVICE", "cuda:0")
	t.Setenv("DOC2MD_OAR_CPU_REPLICAS", "2")
	got := OarStructureDevice()
	if got != "cpu,cpu" {
		t.Fatalf("cuda unavailable should fallback, got %q", got)
	}
	if note := OarDeviceNote(); note != "gpu-fallback" {
		t.Fatalf("note=%q", note)
	}
}

func TestOarStructureDevice_CUDABinaryMissing(t *testing.T) {
	t.Cleanup(func() {
		SetCUDAProbeForTest(nil)
		SetOarCUDAProbeForTest(nil)
	})
	SetCUDAProbeForTest(func() bool { return true })
	SetOarCUDAProbeForTest(func() bool { return false })
	t.Setenv("DOC2MD_OAR_DEVICE", "cuda:0")
	t.Setenv("DOC2MD_OAR_CPU_REPLICAS", "2")
	got := OarStructureDevice()
	if got != "cpu,cpu" {
		t.Fatalf("oar without CUDA should fallback, got %q", got)
	}
	if note := OarDeviceNote(); note != "gpu-no-cuda" {
		t.Fatalf("note=%q", note)
	}
}

func TestOarStructureDevice_ForceReplicas(t *testing.T) {
	t.Setenv("DOC2MD_OAR_DEVICE", "auto-cpu")
	t.Setenv("DOC2MD_OAR_CPU_REPLICAS", "3")
	if got := OarStructureDevice(); got != "cpu,cpu,cpu" {
		t.Fatalf("got %q", got)
	}
}

func TestStructureIntraOpThreads(t *testing.T) {
	t.Setenv("DOC2MD_OAR_ORT_THREADS", "")
	ncpu := runtime.NumCPU()
	got := StructureIntraOpThreads(4)
	want := ncpu / 4
	if want < 1 {
		want = 1
	}
	if want > 8 {
		want = 8
	}
	if got != want {
		t.Fatalf("replicas=4 got %d want %d (ncpu=%d)", got, want, ncpu)
	}
	t.Setenv("DOC2MD_OAR_ORT_THREADS", "3")
	if StructureIntraOpThreads(4) != 3 {
		t.Fatalf("override")
	}
}

func TestSystemMemory(t *testing.T) {
	total, avail, ok := systemMemory()
	if !ok {
		t.Skip("meminfo unavailable on this OS")
	}
	if total == 0 || avail == 0 || avail > total*2 {
		t.Fatalf("total=%d avail=%d", total, avail)
	}
}
