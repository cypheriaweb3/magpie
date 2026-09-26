package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestLoadAdminConfig(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		unsetEnv(t, adminAddrEnv)
		unsetEnv(t, adminTokenEnv)
		_, enabled, err := loadAdminConfig()
		if err != nil || enabled {
			t.Fatalf("enabled=%v err=%v", enabled, err)
		}
	})
	t.Run("address only", func(t *testing.T) {
		t.Setenv(adminAddrEnv, "127.0.0.1:3430")
		unsetEnv(t, adminTokenEnv)
		_, _, err := loadAdminConfig()
		if err == nil || !strings.Contains(err.Error(), adminTokenEnv) {
			t.Fatalf("expected missing token error, got %v", err)
		}
	})
	t.Run("token only", func(t *testing.T) {
		unsetEnv(t, adminAddrEnv)
		t.Setenv(adminTokenEnv, "secret")
		_, _, err := loadAdminConfig()
		if err == nil || !strings.Contains(err.Error(), adminAddrEnv) {
			t.Fatalf("expected missing address error, got %v", err)
		}
	})
	t.Run("empty", func(t *testing.T) {
		t.Setenv(adminAddrEnv, " ")
		t.Setenv(adminTokenEnv, "")
		if _, _, err := loadAdminConfig(); err == nil {
			t.Fatal("expected empty configuration to fail")
		}
	})
	t.Run("blank token", func(t *testing.T) {
		t.Setenv(adminAddrEnv, "127.0.0.1:3430")
		t.Setenv(adminTokenEnv, "   ")
		if _, _, err := loadAdminConfig(); err == nil {
			t.Fatal("expected blank token to fail")
		}
	})
	t.Run("enabled", func(t *testing.T) {
		t.Setenv(adminAddrEnv, " 0.0.0.0:3430 ")
		t.Setenv(adminTokenEnv, " secret with spaces ")
		cfg, enabled, err := loadAdminConfig()
		if err != nil || !enabled {
			t.Fatalf("enabled=%v err=%v", enabled, err)
		}
		if cfg.addr != "0.0.0.0:3430" || cfg.token != " secret with spaces " {
			t.Fatalf("unexpected config: %#v", cfg)
		}
	})
}

type testAdminServer struct {
	serveErr error
	stopped  atomic.Bool
}

func (*testAdminServer) Address() string { return "127.0.0.1:0" }
func (s *testAdminServer) Serve() error  { return s.serveErr }
func (s *testAdminServer) Shutdown(context.Context) error {
	s.stopped.Store(true)
	return nil
}

func TestAdminFailureStopsGateway(t *testing.T) {
	want := errors.New("admin failed")
	admin := &testAdminServer{serveErr: want}
	gatewayStopped := make(chan struct{})
	err := serveGatewayAndAdmin(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		close(gatewayStopped)
		return nil
	}, admin)
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	if !admin.stopped.Load() {
		t.Fatal("admin was not shut down")
	}
	select {
	case <-gatewayStopped:
	default:
		t.Fatal("gateway did not observe cancellation")
	}
}

func TestGatewayFailureStopsAdmin(t *testing.T) {
	want := errors.New("gateway failed")
	adminWait := make(chan struct{})
	// Keep the fake admin in Serve until Shutdown is called.
	blocking := &blockingAdminServer{stopped: adminWait}
	err := serveGatewayAndAdmin(context.Background(), func(context.Context) error { return want }, blocking)
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	select {
	case <-adminWait:
	default:
		t.Fatal("admin was not shut down")
	}
}

type blockingAdminServer struct {
	stopped chan struct{}
}

func (*blockingAdminServer) Address() string { return "127.0.0.1:0" }
func (s *blockingAdminServer) Serve() error {
	<-s.stopped
	return nil
}
func (s *blockingAdminServer) Shutdown(context.Context) error {
	close(s.stopped)
	return nil
}
