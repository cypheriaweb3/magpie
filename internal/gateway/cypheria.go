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

// claudeCommand runs Claude Code at binary with env and args. In
// integration mode it is the Claude Code Cypheria runs, in its
// environment with what env changes from magpie's own over it.
func claudeCommand(ctx context.Context, binary string, env []string, args ...string) *exec.Cmd {
	if cmd, managed := cypheria.Command(ctx, "claude", args...); managed {
		cmd.Env = cypheria.Rebase("claude", env)
		return cmd
	}
	cmd := proc.CommandContext(ctx, binary, args...)
	cmd.Env = env
	return cmd
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
