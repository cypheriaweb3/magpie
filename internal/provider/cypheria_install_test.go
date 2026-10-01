package provider

import (
	"context"
	"strings"
	"testing"
)

// The installer's own tests (install_test.go) install; Cypheria's builds
// never do, which TestSignInNeverInstallsCLI checks.
func init() { installCLIs = true }

func TestSignInNeverInstallsCLI(t *testing.T) {
	old, oldExe, oldInstaller := installCLIs, DevinExecutable, runInstaller
	installCLIs = false
	DevinExecutable = func() string { return "" }
	ran := false
	runInstaller = func(context.Context, agentCLI) ([]byte, error) {
		ran = true
		return nil, nil
	}
	t.Cleanup(func() { installCLIs, DevinExecutable, runInstaller = old, oldExe, oldInstaller })

	_, err := StartSignIn("devin")
	if err == nil || !strings.Contains(err.Error(), "Devin CLI is not installed") {
		t.Fatalf("got %v, want the sign-in refused for the missing CLI", err)
	}
	if ran {
		t.Fatal("the installer ran")
	}
}
