package cypheria

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// useAgents writes an agents file listing agents (with a command each
// that exists) and turns integration mode on for the test.
func useAgents(t *testing.T, agents map[string]Agent) string {
	t.Helper()
	dir := t.TempDir()
	cmd := filepath.Join(dir, "agent-cli")
	if err := os.WriteFile(cmd, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for id, a := range agents {
		if a.Command == "" {
			a.Command = cmd
			agents[id] = a
		}
	}
	path := filepath.Join(dir, "agents.json")
	writeAgents(t, path, agents)
	t.Setenv(AgentsFileEnv, path)
	return path
}

func writeAgents(t *testing.T, path string, agents map[string]Agent) {
	t.Helper()
	b, err := json.Marshal(agentsFile{Version: 1, Agents: agents})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInactive(t *testing.T) {
	t.Setenv(AgentsFileEnv, "")
	t.Setenv("CODEX_HOME", "/from/magpie")
	if Active() || Listed("codex") || !AgentAllowed("kiro") || !SubscriptionAllowed("kiro") || !PluginsAllowed() {
		t.Fatal("outside integration mode everything is upstream's")
	}
	if Getenv("codex", "CODEX_HOME") != "/from/magpie" || Native("codex", "CODEX_HOME") != "" {
		t.Fatal("Getenv is os.Getenv and Native is empty outside integration mode")
	}
	if _, ok := Executable("codex"); ok {
		t.Fatal("no managed executable outside integration mode")
	}
	if _, ok := Command(context.Background(), "codex"); ok {
		t.Fatal("no managed command outside integration mode")
	}
}

func TestHome(t *testing.T) {
	t.Setenv(HomeEnv, "")
	if _, ok := ConfigDir(); ok {
		t.Fatal("no MAGPIE_HOME, no config dir of its own")
	}
	t.Setenv(HomeEnv, "/x/gateway/")
	if d, _ := ConfigDir(); d != filepath.Join("/x/gateway", "config") {
		t.Fatalf("config %q", d)
	}
	if d, _ := CacheDir(); d != filepath.Join("/x/gateway", "cache") {
		t.Fatalf("cache %q", d)
	}
	t.Setenv(HomeEnv, "relative")
	if err := Validate(); err == nil {
		t.Fatal("a relative MAGPIE_HOME was taken")
	}
}

func TestValidate(t *testing.T) {
	t.Setenv(HomeEnv, "")
	useAgents(t, map[string]Agent{"codex": {}})
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := filepath.Join(dir, "cli")
	os.WriteFile(cmd, nil, 0o755)
	for name, body := range map[string]string{
		"version":      `{"version":2,"agents":{}}`,
		"unknown id":   `{"version":1,"agents":{"kiro":{"command":"` + cmd + `"}}}`,
		"relative cmd": `{"version":1,"agents":{"codex":{"command":"codex"}}}`,
		"missing cmd":  `{"version":1,"agents":{"codex":{"command":"` + filepath.Join(dir, "none") + `"}}}`,
		"relative cwd": `{"version":1,"agents":{"codex":{"command":"` + cmd + `","cwd":"here"}}}`,
		"bad env name": `{"version":1,"agents":{"codex":{"command":"` + cmd + `","env":{"A=B":"c"}}}}`,
		"unknown key":  `{"version":1,"agents":{"codex":{"command":"` + cmd + `","home":"x"}}}`,
		"not json":     `{`,
	} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
		os.WriteFile(p, []byte(body), 0o600)
		t.Setenv(AgentsFileEnv, p)
		if err := Validate(); err == nil {
			t.Errorf("%s: taken", name)
		}
	}
	t.Setenv(AgentsFileEnv, "relative.json")
	if err := Validate(); err == nil {
		t.Error("a relative agents file was taken")
	}
}

func TestScope(t *testing.T) {
	useAgents(t, map[string]Agent{"claude": {}, "agy": {}, "goose": {}})
	if !Active() || !Listed("claude") || Listed("codex") {
		t.Fatal("listed agents")
	}
	if got := Listing(); !slices.Equal(got, []string{"agy", "claude", "goose"}) {
		t.Fatalf("listing %v", got)
	}
	for id, want := range map[string]bool{"claude": true, "antigravity": true, "codex": false, "kiro": false, "deepseek": false} {
		if SubscriptionAllowed(id) != want {
			t.Errorf("subscription %s allowed = %v", id, !want)
		}
	}
	if AgentAllowed("codex") || !AgentAllowed("goose") || PluginsAllowed() {
		t.Fatal("agents and plugins in scope")
	}
}

func TestReload(t *testing.T) {
	path := useAgents(t, map[string]Agent{"claude": {}})
	if !Listed("claude") {
		t.Fatal("claude")
	}
	cmd := ""
	if a, ok := Spec("claude"); ok {
		cmd = a.Command
	}
	time.Sleep(10 * time.Millisecond) // a new modification time
	writeAgents(t, path, map[string]Agent{"codex": {Command: cmd}})
	if Listed("claude") || !Listed("codex") {
		t.Fatal("the rewritten file is read again")
	}
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(path, []byte("{"), 0o600)
	if !Listed("codex") {
		t.Fatal("a broken file keeps the agents read before")
	}
}

func TestEnvironment(t *testing.T) {
	t.Setenv("CODEX_HOME", "/magpie/codex")
	t.Setenv("KEEP", "magpie")
	t.Setenv("DROP", "magpie")
	useAgents(t, map[string]Agent{"codex": {Cwd: "/work", Args: []string{"/lib/codex.js"},
		Env: map[string]string{"CODEX_HOME": "/cypheria/codex", "PATH": "/toolchain/bin", "ONLY": "agent"}}})
	if Getenv("codex", "CODEX_HOME") != "/cypheria/codex" || Native("codex", "CODEX_HOME") != "/cypheria/codex" {
		t.Fatal("the agent's own CODEX_HOME")
	}
	if Getenv("codex", "KEEP") != "magpie" || Native("codex", "KEEP") != "" {
		t.Fatal("a variable the agent doesn't set")
	}
	if Getenv("claude", "CODEX_HOME") != "/magpie/codex" {
		t.Fatal("an agent that isn't listed sees magpie's")
	}
	env := envMap(Environ("codex", nil))
	if env["CODEX_HOME"] != "/cypheria/codex" || env["PATH"] != "/toolchain/bin" || env["KEEP"] != "magpie" || env["ONLY"] != "agent" {
		t.Fatalf("environ %v", env)
	}
	// a caller drops DROP, changes CODEX_HOME and adds X; KEEP it leaves
	var callers []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "DROP=") && !strings.HasPrefix(kv, "CODEX_HOME=") {
			callers = append(callers, kv)
		}
	}
	callers = append(callers, "CODEX_HOME=/account/home", "X=1")
	env = envMap(Rebase("codex", callers))
	if env["CODEX_HOME"] != "/account/home" {
		t.Errorf("the caller's own value wins: %q", env["CODEX_HOME"])
	}
	if _, ok := env["DROP"]; ok {
		t.Error("what the caller dropped stays dropped")
	}
	if env["X"] != "1" || env["KEEP"] != "magpie" || env["PATH"] != "/toolchain/bin" || env["ONLY"] != "agent" {
		t.Errorf("rebased %v", env)
	}
	cmd, ok := Command(context.Background(), "codex", "--version")
	if !ok || cmd.Dir != "/work" || !slices.Equal(cmd.Args[1:], []string{"/lib/codex.js", "--version"}) {
		t.Fatalf("command %v in %q", cmd.Args, cmd.Dir)
	}
	if p, ok := Executable("codex"); !ok || p == "" {
		t.Fatal("codex's executable")
	}
	if p, ok := Executable("claude"); !ok || p != "" {
		t.Fatal("an agent that isn't listed has none, and no fallback")
	}
}

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	return m
}
