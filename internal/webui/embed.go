// Package webui serves the frontend build that Vite writes into dist/.
package webui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distribution embed.FS

const (
	indexFile        = "index.html"
	themePlaceholder = `data-theme="dark"`
	assetsPrefix     = "/assets/"
	immutableCache   = "public, max-age=31536000, immutable"
)

// Handler serves static assets and returns index.html (with the signed-in
// user's theme applied before first paint) for every client-side route.
func Handler(themeFor func(*http.Request) string) (http.Handler, error) {
	files, err := fs.Sub(distribution, "dist")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(files, indexFile)
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServerFS(files)

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cleaned := path.Clean(request.URL.Path)
		if strings.HasPrefix(cleaned, assetsPrefix) {
			writer.Header().Set("Cache-Control", immutableCache)
			fileServer.ServeHTTP(writer, request)
			return
		}
		if info, statErr := fs.Stat(files, strings.TrimPrefix(cleaned, "/")); statErr == nil && !info.IsDir() && cleaned != "/"+indexFile {
			fileServer.ServeHTTP(writer, request)
			return
		}
		theme := themeFor(request)
		page := bytes.Replace(index, []byte(themePlaceholder), []byte(`data-theme="`+theme+`"`), 1)
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		writer.Write(page)
	}), nil
}
