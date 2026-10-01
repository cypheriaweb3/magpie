package wslrun

import "github.com/yetone/magpie/internal/cypheria"

// cypheriaOff: a magpie Cypheria runs uses only the agents' CLIs Cypheria
// manages on this machine (internal/cypheria), never one found in a WSL
// distro.
func cypheriaOff() bool { return cypheria.Active() }
