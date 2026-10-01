package stats

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunDisabled(t *testing.T) {
	asked := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { asked <- struct{}{} }))
	defer srv.Close()
	t.Setenv("MAGPIE_STATS_HOST", srv.URL)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	done := make(chan struct{})
	go func() { Run("0.1.0", "serve"); close(done) }()
	select {
	case <-done:
	case <-asked:
		t.Fatal("a usage event was sent")
	case <-time.After(2 * time.Second):
		t.Fatal("Run kept running")
	}
}
