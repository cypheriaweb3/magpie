package wslrun

import (
	"testing"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/cypheria/cypheriatest"
)

// A magpie Cypheria runs never runs an agent's CLI found in a WSL distro.
func TestNoWSLForCypheria(t *testing.T) {
	cypheriatest.Use(t, map[string]cypheria.Agent{"claude": {}})
	on := On
	On = true
	t.Cleanup(func() { On = on })
	found.Lock()
	found.tools = map[string]*Tool{"claude": {Distro: "Ubuntu"}}
	found.Unlock()
	t.Cleanup(Forget)
	if _, ok := Find("claude"); ok {
		t.Fatal("found in WSL")
	}
	if _, ok := Known("claude"); ok {
		t.Fatal("known in WSL")
	}
}
