package indra

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// PreferredHFRevision is the HF model tag with rec_cjk (Chinese).
// Package default v1.1.0 lacks CJK routing and truncates names like 李大钊→李大.
const PreferredHFRevision = "v3.1.1"

var (
	modelOnce sync.Once
	modelDir  string
	modelTag  string
	modelErr  error
)

// ResolveModelDir returns a local folder with indra ONNX weights.
// Order: DOC2MD_INDRA_MODEL_DIR → INDRA_OCR_MODEL_DIR → download HF PreferredHFRevision (cached).
func ResolveModelDir() (string, error) {
	for _, key := range []string{"DOC2MD_INDRA_MODEL_DIR", "INDRA_OCR_MODEL_DIR"} {
		if p := strings.TrimSpace(os.Getenv(key)); p != "" {
			if ok, _ := modelDirOK(p); ok {
				modelTag = modelTagForDir(p)
				return p, nil
			}
			return "", fmt.Errorf("%s=%q missing det.onnx/rec.onnx", key, p)
		}
	}
	modelOnce.Do(func() {
		modelDir, modelErr = ensureHFModels(PreferredHFRevision)
		if modelErr == nil {
			modelTag = PreferredHFRevision
		}
	})
	return modelDir, modelErr
}

func modelTagForDir(p string) string {
	if _, err := os.Stat(filepath.Join(p, "rec_cjk.onnx")); err == nil {
		return PreferredHFRevision + "+"
	}
	return "custom"
}

func modelDirOK(p string) (bool, error) {
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() {
		return false, err
	}
	for _, n := range []string{"det.onnx", "rec.onnx", "meta.json"} {
		if _, err := os.Stat(filepath.Join(p, n)); err != nil {
			return false, err
		}
	}
	return true, nil
}

func ensureHFModels(revision string) (string, error) {
	py := resolveIndraPython()
	if py == "" {
		return "", fmt.Errorf("need Python with huggingface_hub to fetch indra models %s (or set DOC2MD_INDRA_MODEL_DIR)", revision)
	}
	// allow_patterns keep download small; script/rec_cjk/orient come with *.onnx
	code := fmt.Sprintf(`
from huggingface_hub import snapshot_download
p = snapshot_download(
    "datalab-to/indra-ocr",
    revision=%q,
    allow_patterns=["*.onnx", "meta.json", "lm.npz", "lm_*.npz"],
)
print(p)
`, revision)
	cmd := exec.Command(py, "-c", code)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("download indra models %s: %v\n%s", revision, err, strings.TrimSpace(string(out)))
	}
	dir := strings.TrimSpace(string(out))
	// last non-empty line (hub may print progress to stdout on some versions)
	lines := strings.Split(dir, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if s != "" && !strings.HasPrefix(s, "Fetching") {
			dir = s
			break
		}
	}
	if ok, _ := modelDirOK(dir); !ok {
		return "", fmt.Errorf("indra model dir invalid after download: %q", dir)
	}
	return dir, nil
}

func resolveIndraPython() string {
	if p := strings.TrimSpace(os.Getenv("DOC2MD_INDRA_PYTHON")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	bin := ResolveBin()
	if bin != "" {
		scripts := filepath.Dir(bin)
		for _, n := range []string{"python.exe", "python"} {
			p := filepath.Join(scripts, n)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	for _, root := range searchRoots() {
		for _, n := range []string{"python.exe", "python"} {
			p := filepath.Join(root, ".venv-indra", "Scripts", n)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
			p = filepath.Join(root, ".venv-indra", "bin", n)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	if p, err := exec.LookPath("python"); err == nil {
		return p
	}
	return ""
}
