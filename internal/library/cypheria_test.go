package library

import (
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/cypheria/cypheriatest"
)

// The library gives Cypheria's agents their instructions, servers and
// skills where they read them, and leaves rtk alone.
func TestCypheriaTargets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	in := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
	cypheriatest.Use(t, map[string]cypheria.Agent{
		"claude":  {Env: map[string]string{"CLAUDE_CONFIG_DIR": in("claude")}},
		"codex":   {Env: map[string]string{"CODEX_HOME": in("codex")}},
		"cursor":  {Env: map[string]string{"CURSOR_CONFIG_DIR": in("cursor")}},
		"gemini":  {Env: map[string]string{"GEMINI_CLI_HOME": in("gemini")}},
		"agy":     {Env: map[string]string{"GEMINI_HOME": in("agy")}},
		"copilot": {Env: map[string]string{"COPILOT_HOME": in("copilot")}},
	})
	want := map[string][2]string{
		"claude":  {in("claude", "CLAUDE.md"), in("claude", ".claude.json")},
		"codex":   {in("codex", "AGENTS.md"), in("codex", "config.toml")},
		"cursor":  {"", in("cursor", "mcp.json")},
		"gemini":  {in("gemini", ".gemini", "GEMINI.md"), in("gemini", ".gemini", "settings.json")},
		"agy":     {in("agy", "config", "GEMINI.md"), in("agy", "config", "mcp_config.json")},
		"copilot": {in("copilot", "copilot-instructions.md"), in("copilot", "mcp-config.json")},
	}
	seen := 0
	for _, tg := range Targets() {
		w, ok := want[tg.Agent.ID]
		if !ok {
			t.Errorf("%s is no agent of Cypheria's", tg.Agent.ID)
			continue
		}
		seen++
		if tg.Instructions != w[0] || tg.MCP == nil || tg.MCP.Path != w[1] {
			t.Errorf("%s: %q %+v", tg.Agent.ID, tg.Instructions, tg.MCP)
		}
	}
	if seen != len(want) {
		t.Fatalf("%d of %d targets", seen, len(want))
	}
	if apps() != nil {
		t.Fatal("no app outside Cypheria's agents")
	}
	if _, err := SetRTK("claude", true); err != errRTKOff {
		t.Fatalf("rtk: %v", err)
	}
	if _, err := InstallRTK(); err != errRTKOff {
		t.Fatalf("installing rtk: %v", err)
	}
}
