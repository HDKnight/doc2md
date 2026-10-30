//go:build ignore

package main

import (
	"archive/zip"
	"os"
	"path/filepath"
)

func main() {
	out := filepath.Join("testdata", "hello.docx")
	_ = os.MkdirAll("testdata", 0o755)
	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, _ := zw.Create("[Content_Types].xml")
	_, _ = w.Write([]byte(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`))
	w, _ = zw.Create("word/document.xml")
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Hello from DOCX API</w:t></w:r></w:p></w:body>
</w:document>`))
	_ = zw.Close()
}
