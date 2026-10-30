package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"doc2md/internal/convert"
	"doc2md/internal/pdf"
)

func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxBytes)
	if err := r.ParseMultipartForm(s.MaxBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, `{"error":"file too large (max 50MB)"}`, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, `{"error":"invalid multipart form"}`, http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing form field file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, `{"error":"read upload failed"}`, http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, `{"error":"empty file"}`, http.StatusBadRequest)
		return
	}

	ocrMode := pdf.ParseOCRMode(firstNonEmpty(r.FormValue("ocr"), r.URL.Query().Get("ocr")))
	ocrModel := firstNonEmpty(r.FormValue("ocr_model"), r.URL.Query().Get("ocr_model"), "tiny")
	ocrUseCls := parseBoolish(firstNonEmpty(r.FormValue("ocr_use_cls"), r.URL.Query().Get("ocr_use_cls"), "0"))
	pipeline := convert.ParsePipeline(firstNonEmpty(r.FormValue("pipeline"), r.URL.Query().Get("pipeline")))
	featEngine := firstNonEmpty(r.FormValue("feat_engine"), r.URL.Query().Get("feat_engine"))
	raw := r.URL.Query().Get("raw") == "1" || r.FormValue("raw") == "1"

	result, err := convert.Convert(r.Context(), hdr.Filename, data, convert.Options{
		OCRMode:    ocrMode,
		OCRModel:   ocrModel,
		OCRUseCls:  ocrUseCls,
		Pipeline:   pipeline,
		FeatEngine: featEngine,
		OCR:        s.OCR,
	})
	if err != nil {
		status := http.StatusBadRequest
		msg := err.Error()
		if strings.Contains(msg, "ocr not configured") || strings.Contains(msg, "renderer not found") ||
			strings.Contains(msg, "indra-ocr not found") || strings.Contains(msg, "oar CLI not found") {
			status = http.StatusServiceUnavailable
		}
		writeJSONError(w, status, msg)
		return
	}

	if raw {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(result.Markdown))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(result)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseBoolish(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
