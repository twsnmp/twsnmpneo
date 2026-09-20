package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// GetStaticFS returns the embedded filesystem rooted at dist.
func GetStaticFS() (http.FileSystem, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}
