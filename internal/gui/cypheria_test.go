package gui

import (
	"context"
	"errors"
	"testing"

	"github.com/yetone/magpie/internal/update"
)

// upstream's tests of What's new ask a feed of their own
func init() { feedOff = false }

func TestWhatsNewAsksNoFeedForCypheria(t *testing.T) {
	feedOff = true
	t.Cleanup(func() { feedOff = false })
	t.Setenv("MAGPIE_UPDATE_FEED", "http://127.0.0.1:1/never-asked")
	if notes, err := fetchNotes(context.Background(), "0.1.1", "0.1.2", "en"); !errors.Is(err, update.ErrDisabled) || notes != nil {
		t.Fatalf("notes %v, err %v", notes, err)
	}
}
