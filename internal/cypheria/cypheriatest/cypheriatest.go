// Package cypheriatest puts a test into Cypheria's integration mode.
package cypheriatest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/cypheria"
)

// Use writes an agents file listing agents and sets MAGPIE_AGENTS_FILE to
// it for the test. An agent without a command gets one that exists (a
// script printing "<id> 1.2.3"). It answers the folder the file is in.
func Use(t testing.TB, agents map[string]cypheria.Agent) string {
	t.Helper()
	dir := t.TempDir()
	for id, a := range agents {
		if a.Command == "" {
			a.Command = filepath.Join(dir, id+"-cli")
			if err := os.WriteFile(a.Command, []byte("#!/bin/sh\necho '"+id+" 1.2.3'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			agents[id] = a
		}
	}
	b, err := json.Marshal(map[string]any{"version": 1, "agents": agents})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "agents.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(cypheria.AgentsFileEnv, path)
	return dir
}
