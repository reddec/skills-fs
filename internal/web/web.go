// Package web serves the embedded admin SPA. The built Vite bundle lives in dist and is
// embedded at compile time; a committed placeholder index.html keeps the binary buildable
// before the frontend is built. Unknown paths fall back to index.html for client-side routing.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves SPA assets from the embedded dist directory, falling back to index.html for
// any path that is not a real file so client-side routes resolve.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// dist is embedded above; Sub of an existing root cannot fail.
		panic(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveSPA(w, r, sub)
	})
}

func serveSPA(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	// Resolve client-side routes: anything that is not a real file serves the app shell.
	if _, statErr := fs.Stat(sub, name); statErr != nil {
		name = "index.html"
	}
	http.ServeFileFS(w, r, dist, "dist/"+name)
}
