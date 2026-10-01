package plugin

import (
	"context"
	"testing"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/cypheria/cypheriatest"
)

func TestPluginsOffForCypheria(t *testing.T) {
	cypheriatest.Use(t, map[string]cypheria.Agent{"claude": {}})
	ctx := context.Background()
	if l := Load(); len(l.Plugins) != 0 {
		t.Fatal("plugins loaded")
	}
	if Cached() != nil || Market(ctx) != nil {
		t.Fatal("plugin providers or market")
	}
	if _, err := Add(ctx, "opencode-gemini-auth"); err != errOff {
		t.Fatalf("add: %v", err)
	}
	if _, err := Bun(ctx); err != errOff {
		t.Fatalf("bun: %v", err)
	}
	if err := Call(ctx, "list", nil, nil); err != errOff {
		t.Fatalf("call: %v", err)
	}
	if _, err := Search(ctx, "auth"); err != errOff {
		t.Fatalf("search: %v", err)
	}
}
