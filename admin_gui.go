//go:build !nogui

package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/gui"
)

//go:embed openapi.json
var adminOpenAPI []byte

var errServerWindows = errors.New("desktop integration is unavailable in server mode")

// serverWindows deliberately leaves operating-system actions to the client
// of the admin API. That matters when the client and magpie are on different
// machines, and keeps the existing GUI handler otherwise untouched.
type serverWindows struct{}

func (serverWindows) HidePanel()                       {}
func (serverWindows) ShowMain(string)                  {}
func (serverWindows) Quit()                            {}
func (serverWindows) OpenURL(string)                   {}
func (serverWindows) OpenFolder(string) error          { return errServerWindows }
func (serverWindows) Copy(string) bool                 { return false }
func (serverWindows) FitPanel(int, gui.Glide)          {}
func (serverWindows) TintPanel([4]uint8, int) bool     { return false }
func (serverWindows) TintTitleBar([4]uint8, bool) bool { return false }
func (serverWindows) SetTextSize(int)                  {}
func (serverWindows) ChooseFolder(string) (string, error) {
	return "", errServerWindows
}
func (serverWindows) Remote() bool { return true }

type guiAdminServer struct {
	listener net.Listener
	server   *http.Server
}

func prepareAdmin(gw *gateway.Server) (adminServer, error) {
	cfg, enabled, err := loadAdminConfig()
	if err != nil || !enabled {
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return nil, fmt.Errorf("admin listen on %s: %w", cfg.addr, err)
	}
	gui.Version = version
	return &guiAdminServer{
		listener: listener,
		server: &http.Server{
			Handler:           adminHandler(gw, cfg.token),
			ReadHeaderTimeout: 30 * time.Second,
			IdleTimeout:       5 * time.Minute,
		},
	}, nil
}

func (s *guiAdminServer) Address() string { return s.listener.Addr().String() }

func (s *guiAdminServer) Serve() error {
	if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *guiAdminServer) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

func adminHandler(gw *gateway.Server, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(adminOpenAPI)
	})
	mux.Handle("/", gui.Handler(serverWindows{}, gw))
	return bearerAuth(token, mux)
}

func bearerAuth(token string, next http.Handler) http.Handler {
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		got := ""
		if scheme, value, ok := strings.Cut(auth, " "); ok && strings.EqualFold(scheme, "Bearer") {
			got = value
		}
		have := sha256.Sum256([]byte(got))
		if got == "" || subtle.ConstantTimeCompare(have[:], want[:]) != 1 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
