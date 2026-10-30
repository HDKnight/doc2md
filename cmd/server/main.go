package main

import (
	"log"
	"os"

	"doc2md/internal/api"
	"doc2md/internal/ocr"
)

func main() {
	addr := os.Getenv("DOC2MD_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	engine := ocr.NewFromEnv()
	defer engine.Close()
	defer ocr.DefaultPool().Close()
	if ocr.Configured(engine) {
		// 默认不预热 PP-OCR：常驻 worker 会占内存/ORT 线程，把表征管线的 oar structure
		// 从 ~15s 拖到 ~34s（同 PDF 实测）。rules 管线首次 OCR 时仍会按需加载。
		// 若只要 rules、想启动即热：DOC2MD_OCR_WARM=1
		if os.Getenv("DOC2MD_OCR_WARM") == "1" {
			log.Printf("OCR: enabled; warming %d workers (DOC2MD_OCR_WARM=1)…", ocr.DefaultWorkers())
			go func() {
				if err := ocr.WarmDefault("tiny", false); err != nil {
					log.Printf("OCR warm-up: %v", err)
					return
				}
				log.Printf("OCR warm-up done (tiny, %d workers, dpi=%d)", ocr.DefaultWorkers(), ocr.DefaultOCRDPI())
			}()
		} else {
			log.Printf("OCR: enabled (lazy; set DOC2MD_OCR_WARM=1 to pre-warm rules workers)")
		}
	} else {
		log.Printf("OCR: disabled — pip install faster-paddle (or use .venv-fpaddle), or set DOC2MD_OCR_CMD")
	}

	srv := api.NewServer(engine)
	log.Printf("doc2md listening on %s", addr)
	if err := api.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
