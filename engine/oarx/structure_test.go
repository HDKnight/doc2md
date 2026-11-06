package oarx

import (
	"os"
	"testing"
)

func TestPageBatchesAllAtOnce(t *testing.T) {
	got := pageBatches(11, 0)
	if len(got) != 1 || got[0] != nil {
		t.Fatalf("batch=0 want one nil batch, got %#v", got)
	}
	got = pageBatches(11, 100)
	if len(got) != 1 || got[0] != nil {
		t.Fatalf("batch>=pages want one nil batch, got %#v", got)
	}
	got = pageBatches(5, 2)
	if len(got) != 3 || len(got[0]) != 2 || len(got[2]) != 1 {
		t.Fatalf("batch=2: %#v", got)
	}
}

func TestPreferStructureWarm(t *testing.T) {
	t.Setenv("DOC2MD_OAR_STRUCTURE_WARM", "")
	if preferStructureWarm("cpu,cpu,cpu,cpu") {
		t.Fatal("multi-cpu should prefer CLI")
	}
	if !preferStructureWarm("cpu") {
		t.Fatal("single cpu may use warm")
	}
	if !preferStructureWarm("cuda:0") {
		t.Fatal("cuda should allow warm")
	}
	if !preferStructureWarm("") {
		t.Fatal("empty(oar-auto) should allow warm")
	}
	t.Setenv("DOC2MD_OAR_STRUCTURE_WARM", "1")
	if !preferStructureWarm("cpu,cpu,cpu,cpu") {
		t.Fatal("force warm")
	}
	t.Setenv("DOC2MD_OAR_STRUCTURE_WARM", "0")
	if preferStructureWarm("cuda:0") {
		t.Fatal("force off")
	}
}

func TestStructureUsesGPUPool(t *testing.T) {
	if !structureUsesGPUPool("") || !structureUsesGPUPool("cuda:0") {
		t.Fatal("single GPU device should use the worker pool")
	}
	if structureUsesGPUPool("cpu") || structureUsesGPUPool("cpu,cpu") || structureUsesGPUPool("cuda:0,cuda:0") {
		t.Fatal("cpu and explicit device lists stay on the CLI path")
	}
}

func TestSplitPages(t *testing.T) {
	got := splitPages(16, 4)
	if len(got) != 4 {
		t.Fatalf("groups=%d", len(got))
	}
	seen := map[int]int{}
	for _, g := range got {
		if len(g) != 4 {
			t.Fatalf("group len %d: %#v", len(g), got)
		}
		for _, p := range g {
			seen[p]++
		}
	}
	for p := 1; p <= 16; p++ {
		if seen[p] != 1 {
			t.Fatalf("page %d count %d", p, seen[p])
		}
	}
	got = splitPages(5, 4)
	if len(got) != 4 || len(got[0]) != 2 || len(got[3]) != 1 || got[3][0] != 5 {
		t.Fatalf("5/4: %#v", got)
	}
	if splitPages(0, 4) != nil {
		t.Fatal("unknown page count should not split")
	}
}

func TestStructurePageBatchSizeEnv(t *testing.T) {
	t.Setenv("DOC2MD_OAR_STRUCTURE_BATCH", "")
	if structurePageBatchSize() != 0 {
		t.Fatalf("default want 0")
	}
	t.Setenv("DOC2MD_OAR_STRUCTURE_BATCH", "2")
	if structurePageBatchSize() != 2 {
		t.Fatalf("env=2 want 2")
	}
	_ = os.Unsetenv("DOC2MD_OAR_STRUCTURE_BATCH")
}

func TestPageNumFromOarStem(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{`C:\tmp\doc2md-engine-1882345123_p1.json`, 1},
		{`C:\tmp\doc2md-engine-1882345123_p2.json`, 2},
		{`C:\tmp\doc2md-engine-1882345123_p10.json`, 10},
		{`C:\tmp\doc2md-engine-1882345123_p11.json`, 11},
		{`《复习题》_p10.json`, 10},
		{`no_page_marker.json`, 0},
	}
	for _, c := range cases {
		if got := pageNum(c.path); got != c.want {
			t.Fatalf("pageNum(%q)=%d want %d", c.path, got, c.want)
		}
	}
}

func TestPageNumSortOrder(t *testing.T) {
	files := []string{
		`out/doc2md-engine-9_p10.json`,
		`out/doc2md-engine-9_p2.json`,
		`out/doc2md-engine-9_p1.json`,
		`out/doc2md-engine-9_p11.json`,
	}
	// mimic readJSONPages sort
	sorted := append([]string(nil), files...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if pageNum(sorted[j]) < pageNum(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	want := []int{1, 2, 10, 11}
	for i, f := range sorted {
		if pageNum(f) != want[i] {
			t.Fatalf("sorted[%d]=%s page=%d want %d", i, f, pageNum(f), want[i])
		}
	}
}
