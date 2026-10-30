package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var webFS embed.FS

func (s *Server) mountStatic() {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return
	}
	s.Mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(mustSub(sub, "static"))))
	s.Mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "index.html")
	})
}

func mustSub(fsys fs.FS, path string) fs.FS {
	sub, err := fs.Sub(fsys, path)
	if err != nil {
		panic(err)
	}
	return sub
}
