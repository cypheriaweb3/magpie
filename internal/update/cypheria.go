package update

import "errors"

// Disabled turns self-updating off in Cypheria's builds of magpie. They are
// released from cypheriaweb3/magpie-releases and replaced by whatever runs
// them; the feed here describes upstream's builds, which would put back a
// magpie without Cypheria's changes. Nothing asks the feed while it is set:
// neither for a release (update_cli.go, internal/gui's updater) nor for
// the notes of one (internal/gui's What's new).
const Disabled = true

// ErrDisabled answers what would ask the feed.
var ErrDisabled = errors.New("magpie is Cypheria's build: it is updated with Cypheria, not by itself")
