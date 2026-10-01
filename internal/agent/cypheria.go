package agent

// Where an agent Cypheria manages keeps its files: in the folder its own
// variable names in the agents file (internal/cypheria), as the agent finds
// it. cypheria.Native is "" outside integration mode, and for an agent of
// a WSL distro (place.spell), so these are upstream's paths there.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/cypheria"
)

// scoped is the agents magpie knows in integration mode: the listed ones.
func scoped(all []*Agent) []*Agent {
	var out []*Agent
	for _, a := range all {
		if cypheria.Listed(a.ID) {
			out = append(out, a)
		}
	}
	return out
}

// envOf is variable k as agent at p sees it: from its environment in the
// agents file for an agent Cypheria manages on this machine, else
// p.getenv's. It stands in for p.getenv where upstream reads the agent's
// own variable.
func (p place) envOf(agent, k string) string {
	if v := cypheria.Native(agent, k); v != "" && p.spell == nil {
		return v
	}
	return p.getenv(k)
}

func claudeHomeOf(at place) string {
	if d := cypheria.Native("claude", "CLAUDE_CONFIG_DIR"); d != "" && at.spell == nil {
		return d
	}
	return filepath.Join(at.home, ".claude")
}

func codexHomeOf(at place) string {
	if d := cypheria.Native("codex", "CODEX_HOME"); d != "" && at.spell == nil {
		return d
	}
	return filepath.Join(at.home, ".codex")
}

// geminiHome is Gemini CLI's ~/.gemini, under GEMINI_CLI_HOME when set.
func geminiHomeOf(home string) string {
	if d := cypheria.Native("gemini", "GEMINI_CLI_HOME"); d != "" {
		return filepath.Join(d, ".gemini")
	}
	return filepath.Join(home, ".gemini")
}

// agyHome is the ~/.gemini Antigravity's CLI keeps its folders in, which
// GEMINI_HOME names.
func agyHomeOf(home string) string {
	if d := cypheria.Native("agy", "GEMINI_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".gemini")
}

func cursorHomeOf(home string) string {
	if d := cypheria.Native("cursor", "CURSOR_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(home, ".cursor")
}

func copilotHomeOf(home string) string {
	if d := cypheria.Native("copilot", "COPILOT_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".copilot")
}

// configHome is $XDG_CONFIG_HOME as agent sees it: its own in the agents
// file, else cfg.
func configHomeOf(agent, cfg string) string {
	if d := cypheria.Native(agent, "XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return cfg
}

// dataHome is $XDG_DATA_HOME as agent sees it, else ~/.local/share.
func dataHomeOf(agent, home string) string {
	if d := cypheria.Native(agent, "XDG_DATA_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".local", "share")
}

// gooseConfig is Goose's config.yaml: under GOOSE_PATH_ROOT's config
// folder when it sets one, else in its config home's goose folder.
func gooseConfigOf(cfg string) (string, bool) {
	if root := cypheria.Native("goose", "GOOSE_PATH_ROOT"); root != "" {
		return filepath.Join(root, "config", "config.yaml"), true
	}
	if d := cypheria.Native("goose", "XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "goose", "config.yaml"), true
	}
	return filepath.Join(cfg, "goose", "config.yaml"), false
}

// managedCLI is the version of the CLI Cypheria runs the agent with, asked
// the way Cypheria runs it. Cypheria installs and updates it, so magpie
// offers no update.
func (a *Agent) managedCLI() (CLI, bool) {
	spec, ok := cypheria.Spec(a.ID)
	if _, known := cliSpecs[a.ID]; !ok || !known {
		return CLI{}, false
	}
	st, err := os.Stat(spec.Command)
	if err != nil {
		return CLI{}, true
	}
	key := "cypheria:" + a.ID + ":" + spec.Command + "\x00" + strings.Join(spec.Args, "\x00")
	stamp := fmt.Sprint(spec.Command, st.Size(), st.ModTime().UnixNano(), spec.Args)
	v := versions.get(key, stamp, 24*time.Hour, 10*time.Minute, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd, _ := cypheria.Probe(ctx, a.ID, "--version")
		cmd.Stdin = nil
		out, _ := cmd.Output()
		if v := parseVersion(string(out)); v != "" {
			return v, nil
		}
		return "", errors.New("no version")
	})
	return CLI{Version: v}, true
}
