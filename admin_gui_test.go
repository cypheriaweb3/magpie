//go:build !nogui

package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/gateway"
)

func authorizedRequest(method, target, body, token string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestBearerAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := bearerAuth("correct horse battery staple", next)
	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic abc", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
		{"extra token", "Bearer correct horse battery staple extra", http.StatusUnauthorized},
		{"case-insensitive scheme", "bEaReR correct horse battery staple", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d", w.Code, tc.want)
			}
			if tc.want == http.StatusUnauthorized {
				if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Fatalf("WWW-Authenticate = %q", got)
				}
				var body map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["error"] != "unauthorized" {
					t.Fatalf("unexpected body %q: %v", w.Body.String(), err)
				}
			}
		})
	}
}

func TestAdminHandlerProtectsAllSurfaces(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := adminHandler(gateway.New(), "secret")
	for _, path := range []string{"/", "/api/state", "/openapi.json"} {
		t.Run(path, func(t *testing.T) {
			without := httptest.NewRecorder()
			h.ServeHTTP(without, authorizedRequest("GET", path, "", ""))
			if without.Code != http.StatusUnauthorized {
				t.Fatalf("without auth: got %d", without.Code)
			}
			with := httptest.NewRecorder()
			h.ServeHTTP(with, authorizedRequest("GET", path, "", "secret"))
			if with.Code != http.StatusOK {
				t.Fatalf("with auth: got %d: %s", with.Code, with.Body.String())
			}
		})
	}
}

func TestServerHostActions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := adminHandler(gateway.New(), "secret")
	for _, tc := range []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{"POST", "/api/open", `{"URL":"https://example.com"}`, http.StatusNoContent},
		{"POST", "/api/copy", `{"Text":"copy me"}`, http.StatusNotImplemented},
		{"POST", "/api/window/hide", "", http.StatusNoContent},
		{"POST", "/api/settings/reveal", "", http.StatusBadRequest},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, authorizedRequest(tc.method, tc.path, tc.body, "secret"))
		if w.Code != tc.want {
			t.Errorf("%s %s: got %d (%s), want %d", tc.method, tc.path, w.Code, w.Body.String(), tc.want)
		}
	}
}

func TestPreparedAdminServesSharedGateway(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(adminAddrEnv, "127.0.0.1:0")
	t.Setenv(adminTokenEnv, "secret")
	admin, err := prepareAdmin(gateway.New())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- admin.Serve() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = admin.Shutdown(ctx)
		<-done
	})

	req, _ := http.NewRequest("GET", "http://"+admin.Address()+"/api/providers", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("got %d: %s", res.StatusCode, body)
	}
	var state struct {
		Gateway struct {
			Mine bool `json:"mine"`
		} `json:"gateway"`
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if !state.Gateway.Mine {
		t.Fatal("admin handler does not share its gateway instance")
	}
}

func TestPrepareAdminReportsBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv(adminAddrEnv, listener.Addr().String())
	t.Setenv(adminTokenEnv, "secret")
	if _, err := prepareAdmin(gateway.New()); err == nil || !strings.Contains(err.Error(), "admin listen") {
		t.Fatalf("expected bind error, got %v", err)
	}
}
