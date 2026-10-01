package gui

import "github.com/yetone/magpie/internal/update"

// feedOff keeps What's new from asking upstream's release feed for notes:
// Cypheria's builds never update themselves (update.Disabled), and the
// feed's notes are of upstream's builds. A var so upstream's tests of the
// notes can run.
var feedOff = update.Disabled
