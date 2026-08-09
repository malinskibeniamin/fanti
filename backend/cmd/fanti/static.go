package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// withStaticApp serves the built web SPA next to the API: API and health
// paths pass through; file paths serve from dir; everything else falls
// back to index.html for client-side routing.
func withStaticApp(api http.Handler, dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/fanti.v1.") || r.URL.Path == "/healthz" {
			api.ServeHTTP(w, r)

			return
		}

		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean != "." {
			if info, err := os.Stat(filepath.Join(dir, clean)); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)

				return
			}
		}

		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
