package sessions

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/cypheria/cypheriatest"
)

// In integration mode the sessions are read where Cypheria's agents keep
// them, and only theirs.
func TestCypheriaSessionDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	root := t.TempDir()
	in := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
	// sessions of an agent Cypheria doesn't manage, on this machine
	os.MkdirAll(filepath.Join(home, ".grok", "sessions"), 0o700)
	os.MkdirAll(filepath.Join(home, ".cursor", "chats"), 0o700)
	os.MkdirAll(filepath.Join(home, ".factory", "sessions"), 0o700)
	os.MkdirAll(filepath.Join(home, ".claude", "projects", "p"), 0o700)
	os.WriteFile(filepath.Join(home, ".claude", "projects", "p", "s.jsonl"), []byte("{}\n"), 0o600)
	os.MkdirAll(in("pi"), 0o700)
	cypheriatest.Use(t, map[string]cypheria.Agent{
		"claude": {Env: map[string]string{"CLAUDE_CONFIG_DIR": in("claude")}},
		"codex":  {Env: map[string]string{"CODEX_HOME": in("codex")}},
		"pi":     {Env: map[string]string{"PI_CODING_AGENT_DIR": in("pi")}},
	})
	if ClaudeDir() != in("claude") || CodexDir() != in("codex") || PiDir() != in("pi") {
		t.Fatalf("dirs %s %s %s", ClaudeDir(), CodexDir(), PiDir())
	}
	if got := Dirs(); !slices.Equal(got, []string{in("claude"), in("codex"), in("pi")}) {
		t.Fatalf("Dirs %v: Cypheria's agents' only", got)
	}
	for _, f := range allFiles() {
		if !cypheria.Listed(f.agent) {
			t.Errorf("%s: a session of %s, which Cypheria doesn't manage", f.path, f.agent)
		}
	}
}
