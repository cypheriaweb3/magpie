package provider

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/cypheria/cypheriatest"
)

// cypheriaSubscriptions puts the test in integration mode with Claude,
// Codex, Copilot, Cursor, Devin, Gemini and Grok, each in a folder of the
// answered root, and nothing else.
func cypheriaSubscriptions(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	in := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
	cypheriatest.Use(t, map[string]cypheria.Agent{
		"claude":  {Env: map[string]string{"CLAUDE_CONFIG_DIR": in("claude")}},
		"codex":   {Env: map[string]string{"CODEX_HOME": in("codex")}},
		"copilot": {Env: map[string]string{"COPILOT_HOME": in("copilot")}},
		"cursor":  {Env: map[string]string{"CURSOR_CONFIG_DIR": in("cursor")}},
		"devin":   {Env: map[string]string{"XDG_DATA_HOME": in("devin", "data"), "APPDATA": in("devin", "appdata")}},
		"gemini":  {Env: map[string]string{"GEMINI_CLI_HOME": in("gemini")}},
		"grok":    {Env: map[string]string{"GROK_HOME": in("grok"), "PATH": "/cypheria/toolchain/bin"}},
	})
	return root
}

func TestClaudeKeychainService(t *testing.T) {
	t.Setenv(cypheria.AgentsFileEnv, "")
	t.Setenv("CLAUDE_CONFIG_DIR", "/anywhere")
	if s := claudeKeychainService(); s != "Claude Code-credentials" {
		t.Fatalf("upstream reads the plain item: %q", s)
	}
	// the item Claude Code made for this folder on a Mac Cypheria runs on
	cypheriatest.Use(t, map[string]cypheria.Agent{"claude": {Env: map[string]string{
		"CLAUDE_CONFIG_DIR": "/Users/ridewindx/.cypheria/agents/claude/home"}}})
	if s := claudeKeychainService(); s != "Claude Code-credentials-2305c151" {
		t.Fatalf("Cypheria's Claude Code's item: %q", s)
	}
}

func TestCypheriaSubscriptionPaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := cypheriaSubscriptions(t)
	in := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
	want := map[string][2]string{
		"claude credentials": {claudeCredentialsPath(), in("claude", ".credentials.json")},
		"claude profile":     {claudeProfilePath(), in("claude", ".claude.json")},
		"codex auth":         {codexAuthPath(), in("codex", "auth.json")},
		"codex home":         {codexCLIHome(), in("codex")},
		"copilot":            {copilotCLIHome(), in("copilot")},
		"cursor":             {cursorAuthFile(), in("cursor", "auth.json")},
		"cursor usage":       {cursorAuthPath(), in("cursor", "auth.json")},
		"gemini":             {geminiDir(), in("gemini", ".gemini")},
		"grok":               {GrokHome(), in("grok")},
	}
	if runtime.GOOS == "windows" {
		want["devin"] = [2]string{DevinCredentialsPath(), in("devin", "appdata", "devin", "credentials.toml")}
	} else {
		want["devin"] = [2]string{DevinCredentialsPath(), in("devin", "data", "devin", "credentials.toml")}
	}
	for name, w := range want {
		if w[0] != w[1] {
			t.Errorf("%s: %s, want %s", name, w[0], w[1])
		}
	}
	for _, id := range []string{"claude", "codex", "cursor", "devin", "grok"} {
		exe := map[string]func() string{"claude": claudeExecutable, "codex": codexExecutable,
			"cursor": CursorExecutable, "devin": DevinExecutable, "grok": GrokExecutable}[id]()
		if a, _ := cypheria.Spec(id); exe != a.Command {
			t.Errorf("%s runs %q, not Cypheria's %q", id, exe, a.Command)
		}
	}
}

func TestCypheriaScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cypheriaSubscriptions(t)
	for id, out := range map[string]bool{"claude": false, "grok": false, "antigravity": true, "kiro": true,
		"zed": true, "workbuddy": true, CommandCodePlanID: true, "deepseek": false, "openrouter": false} {
		if outOfScope(id) != out {
			t.Errorf("%s out of scope = %v", id, !out)
		}
	}
	for _, id := range []string{"kiro", "zed", "factory", "qoder", "antigravity"} {
		if _, err := StartSignIn(id); err == nil || !strings.Contains(err.Error(), "Cypheria") {
			t.Errorf("signing in to %s: %v", id, err)
		}
		if Logins(id) != nil {
			t.Errorf("%s's accounts listed", id)
		}
		if err := SwitchLogin(id, "a@b"); err == nil {
			t.Errorf("switching %s's account", id)
		}
		if err := SetLoginOn(id, "a@b", true); err == nil {
			t.Errorf("switching %s's account on", id)
		}
	}
	if _, err := ImportLogins(context.Background(), "zed", nil); err == nil {
		t.Error("importing to zed")
	}
	for _, p := range Accounts() {
		if outOfScope(p.ID) {
			t.Errorf("%s listed", p.ID)
		}
	}
	hidden := map[string]bool{}
	hideOutOfScope(hidden)
	if !hidden["kiro"] || hidden["claude"] || hidden["deepseek"] {
		t.Errorf("hidden from the quota fetches: %v", hidden)
	}
}

// The Copilot editors' sign-in is not Cypheria's Copilot CLI's.
func TestCypheriaCopilotSkipsEditors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cypheriaSubscriptions(t)
	cfg := t.TempDir()
	os.MkdirAll(filepath.Join(cfg, "github-copilot"), 0o700)
	os.WriteFile(filepath.Join(cfg, "github-copilot", "apps.json"), []byte(`{"github.com:x":{"user":"editor","oauth_token":"gho_editor"}}`), 0o600)
	if app, ok := copilotLogin(cfg); ok && app.User == "editor" {
		t.Fatal("the editors' sign-in was read")
	}
}

func TestCypheriaCLICommand(t *testing.T) {
	cypheriaSubscriptions(t)
	a, _ := cypheria.Spec("grok")
	// another account of Grok's, in a home of magpie's: its GROK_HOME wins
	cmd := cliCommand(context.Background(), "grok", "/usr/local/bin/grok", grokOwnEnv(os.Environ(), "/magpie/grok/2"), "models")
	if cmd.Path != a.Command && cmd.Args[0] != a.Command {
		t.Fatalf("runs %v, not Cypheria's %s", cmd.Args, a.Command)
	}
	env := map[string]string{}
	for _, kv := range cmd.Env {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	if env["GROK_HOME"] != "/magpie/grok/2" || env["PATH"] != "/cypheria/toolchain/bin" {
		t.Fatalf("GROK_HOME %q PATH %q", env["GROK_HOME"], env["PATH"])
	}
	// outside integration mode it is upstream's command
	t.Setenv(cypheria.AgentsFileEnv, "")
	cmd = cliCommand(context.Background(), "grok", "/usr/local/bin/grok", nil, "models")
	if cmd.Args[0] != "/usr/local/bin/grok" || cmd.Env != nil {
		t.Fatalf("upstream's: %v %v", cmd.Args, cmd.Env)
	}
}

func TestMagpieHome(t *testing.T) {
	t.Setenv(cypheria.HomeEnv, "/cypheria/gateway")
	if p := Path(); p != filepath.Join("/cypheria/gateway", "config", "providers.json") {
		t.Fatalf("providers.json at %s", p)
	}
}
