package pdf

import "testing"

func TestPDFInfoPagesLine(t *testing.T) {
	out := "Title: x\r\nPages:           16\r\nEncrypted: no\r\n"
	out = replaceCRLF(out)
	m := pdfInfoPagesRe.FindStringSubmatch(out)
	if len(m) != 2 || m[1] != "16" {
		t.Fatalf("match %#v", m)
	}
}

func replaceCRLF(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			continue
		}
		b = append(b, s[i])
	}
	return string(b)
}
