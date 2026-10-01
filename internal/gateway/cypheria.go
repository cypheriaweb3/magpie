package gateway

// Claude Code run as Cypheria runs it, for a Claude subscription Cypheria
// manages (internal/cypheria): upstream's command outside integration mode.

import (
	"context"
	"errors"
	"os/exec"

	"github.com/yetone/magpie/internal/cypheria"
	"github.com/yetone/magpie/internal/proc"
)

// claudeCommand runs Claude Code at binary with args: in integration mode,
// the Claude Code Cypheria runs, in its folder. The caller sets cmd.Env
// through claudeEnv.
func claudeCommand(ctx context.Context, binary string, args ...string) *exec.Cmd {
	if cmd, managed := cypheria.Command(ctx, "claude", args...); managed {
		return cmd
	}
	return proc.CommandContext(ctx, binary, args...)
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

// managedClaude is the Claude Code Cypheria runs; ok is true in
// integration mode, where err says when Cypheria manages none.
func managedClaude() (path string, ok bool, err error) {
	p, ok := cypheria.Executable("claude")
	if ok && p == "" {
		err = errors.New("Claude Code isn't among the agents Cypheria manages")
	}
	return p, ok, err
}
