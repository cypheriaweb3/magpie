package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	adminAddrEnv  = "MAGPIE_ADMIN_ADDR"
	adminTokenEnv = "MAGPIE_ADMIN_TOKEN"
)

type adminConfig struct {
	addr  string
	token string
}

// adminServer is the optional HTTP server exposing the desktop handler from
// `magpie serve`. Its implementations are build-specific so a nogui build
// never links the GUI package or Wails.
type adminServer interface {
	Address() string
	Serve() error
	Shutdown(context.Context) error
}

// loadAdminConfig enables the admin server only when both settings are
// present. A half-configured server is almost certainly a deployment typo,
// so fail instead of silently leaving it disabled or unauthenticated.
func loadAdminConfig() (adminConfig, bool, error) {
	addr, hasAddr := os.LookupEnv(adminAddrEnv)
	token, hasToken := os.LookupEnv(adminTokenEnv)
	if !hasAddr && !hasToken {
		return adminConfig{}, false, nil
	}
	if !hasAddr || strings.TrimSpace(addr) == "" {
		return adminConfig{}, false, fmt.Errorf("%s is required when %s is set", adminAddrEnv, adminTokenEnv)
	}
	if !hasToken || strings.TrimSpace(token) == "" {
		return adminConfig{}, false, fmt.Errorf("%s is required when %s is set", adminTokenEnv, adminAddrEnv)
	}
	return adminConfig{addr: strings.TrimSpace(addr), token: token}, true, nil
}

// serveGatewayAndAdmin keeps the optional admin listener and the gateway in
// one lifecycle. The first one to stop cancels and closes the other.
func serveGatewayAndAdmin(parent context.Context, serveGateway func(context.Context) error, admin adminServer) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	errs := make(chan error, 2)
	go func() { errs <- serveGateway(ctx) }()
	servers := 1
	if admin != nil {
		servers++
		go func() { errs <- admin.Serve() }()
	}
	err := <-errs
	cancel()
	if admin != nil {
		shutdownCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		_ = admin.Shutdown(shutdownCtx)
		stop()
	}
	// Let the other server observe cancellation before returning. This keeps
	// tests, embedders and repeated serve attempts from inheriting a listener.
	if servers > 1 {
		select {
		case <-errs:
		case <-time.After(2 * time.Second):
		}
	}
	return err
}
