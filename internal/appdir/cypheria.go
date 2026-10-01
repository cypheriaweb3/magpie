package appdir

import (
	"os"
	"path/filepath"
	"strings"
)

// HomeEnv names the folder Cypheria has magpie keep its own files in: its
// settings, providers, sign-ins and usage in config/, what it can fetch
// again in cache/. Set, it wins over a portable data folder and the XDG
// folders, so a magpie Cypheria runs shares nothing with the user's own.
const HomeEnv = "MAGPIE_HOME"

// MagpieHome is MAGPIE_HOME, "" when it is not set. internal/cypheria
// checks it is absolute before magpie serves.
func MagpieHome() string {
	h := strings.TrimSpace(os.Getenv(HomeEnv))
	if h == "" {
		return ""
	}
	return filepath.Clean(h)
}

// magpieHomeDir is the folder sub of MAGPIE_HOME; ok is false when it is
// not set.
func magpieHomeDir(sub string) (string, bool) {
	if h := MagpieHome(); h != "" {
		return filepath.Join(h, sub), true
	}
	return "", false
}
