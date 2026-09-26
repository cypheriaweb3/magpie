package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeHomeAndEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MAGPIE_CODEX_HOME", home)
	env := RuntimeEnv("codex", []string{
		"HOME=/host", "CODEX_HOME=/host/.codex", "XDG_CONFIG_HOME=/host/.config", "KEEP=yes",
	})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{"\nHOME=" + home + "\n", "\nCODEX_HOME=" + filepath.Join(home, ".codex") + "\n", "\nKEEP=yes\n"} {
		if !strings.Contains(joined, want) {
			t.Errorf("environment does not contain %q: %v", want, env)
		}
	}
	if strings.Contains(joined, "/host") {
		t.Fatalf("host roots leaked into isolated environment: %v", env)
	}
	if got := RuntimeHome("codex"); got != home {
		t.Fatalf("RuntimeHome = %q, want %q", got, home)
	}
}

func TestConfiguredCLIIsExact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cursor-agent")
	if err := os.WriteFile(path, []byte("cli"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAGPIE_CURSOR_CLI", path)
	if got, set := configuredCLI("cursor"); !set || got != path {
		t.Fatalf("configuredCLI = %q, %v", got, set)
	}
	t.Setenv("MAGPIE_CURSOR_CLI", "relative/cursor-agent")
	if got, set := configuredCLI("cursor"); !set || got != "" {
		t.Fatalf("relative configuredCLI = %q, %v", got, set)
	}
	if err := configuredCLIError("cursor", "Cursor CLI"); err == nil {
		t.Fatal("relative CLI did not produce a configuration error")
	}
}

func TestCLIAutoInstallSetting(t *testing.T) {
	t.Setenv(cliAutoInstallEnv, "0")
	if enabled, err := cliAutoInstallEnabled(); err != nil || enabled {
		t.Fatalf("enabled=%v err=%v", enabled, err)
	}
	t.Setenv(cliAutoInstallEnv, "invalid")
	if _, err := cliAutoInstallEnabled(); err == nil {
		t.Fatal("invalid setting was accepted")
	}
}

func TestProviderRuntimePaths(t *testing.T) {
	homes := map[string]string{}
	for _, agent := range runtimeHomeAgents {
		homes[agent] = filepath.Join(t.TempDir(), agent)
		t.Setenv(runtimeEnvName(agent, "HOME"), homes[agent])
	}
	wants := map[string]string{
		"claude credentials": filepath.Join(homes["claude"], ".claude", ".credentials.json"),
		"claude profile":     filepath.Join(homes["claude"], ".claude", ".claude.json"),
		"codex":              filepath.Join(homes["codex"], ".codex", "auth.json"),
		"copilot editors":    filepath.Join(homes["copilot"], ".config"),
		"copilot cli":        filepath.Join(homes["copilot"], ".copilot"),
		"devin":              filepath.Join(homes["devin"], ".local", "share", "devin", "credentials.toml"),
		"gemini":             filepath.Join(homes["gemini"], ".gemini"),
		"grok":               filepath.Join(homes["grok"], ".grok"),
		"zcode":              filepath.Join(homes["zcode"], ".zcode", "v2", "credentials.json"),
	}
	if runtime.GOOS == "windows" {
		wants["devin"] = filepath.Join(homes["devin"], "AppData", "Roaming", "devin", "credentials.toml")
	}
	got := map[string]string{
		"claude credentials": claudeCredentialsPath(),
		"claude profile":     claudeProfilePath(),
		"codex":              codexAuthPath(),
		"copilot editors":    copilotConfigDir(),
		"copilot cli":        copilotCLIHome(),
		"devin":              DevinCredentialsPath(),
		"gemini":             geminiDir(),
		"grok":               GrokHome(),
		"zcode":              zcodeCredentialsPath(),
	}
	for name, want := range wants {
		if got[name] != want {
			t.Errorf("%s path = %q, want %q", name, got[name], want)
		}
	}
}

func TestProviderCLIOverrides(t *testing.T) {
	finders := map[string]func() string{
		"claude": claudeExecutable,
		"codex":  codexExecutable,
		"cursor": CursorExecutable,
		"devin":  DevinExecutable,
		"grok":   GrokExecutable,
	}
	for agent, find := range finders {
		t.Run(agent, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), agent)
			if err := os.WriteFile(path, []byte("cli"), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv(runtimeEnvName(agent, "CLI"), path)
			if got := find(); got != path {
				t.Fatalf("executable = %q, want %q", got, path)
			}
		})
	}
}

func TestValidateRuntimeRejectsRelativeHome(t *testing.T) {
	t.Setenv("MAGPIE_CODEX_HOME", "relative")
	if err := ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "MAGPIE_CODEX_HOME") {
		t.Fatalf("expected CODEX home error, got %v", err)
	}
}

func TestRuntimeEnvLeavesDefaultUntouched(t *testing.T) {
	key := "MAGPIE_GEMINI_HOME"
	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
	in := []string{"HOME=/host"}
	out := RuntimeEnv("gemini", in)
	if len(out) != len(in) || out[0] != in[0] {
		t.Fatalf("default environment changed: %v", out)
	}
}
