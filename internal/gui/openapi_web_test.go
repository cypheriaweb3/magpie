package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/gateway"
)

// apiRequest is a program's call: the key as a bearer token, no cookie.
func apiRequest(method, target, body, token string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func TestWebGuardBearer(t *testing.T) {
	h := webGuard("c", "0123456789abcdef-key", 0, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { rw.WriteHeader(http.StatusTeapot) }))
	for _, tc := range []struct {
		name, auth string
		want       int
	}{
		{"right token", "Bearer 0123456789abcdef-key", http.StatusTeapot},
		{"scheme in any case", "bearer 0123456789abcdef-key", http.StatusTeapot},
		{"wrong token", "Bearer 0123456789abcdef-nope", http.StatusUnauthorized},
		{"empty token", "Bearer ", http.StatusUnauthorized},
		{"another scheme", "Basic 0123456789abcdef-key", http.StatusUnauthorized},
	} {
		r := httptest.NewRequest("POST", "/api/settings", nil)
		r.Header.Set("Authorization", tc.auth)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, rec.Code, tc.want)
		}
	}
	// a wrong token is refused even beside a right cookie
	r := httptest.NewRequest("GET", "/api/state", nil)
	r.Header.Set("Authorization", "Bearer 0123456789abcdef-nope")
	r.AddCookie(&http.Cookie{Name: "c", Value: "0123456789abcdef-key"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("wrong token with a cookie: %d %q", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}
}

// webAPI is `magpie web`'s handler as StartWeb puts it together.
func webAPI(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { served.Store(nil) }) // Handler keeps the gateway it was given
	return webGuard("magpie_web_0", "0123456789abcdef-key", 0, withOpenAPI(Handler(webHost{quit: func() {}}, gateway.New())))
}

func TestWebServesOpenAPI(t *testing.T) {
	h := webAPI(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, apiRequest("GET", "/openapi.json", "", ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without the key: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, apiRequest("GET", "/openapi.json", "", "0123456789abcdef-key"))
	var doc struct {
		OpenAPI string `json:"openapi"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &doc) != nil || doc.OpenAPI != "3.1.0" {
		t.Fatalf("with the key: %d %.80s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, apiRequest("GET", "/api/state", "", "0123456789abcdef-key"))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/state with the key: %d", rec.Code)
	}
}

// The actions a desktop does for its user are left to the program calling
// `magpie web`, which may be on another machine.
func TestWebHostActions(t *testing.T) {
	h := webAPI(t)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/open", `{"URL":"https://example.com"}`, http.StatusNoContent},
		{"POST", "/api/copy", `{"Text":"copy me"}`, http.StatusNotImplemented},
		{"POST", "/api/window/hide", "", http.StatusNoContent},
		{"POST", "/api/settings/reveal", "", http.StatusBadRequest},
		{"POST", "/api/library/projects/choose", "", http.StatusBadRequest},
		{"POST", "/api/omarchy/widget", `{"On":true}`, http.StatusConflict},
		{"POST", "/api/sessions/terminal", `{"Agent":"claude","ID":"missing"}`, http.StatusBadRequest},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, apiRequest(tc.method, tc.path, tc.body, "0123456789abcdef-key"))
		if rec.Code != tc.want {
			t.Errorf("%s %s: %d (%s), want %d", tc.method, tc.path, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestUpdatesDisabled(t *testing.T) {
	u := &updater{}
	u.start()
	if s := u.json().State; s != "off" {
		t.Fatalf("state %q, want off", s)
	}
	if u.begin() {
		t.Fatal("a check began in a build that never updates")
	}
	u.check()
	if s := u.json().State; s != "off" {
		t.Fatalf("state after a check %q, want off", s)
	}
}
