package provider

// Runtime overrides let a process manager give each subscription provider an
// isolated user home and an exact CLI without changing the provider's saved
// format. They are deliberately process settings: changing a home while
// requests or a sign-in are in flight would mix two accounts.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var runtimeHomeAgents = []string{"claude", "codex", "commandcode", "copilot", "cursor", "devin", "gemini", "grok", "kiro", "workbuddy", "zcode"}

func runtimeEnvName(agent, suffix string) string {
	return "MAGPIE_" + strings.ToUpper(strings.ReplaceAll(agent, "-", "_")) + "_" + suffix
}

func runtimeHomeOverride(agent string) (string, bool) {
	raw, ok := os.LookupEnv(runtimeEnvName(agent, "HOME"))
	if !ok {
		return "", false
	}
	return filepath.Clean(strings.TrimSpace(raw)), true
}

// RuntimeHome is the virtual user home used to find one subscription
// provider's own files. With no override it is the real user's home.
func RuntimeHome(agent string) string {
	if home, ok := runtimeHomeOverride(agent); ok {
		return home
	}
	home, _ := os.UserHomeDir()
	return home
}

func runtimeHomeConfigured(agent string) bool {
	_, ok := runtimeHomeOverride(agent)
	return ok
}

func configuredCLI(agent string) (string, bool) {
	raw, ok := os.LookupEnv(runtimeEnvName(agent, "CLI"))
	if !ok {
		return "", false
	}
	path := strings.TrimSpace(raw)
	if path == "" || !filepath.IsAbs(path) || !executableFile(path) {
		return "", true
	}
	return filepath.Clean(path), true
}

func configuredCLIError(agent, name string) error {
	raw, ok := os.LookupEnv(runtimeEnvName(agent, "CLI"))
	if !ok {
		return nil
	}
	path := strings.TrimSpace(raw)
	if path == "" {
		return fmt.Errorf("%s is empty; set it to the absolute path of %s", runtimeEnvName(agent, "CLI"), name)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s must be an absolute path, not %q", runtimeEnvName(agent, "CLI"), path)
	}
	if !executableFile(path) {
		return fmt.Errorf("%s does not name an executable file: %s", runtimeEnvName(agent, "CLI"), path)
	}
	return nil
}

func executableFile(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return runtime.GOOS == "windows" || st.Mode().Perm()&0o111 != 0
}

// RuntimeEnv gives a provider CLI the same isolated home Magpie reads. It
// removes host-specific roots first so XDG or Windows variables cannot lead
// the CLI back to the real user's account.
func RuntimeEnv(agent string, env []string) []string {
	home, isolated := runtimeHomeOverride(agent)
	if !isolated {
		return env
	}
	if env == nil {
		env = os.Environ()
	}
	blocked := map[string]bool{
		"HOME": true, "USERPROFILE": true, "HOMEDRIVE": true, "HOMEPATH": true,
		"APPDATA": true, "LOCALAPPDATA": true,
		"XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_CACHE_HOME": true,
		"CLAUDE_CONFIG_DIR": true, "CODEX_HOME": true, "COPILOT_HOME": true, "GROK_HOME": true,
	}
	out := make([]string, 0, len(env)+8)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !blocked[strings.ToUpper(key)] {
			out = append(out, entry)
		}
	}
	out = append(out, "HOME="+home)
	if runtime.GOOS == "windows" {
		out = append(out,
			"USERPROFILE="+home,
			"APPDATA="+filepath.Join(home, "AppData", "Roaming"),
			"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"))
	} else {
		out = append(out,
			"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
			"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
			"XDG_CACHE_HOME="+filepath.Join(home, ".cache"))
	}
	switch agent {
	case "claude":
		out = append(out, "CLAUDE_CONFIG_DIR="+filepath.Join(home, ".claude"))
	case "codex":
		out = append(out, "CODEX_HOME="+filepath.Join(home, ".codex"))
	case "copilot":
		out = append(out, "COPILOT_HOME="+filepath.Join(home, ".copilot"))
	case "grok":
		out = append(out, "GROK_HOME="+filepath.Join(home, ".grok"))
	}
	return out
}

// ValidateRuntime checks all process-level overrides before a server starts,
// so a typo cannot silently fall back to a working directory or host CLI.
func ValidateRuntime() error {
	for _, agent := range runtimeHomeAgents {
		if raw, ok := os.LookupEnv(runtimeEnvName(agent, "HOME")); ok {
			home := strings.TrimSpace(raw)
			if home == "" || !filepath.IsAbs(home) {
				return fmt.Errorf("%s must be a non-empty absolute path", runtimeEnvName(agent, "HOME"))
			}
		}
	}
	for agent, name := range map[string]string{
		"claude": "Claude Code", "codex": "Codex CLI", "cursor": "Cursor CLI",
		"devin": "Devin CLI", "grok": "Grok Build", "kiro": "Kiro CLI",
	} {
		if err := configuredCLIError(agent, name); err != nil {
			return err
		}
	}
	return nil
}
