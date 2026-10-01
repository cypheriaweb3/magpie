package provider

// The subscriptions of the agents Cypheria manages, read where those
// agents keep them and refreshed with their CLIs run as Cypheria runs them
// (internal/cypheria). Outside integration mode every function here is
// upstream's behavior.

import (
	"context"
	"fmt"
	"os/exec"
	"slices"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/proc"
)

// cliCommand runs agent's CLI found at path with env (nil: magpie's own),
// as upstream runs it with proc.CommandContext. In integration mode it runs
// as Cypheria runs the agent: its command, folder and environment, with
// what env changes from magpie's own over that.
func cliCommand(ctx context.Context, agent, path string, env []string, args ...string) *exec.Cmd {
	if cmd, managed := cypheria.Command(ctx, agent, args...); managed {
		cmd.Env = cypheria.Rebase(agent, env)
		return cmd
	}
	cmd := proc.CommandContext(ctx, path, args...)
	if env != nil {
		cmd.Env = env
	}
	return cmd
}

// cliProbe is cliCommand for a CLI asked something, as upstream runs it
// with proc.ProbeContext.
func cliProbe(ctx context.Context, agent, path string, env []string, args ...string) *exec.Cmd {
	if cmd, managed := cypheria.Probe(ctx, agent, args...); managed {
		cmd.Env = cypheria.Rebase(agent, env)
		return cmd
	}
	cmd := proc.ProbeContext(ctx, path, args...)
	if env != nil {
		cmd.Env = env
	}
	return cmd
}

// claudeEnv is env, made from magpie's own, for the Claude Code Cypheria
// manages: its environment with env's changes over it (cypheria.Rebase).
// Outside integration mode it is env.
func claudeEnv(env []string) []string {
	if _, managed := cypheria.Spec("claude"); managed {
		return cypheria.Rebase("claude", env)
	}
	return env
}

// managedCLI is the CLI Cypheria runs agent with; ok is true in
// integration mode, where an agent that isn't listed has none.
func managedCLI(agent string) (string, bool) { return cypheria.Executable(agent) }

// claudeKeychainService is the keychain item Claude Code keeps its sign-in
// in: "Claude Code-credentials", and for a CLAUDE_CONFIG_DIR of its own the
// first 8 hex digits of that folder's SHA-256 after it, as Claude Code
// names it. Only an agent Cypheria manages gets the suffix here; upstream
// reads the plain item whatever CLAUDE_CONFIG_DIR says.
func claudeKeychainService() string {
	const service = "Claude Code-credentials"
	if dir := cypheria.Native("claude", "CLAUDE_CONFIG_DIR"); dir != "" {
		return claudeDirService(dir)
	}
	return service
}

// outOfScope reports whether id is a subscription magpie leaves alone in
// integration mode: one of an agent Cypheria doesn't manage. Subscriptions
// are the providers magpie signs in to as an agent or app does
// (accountIDs); providers with a key are not limited, and plugins'
// providers are never there (internal/plugin is off).
func outOfScope(id string) bool {
	return cypheria.Active() && slices.Contains(accountIDs, id) && !cypheria.SubscriptionAllowed(id)
}

// hideOutOfScope marks the subscriptions out of scope as hidden, for the
// quota fetches not to ask them.
func hideOutOfScope(hidden map[string]bool) {
	for _, id := range accountIDs {
		if outOfScope(id) {
			hidden[id] = true
		}
	}
}

// errOutOfScope answers an operation on a subscription out of scope.
func errOutOfScope(id string) error {
	return fmt.Errorf("%s is not among the subscriptions of the agents Cypheria manages", id)
}

// scopedAccounts is Accounts in integration mode: the subscriptions of the
// agents Cypheria manages alone, so no other tool's sign-in is even read.
func scopedAccounts(home, cfg string) []Provider {
	var out []Provider
	add := func(id string, account func() (Provider, bool)) {
		if outOfScope(id) {
			return
		}
		if p, ok := account(); ok {
			out = append(out, p)
		}
	}
	add("claude", claudeAccount)
	add("codex", func() (Provider, bool) { return codexAccount(home) })
	add("copilot", func() (Provider, bool) { return copilotAccount(cfg) })
	add("cursor", cursorAccount)
	add("grok", grokAccount)
	add("devin", devinAccount)
	for _, agent := range []string{"gemini", "antigravity"} {
		add(agent, func() (Provider, bool) { return googleAccountOf(agent) })
	}
	return out
}
