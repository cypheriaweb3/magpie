package settings

import (
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/cypheria"
)

// MAGPIE_HOME holds all of magpie's own files (appdir): none in
// ~/.config/magpie or ~/.cache/magpie, nor in a portable data folder.
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
	if d, err := appdir.SystemCache(); err != nil || d != filepath.Join(home, "cache") {
		t.Fatalf("system cache at %s (%v)", d, err)
	}
	if d := appdir.WebView(); d != filepath.Join(home, "webview2") {
		t.Fatalf("webview at %s", d)
	}
}
