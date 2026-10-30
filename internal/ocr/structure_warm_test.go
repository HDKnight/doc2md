package ocr

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func buildFakeStructureWorker(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "fake_struct_worker")
	out := filepath.Join(t.TempDir(), "fake_struct_worker.exe")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = src
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake worker: %v\n%s", err, b)
	}
	return out
}

func TestStructureIdleSec(t *testing.T) {
	t.Setenv("DOC2MD_OAR_STRUCTURE_IDLE_SEC", "")
	if StructureIdleSec() != 60 {
		t.Fatalf("default=%d", StructureIdleSec())
	}
	t.Setenv("DOC2MD_OAR_STRUCTURE_IDLE_SEC", "0")
	if StructureIdleSec() != 0 {
		t.Fatalf("0=%d", StructureIdleSec())
	}
	t.Setenv("DOC2MD_OAR_STRUCTURE_IDLE_SEC", "90")
	if StructureIdleSec() != 90 {
		t.Fatalf("90=%d", StructureIdleSec())
	}
}

func TestStructureWarm_RunAndIdleKill(t *testing.T) {
	bin := buildFakeStructureWorker(t)
	t.Setenv("DOC2MD_OCR_OAR_STRUCTURE_WORKER", bin)
	t.Setenv("DOC2MD_OAR_STRUCTURE_IDLE_SEC", "1")

	w := &StructureWarm{}
	defer w.Close()

	pdf := filepath.Join(t.TempDir(), "a.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out1 := filepath.Join(t.TempDir(), "o1")
	pages, err := w.RunStructure(context.Background(), pdf, out1, 96, "json")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if pages != 1 {
		t.Fatalf("pages=%d", pages)
	}
	w.mu.Lock()
	alive := w.cmd != nil
	w.mu.Unlock()
	if !alive {
		t.Fatal("worker should stay up after job")
	}

	// second job reuses process
	out2 := filepath.Join(t.TempDir(), "o2")
	if _, err := w.RunStructure(context.Background(), pdf, out2, 96, "json"); err != nil {
		t.Fatalf("second: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		dead := w.cmd == nil
		w.mu.Unlock()
		if dead {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("worker should be killed after idle")
}
