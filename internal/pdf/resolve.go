package pdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// popplerBin resolves pdftoppm / pdftotext: env → 旁路 tools/poppler → PATH.
func popplerBin(envKey, name string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if p := findLocalPoppler(name); p != "" {
		return p
	}
	return name
}

func findLocalPoppler(name string) string {
	exe := name
	if !strings.HasSuffix(strings.ToLower(exe), ".exe") {
		exe = name + ".exe"
	}
	for _, root := range searchRoots() {
		for _, rel := range []string{
			filepath.Join("tools", "poppler", "Library", "bin", exe),
			filepath.Join("tools", "poppler", "bin", exe),
			filepath.Join("poppler", "Library", "bin", exe),
		} {
			p := filepath.Join(root, rel)
			if fileExists(p) {
				return p
			}
		}
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

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// lookPoppler ensures the binary is resolvable (absolute path or on PATH).
func lookPoppler(bin string) error {
	_, err := exec.LookPath(bin)
	return err
}
