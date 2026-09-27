package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func spaFileServer(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		cleanPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if cleanPath == "." || strings.HasPrefix(cleanPath, "..") {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		requestedFile := filepath.Join(staticDir, cleanPath)
		if info, err := os.Stat(requestedFile); err == nil && !info.IsDir() {
			if strings.HasPrefix(cleanPath, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}
