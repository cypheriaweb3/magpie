package plugin

import "github.com/yetone/magpie/internal/cypheria"

// pluginsOff: plugins are never added, listed or run when magpie runs for
// Cypheria. A plugin is another party's code, which signs in to a
// subscription of no agent Cypheria manages; nothing is downloaded for it.
func pluginsOff() bool { return !cypheria.PluginsAllowed() }

// errOff answers a plugin operation when they are off.
var errOff = cypheria.ErrPlugins
