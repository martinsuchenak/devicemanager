package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed assets
var Assets embed.FS

// GetFS returns the UI filesystem
func GetFS() fs.FS {
	return Assets
}

// AssetHandler serves UI assets with cache control for index.html
func AssetHandler() http.HandlerFunc {
	// Create a sub filesystem from the assets directory
	assetsFS, _ := fs.Sub(Assets, "assets")

	return func(w http.ResponseWriter, r *http.Request) {
		// Handle root path - serve index.html directly
		if r.URL.Path == "/" || r.URL.Path == "" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			content, err := fs.ReadFile(assetsFS, "index.html")
			if err != nil {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			w.Write(content)
			return
		}

		// Strip /assets/ prefix if present
		path := r.URL.Path
		if strings.HasPrefix(path, "/assets/") {
			path = strings.TrimPrefix(path, "/assets/")
		}

		// Set content type headers
		ext := strings.LastIndex(path, ".")
		if ext > 0 {
			switch path[ext:] {
			case ".css":
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case ".js":
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			}
		}

		// Serve the file
		content, err := fs.ReadFile(assetsFS, strings.TrimPrefix(path, "/"))
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Write(content)
	}
}
