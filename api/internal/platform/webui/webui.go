// Package webui serves the built single-page app from the binary.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Embedded returns the app build that was copied into dist/ before compiling.
func Embedded() fs.FS {
	sub, _ := fs.Sub(dist, "dist")
	return sub
}

// Handler serves static files and falls back to index.html for client-side routes.
func Handler(files fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		switch {
		case isFile(files, name):
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
		case name != "index.html" && path.Ext(name) != "":
			http.NotFound(w, r)
		default:
			serveIndex(w, files)
		}
	})
}

func isFile(files fs.FS, name string) bool {
	if name == "" || name == "index.html" {
		return false
	}
	info, err := fs.Stat(files, name)
	return err == nil && !info.IsDir()
}

func serveIndex(w http.ResponseWriter, files fs.FS) {
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		http.Error(w, "The app has not been built into this binary.", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}
