package httpserver

import (
	"embed"
	"net/http"
	"path"
	"strings"
)

//go:embed adminui/*
var adminUIFS embed.FS

func (s *Server) adminUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	assetPath := strings.TrimPrefix(r.URL.Path, "/admin/ui")
	assetPath = strings.TrimPrefix(assetPath, "/")
	if assetPath == "" {
		assetPath = "index.html"
	}
	if strings.Contains(assetPath, "..") {
		s.notFound(w, r)
		return
	}
	filePath := path.Clean("adminui/" + assetPath)
	if !strings.HasPrefix(filePath, "adminui/") {
		s.notFound(w, r)
		return
	}

	content, err := adminUIFS.ReadFile(filePath)
	if err != nil {
		fallbackPath := path.Clean("adminui/index.html")
		fallback, fallbackErr := adminUIFS.ReadFile(fallbackPath)
		if fallbackErr != nil {
			s.notFound(w, r)
			return
		}
		w.Header().Set("Content-Type", adminUIContentType(fallbackPath))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fallback)
		return
	}
	w.Header().Set("Content-Type", adminUIContentType(filePath))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func adminUIContentType(filePath string) string {
	switch {
	case strings.HasSuffix(filePath, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(filePath, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(filePath, ".js"):
		return "application/javascript; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
