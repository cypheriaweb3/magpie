package gui

import (
	_ "embed"
	"net/http"
)

// openAPI describes the page's API for programs that drive `magpie web`
// with its key as a bearer token (Cypheria's builds; see the README).
//
//go:embed openapi.json
var openAPI []byte

// withOpenAPI serves the description beside the page, behind the same key.
func withOpenAPI(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.json", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.Header().Set("Cache-Control", "no-store")
		_, _ = rw.Write(openAPI)
	})
	mux.Handle("/", next)
	return mux
}
