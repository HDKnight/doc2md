// Fake structure worker for Go tests: no models, writes empty page JSON.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "--worker" && os.Args[1] != "-w") {
		fmt.Fprintln(os.Stderr, "usage: fake_struct_worker --worker")
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "[fake_struct_worker] ready")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		if strings.EqualFold(raw, "EXIT") || strings.EqualFold(raw, "QUIT") {
			return
		}
		var req struct {
			Op     string  `json:"op"`
			PDF    string  `json:"pdf"`
			Out    string  `json:"out"`
			DPI    float64 `json:"dpi"`
			Format string  `json:"format"`
		}
		if err := json.Unmarshal([]byte(raw), &req); err != nil {
			enc(map[string]any{"ok": false, "error": err.Error()})
			continue
		}
		_ = os.MkdirAll(req.Out, 0o755)
		stem := strings.TrimSuffix(filepath.Base(req.PDF), filepath.Ext(req.PDF))
		if stem == "" {
			stem = "page"
		}
		name := filepath.Join(req.Out, stem+"_p1.json")
		_ = os.WriteFile(name, []byte(`{"version":1,"pipeline":"classic","page":{"index":0,"width":10,"height":10},"blocks":[],"markdown":""}`), 0o644)
		time.Sleep(20 * time.Millisecond)
		enc(map[string]any{"ok": true, "pages": 1, "ms": 20})
	}
}

func enc(v any) {
	b, _ := json.Marshal(v)
	os.Stdout.Write(append(b, '\n'))
}
