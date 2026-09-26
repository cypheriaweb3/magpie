//go:build nogui

package main

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/gateway"
)

func TestNoGUIBuildRejectsAdmin(t *testing.T) {
	t.Setenv(adminAddrEnv, "127.0.0.1:3430")
	t.Setenv(adminTokenEnv, "secret")
	_, err := prepareAdmin(gateway.New())
	if err == nil || !strings.Contains(err.Error(), "nogui") {
		t.Fatalf("expected nogui error, got %v", err)
	}
}
