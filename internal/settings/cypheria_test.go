package settings

import (
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/cypheria"
)

// MAGPIE_HOME holds all of magpie's own files: none in ~/.config/magpie
// or ~/.cache/magpie.
func TestMagpieHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv(cypheria.HomeEnv, home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if Path() != filepath.Join(home, "config", "settings.json") || Dir() != filepath.Join(home, "config") {
		t.Fatalf("settings at %s", Path())
	}
	if catalog.CachePath() != filepath.Join(home, "cache", "models.json") {
		t.Fatalf("catalog at %s", catalog.CachePath())
	}
}
