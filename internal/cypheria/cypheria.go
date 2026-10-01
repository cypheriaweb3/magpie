// Package cypheria is how Cypheria runs magpie: as a server of its own,
// over the agents Cypheria manages rather than the ones on this machine.
//
// MAGPIE_HOME moves magpie's own files (its settings, providers, sign-ins,
// usage and caches) out of ~/.config/magpie and ~/.cache/magpie, so it
// shares nothing with a magpie the user runs.
//
// MAGPIE_AGENTS_FILE names the agents file Cypheria writes, which turns on
// integration mode. The file lists each agent Cypheria manages, keyed by
// magpie's agent id, with the command, arguments, working folder and
// environment Cypheria runs it with. In integration mode magpie:
//
//   - knows only the listed agents, and finds each one's settings, sessions
//     and sign-in through that agent's environment (CODEX_HOME,
//     CLAUDE_CONFIG_DIR, its own XDG folders…), the way the agent itself does;
//   - runs an agent's CLI exactly as Cypheria does, and never installs or
//     updates it;
//   - signs in only to the subscriptions of the listed agents, and runs no
//     plugins. Providers with a key are not limited.
//
// Without MAGPIE_AGENTS_FILE magpie behaves as upstream's does.
package cypheria

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/proc"
)

const (
	// HomeEnv names the folder magpie keeps its own files in.
	HomeEnv = appdir.HomeEnv
	// AgentsFileEnv names the agents file; set, it is integration mode.
	AgentsFileEnv = "MAGPIE_AGENTS_FILE"
)

// Agents are the agents an agents file may list: those Cypheria manages,
// by magpie's id.
var Agents = []string{"agy", "claude", "cline", "codex", "copilot", "cursor", "devin", "gemini", "goose", "grok", "opencode", "pi"}

// subscriptionAgent is the agent each subscription magpie signs in to
// belongs to. A subscription of an agent that isn't listed, and every
// other subscription, is off in integration mode.
var subscriptionAgent = map[string]string{
	"claude":      "claude",
	"codex":       "codex",
	"copilot":     "copilot",
	"cursor":      "cursor",
	"devin":       "devin",
	"gemini":      "gemini",
	"antigravity": "agy",
	"grok":        "grok",
}

// Agent is how Cypheria runs one agent.
type Agent struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type agentsFile struct {
	Version int              `json:"version"`
	Agents  map[string]Agent `json:"agents"`
}

// Home is MAGPIE_HOME, "" when it is not set. appdir puts magpie's own
// folders under it.
func Home() string { return appdir.MagpieHome() }

// Active reports whether magpie runs in integration mode.
func Active() bool { return strings.TrimSpace(os.Getenv(AgentsFileEnv)) != "" }

var loaded struct {
	sync.Mutex
	path   string
	mod    time.Time
	size   int64
	agents map[string]Agent
	ok     bool
}

// Validate checks MAGPIE_HOME and the agents file before magpie serves,
// so a mistake stops it instead of leaving it on the machine's own agents.
func Validate() error {
	if h := strings.TrimSpace(os.Getenv(HomeEnv)); h != "" && !filepath.IsAbs(h) {
		return fmt.Errorf("%s must be an absolute path, not %q", HomeEnv, h)
	}
	if !Active() {
		return nil
	}
	path := strings.TrimSpace(os.Getenv(AgentsFileEnv))
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s must be an absolute path", AgentsFileEnv)
	}
	agents, st, err := read(path)
	if err != nil {
		return err
	}
	loaded.Lock()
	defer loaded.Unlock()
	loaded.path, loaded.mod, loaded.size, loaded.agents, loaded.ok = path, st.ModTime(), st.Size(), agents, true
	return nil
}

// read parses and checks an agents file.
func read(path string) (map[string]Agent, os.FileInfo, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", AgentsFileEnv, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", AgentsFileEnv, err)
	}
	var f agentsFile
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, nil, fmt.Errorf("%s %s: %w", AgentsFileEnv, path, err)
	}
	if f.Version != 1 {
		return nil, nil, fmt.Errorf("%s %s: version %d, want 1", AgentsFileEnv, path, f.Version)
	}
	known := map[string]bool{}
	for _, id := range Agents {
		known[id] = true
	}
	for id, a := range f.Agents {
		if !known[id] {
			return nil, nil, fmt.Errorf("%s %s: unknown agent %q (one of %s)", AgentsFileEnv, path, id, strings.Join(Agents, ", "))
		}
		if !filepath.IsAbs(a.Command) {
			return nil, nil, fmt.Errorf("%s %s: %s's command must be an absolute path, not %q", AgentsFileEnv, path, id, a.Command)
		}
		if st, err := os.Stat(a.Command); err != nil || st.IsDir() {
			return nil, nil, fmt.Errorf("%s %s: %s's command %s is not a file", AgentsFileEnv, path, id, a.Command)
		}
		if a.Cwd != "" && !filepath.IsAbs(a.Cwd) {
			return nil, nil, fmt.Errorf("%s %s: %s's cwd must be an absolute path, not %q", AgentsFileEnv, path, id, a.Cwd)
		}
		for k := range a.Env {
			if k == "" || strings.ContainsAny(k, "=\x00") {
				return nil, nil, fmt.Errorf("%s %s: %s has an invalid environment name %q", AgentsFileEnv, path, id, k)
			}
		}
	}
	if f.Agents == nil {
		f.Agents = map[string]Agent{}
	}
	return f.Agents, st, nil
}

// agents is the agents file as it is now: read again when it has changed,
// as Cypheria rewrites it when it installs or removes an agent. A file
// that has become unreadable or wrong keeps the agents last read.
func agents() map[string]Agent {
	if !Active() {
		return nil
	}
	loaded.Lock()
	defer loaded.Unlock()
	path := strings.TrimSpace(os.Getenv(AgentsFileEnv))
	st, err := os.Stat(path)
	if loaded.ok && loaded.path == path && err == nil && st.ModTime().Equal(loaded.mod) && st.Size() == loaded.size {
		return loaded.agents
	}
	a, st, err := read(path)
	if err != nil {
		if loaded.ok {
			log.Printf("cypheria: keeping the agents read before: %v", err)
			return loaded.agents
		}
		log.Printf("cypheria: no agents: %v", err)
		return map[string]Agent{}
	}
	loaded.path, loaded.mod, loaded.size, loaded.agents, loaded.ok = path, st.ModTime(), st.Size(), a, true
	return a
}

// Listed reports whether the agents file lists agent.
func Listed(agent string) bool {
	_, ok := agents()[agent]
	return ok
}

// Listing is the ids the agents file lists, sorted.
func Listing() []string {
	var out []string
	for id := range agents() {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Spec is how Cypheria runs agent; ok is false outside integration mode or
// when the agent isn't listed.
func Spec(agent string) (Agent, bool) {
	a, ok := agents()[agent]
	return a, ok
}

// AgentAllowed reports whether magpie may know agent: every one outside
// integration mode, the listed ones in it.
func AgentAllowed(agent string) bool { return !Active() || Listed(agent) }

// SubscriptionAllowed reports whether magpie may sign in to, list and use
// the subscription provider id: every one outside integration mode; in it,
// only those of a listed agent.
func SubscriptionAllowed(id string) bool {
	if !Active() {
		return true
	}
	agent, ok := subscriptionAgent[id]
	return ok && Listed(agent)
}

// PluginsAllowed reports whether plugins may be added, signed in to and
// run: never in integration mode.
func PluginsAllowed() bool { return !Active() }

// ErrPlugins is the answer to a plugin operation in integration mode.
var ErrPlugins = errors.New("plugins are off when magpie runs for Cypheria")

// Getenv is variable key as agent sees it: from its environment in the
// agents file, else magpie's own (appdir.Getenv). It stands in for
// os.Getenv and appdir.Getenv where magpie already reads the agent's own
// variable.
func Getenv(agent, key string) string {
	if a, ok := Spec(agent); ok {
		if v, ok := a.Env[key]; ok {
			return v
		}
	}
	return appdir.Getenv(key)
}

// Native is variable key from agent's environment in the agents file, ""
// outside integration mode or when it isn't set there. It is for where
// upstream doesn't read the agent's variable at all, so a variable in
// magpie's own environment keeps not counting.
func Native(agent, key string) string {
	if a, ok := Spec(agent); ok {
		return a.Env[key]
	}
	return ""
}

// Environ is base (magpie's environment when nil) with agent's environment
// from the agents file over it.
func Environ(agent string, base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	a, ok := Spec(agent)
	if !ok || len(a.Env) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(a.Env))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if _, over := a.Env[k]; !over {
			out = append(out, kv)
		}
	}
	keys := make([]string, 0, len(a.Env))
	for k := range a.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+a.Env[k])
	}
	return out
}

// Command runs agent's CLI as Cypheria does: its command and arguments
// before args, in its environment over magpie's, in its folder. A caller
// adjusts cmd.Env and cmd.Dir after, for what it runs the CLI for. ok is
// false when agent isn't listed, for the caller to run it its own way.
func Command(ctx context.Context, agent string, args ...string) (cmd *exec.Cmd, ok bool) {
	return run(proc.CommandContext, ctx, agent, args)
}

// Probe is Command for a CLI asked something (proc.ProbeContext): ctx
// must end, and ends whatever the CLI started.
func Probe(ctx context.Context, agent string, args ...string) (cmd *exec.Cmd, ok bool) {
	return run(proc.ProbeContext, ctx, agent, args)
}

func run(start func(context.Context, string, ...string) *exec.Cmd, ctx context.Context, agent string, args []string) (*exec.Cmd, bool) {
	a, ok := Spec(agent)
	if !ok {
		return nil, false
	}
	cmd := start(ctx, a.Command, append(append([]string{}, a.Args...), args...)...)
	cmd.Env = Environ(agent, nil)
	cmd.Dir = a.Cwd
	return cmd, true
}

// Executable is the command Cypheria runs agent with. ok is true in
// integration mode, where an agent that isn't listed has none: magpie
// never falls back to one of its own on PATH.
func Executable(agent string) (path string, ok bool) {
	if !Active() {
		return "", false
	}
	a, _ := Spec(agent)
	return a.Command, true
}

// Rebase is env, an environment a caller made from magpie's own (adding,
// changing or dropping variables for what it runs), made from agent's
// instead: agent's environment from the agents file with the caller's
// changes over it. The caller's changes win, as they are why it runs the
// CLI — another account's GROK_HOME, say. nil env is agent's as it is.
func Rebase(agent string, env []string) []string {
	base := Environ(agent, nil)
	if env == nil {
		return base
	}
	own := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		own[k] = v
	}
	given := map[string]string{}
	var order []string
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if _, dup := given[k]; !dup {
			order = append(order, k)
		}
		given[k] = v
	}
	out := make([]string, 0, len(base)+len(env))
	seen := map[string]bool{}
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		seen[k] = true
		gv, kept := given[k]
		ov, mine := own[k]
		switch {
		case kept && mine && gv == ov:
			out = append(out, kv) // the caller left magpie's value: the agent's
		case kept:
			out = append(out, k+"="+gv) // the caller's own value
		case mine:
			// the caller dropped magpie's variable; an agent's own value
			// of it goes too, as the caller wants none
		default:
			out = append(out, kv) // the agent's alone
		}
	}
	for _, k := range order {
		if !seen[k] {
			out = append(out, k+"="+given[k])
		}
	}
	return out
}
