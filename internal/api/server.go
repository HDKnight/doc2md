package api

import (
	"net/http"
	"time"

	"doc2md/internal/ocr"
)

// Server is the HTTP API.
type Server struct {
	OCR        ocr.Engine
	MaxBytes   int64
	Mux        *http.ServeMux
}

// NewServer wires routes.
func NewServer(engine ocr.Engine) *Server {
	if engine == nil {
		engine = ocr.NewFromEnv()
	}
	s := &Server{
		OCR:      engine,
		MaxBytes: 50 << 20, // 50MB
		Mux:      http.NewServeMux(),
	}
	s.Mux.HandleFunc("GET /health", s.handleHealth)
	s.Mux.HandleFunc("POST /convert", s.handleConvert)
	s.mountStatic()
	return s
}

// Handler returns the root handler with timeouts applied by the caller.
func (s *Server) Handler() http.Handler {
	return s.Mux
}

// ListenAndServe starts the HTTP server.
func ListenAndServe(addr string, h http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}
	return srv.ListenAndServe()
}
