package agent

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/cypheria/cypheriatest"
)

// cypheriaAgents lists the twelve agents with a folder each, in t's temp
// folder, set the way Cypheria sets them; it answers that root.
func cypheriaAgents(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := func(id string) string { return filepath.Join(root, id) }
	cypheriatest.Use(t, map[string]cypheria.Agent{
		"claude":   {Env: map[string]string{"CLAUDE_CONFIG_DIR": home("claude")}},
		"codex":    {Env: map[string]string{"CODEX_HOME": home("codex")}},
		"gemini":   {Env: map[string]string{"GEMINI_CLI_HOME": home("gemini")}},
		"agy":      {Env: map[string]string{"GEMINI_HOME": home("agy")}},
		"cursor":   {Env: map[string]string{"CURSOR_CONFIG_DIR": home("cursor")}},
		"copilot":  {Env: map[string]string{"COPILOT_HOME": home("copilot")}},
		"devin":    {Env: map[string]string{"XDG_CONFIG_HOME": filepath.Join(home("devin"), "config"), "XDG_DATA_HOME": filepath.Join(home("devin"), "data")}},
		"goose":    {Env: map[string]string{"GOOSE_PATH_ROOT": home("goose")}},
		"opencode": {Env: map[string]string{"XDG_CONFIG_HOME": filepath.Join(home("opencode"), "config"), "XDG_DATA_HOME": filepath.Join(home("opencode"), "data")}},
		"pi":       {Env: map[string]string{"PI_CODING_AGENT_DIR": home("pi")}},
		"cline":    {Env: map[string]string{"CLINE_DIR": home("cline")}},
		"grok":     {Env: map[string]string{"GROK_HOME": home("grok")}},
	})
	return root
}

func TestCypheriaAgentPaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	root := cypheriaAgents(t)
	in := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
	want := map[string]string{
		"claude":   in("claude", "settings.json"),
		"codex":    in("codex", "config.toml"),
		"gemini":   in("gemini", ".gemini", "settings.json"),
		"agy":      in("agy", "antigravity-cli", "settings.json"),
		"cursor":   in("cursor", "cli-config.json"),
		"copilot":  in("copilot", "settings.json"),
		"goose":    in("goose", "config", "config.yaml"),
		"cline":    in("cline", "data", "settings", "providers.json"),
		"grok":     in("grok", "config.toml"),
		"opencode": in("opencode", "config", "opencode"), // its folder
		"pi":       in("pi"),                             // its folder
	}
	if runtime.GOOS != "windows" {
		want["devin"] = in("devin", "config", "devin", "config.json")
	}
	all := All()
	var ids []string
	for _, a := range all {
		ids = append(ids, a.ID)
		if !a.Detected() {
			t.Errorf("%s isn't detected", a.ID)
		}
		w, ok := want[a.ID]
		if !ok {
			continue
		}
		got := a.Path
		if a.ID == "opencode" || a.ID == "pi" {
			got = a.Dir
		}
		if got != w {
			t.Errorf("%s: %s, want %s", a.ID, got, w)
		}
	}
	slices.Sort(ids)
	if !slices.Equal(ids, cypheria.Agents) {
		t.Fatalf("agents %v, want only Cypheria's %v", ids, cypheria.Agents)
	}
	if len(Detected()) != len(cypheria.Agents) {
		t.Fatalf("detected %d", len(Detected()))
	}
	// StandIn reads Codex's and Claude Code's settings where these are
	if codexHomeOf(here(t.TempDir())) != in("codex") || claudeHomeOf(here(t.TempDir())) != in("claude") {
		t.Error("Codex's and Claude Code's folders are Cypheria's")
	}
}

// Outside integration mode the agents are upstream's, wherever a variable
// upstream ignores points.
func TestUpstreamAgentPaths(t *testing.T) {
	t.Setenv(cypheria.AgentsFileEnv, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "elsewhere"))
	t.Setenv("CODEX_HOME", filepath.Join(home, "elsewhere"))
	for _, a := range All() {
		switch a.ID {
		case "claude":
			if a.Path != filepath.Join(home, ".claude", "settings.json") {
				t.Errorf("claude: %s", a.Path)
			}
		case "codex":
			if a.Path != filepath.Join(home, ".codex", "config.toml") {
				t.Errorf("codex: %s", a.Path)
			}
		}
	}
	if len(All()) <= len(cypheria.Agents) {
		t.Fatal("every agent magpie knows")
	}
}

func TestCypheriaCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake CLI is a shell script")
	}
	cypheriaAgents(t)
	a, err := Find("codex")
	if err != nil {
		t.Fatal(err)
	}
	c, ok := a.CLI()
	if !ok || c.Version != "1.2.3" || c.Update || c.Command != "" {
		t.Fatalf("CLI %+v %v: the version of Cypheria's, no update", c, ok)
	}
	if _, err := a.UpdateCLI(); err == nil || !strings.Contains(err.Error(), "Cypheria") {
		t.Fatalf("an update of Cypheria's CLI: %v", err)
	}
}
