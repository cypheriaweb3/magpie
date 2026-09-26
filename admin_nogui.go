//go:build nogui

package main

import (
	"fmt"

	"github.com/yetone/magpie/internal/gateway"
)

func prepareAdmin(*gateway.Server) (adminServer, error) {
	_, enabled, err := loadAdminConfig()
	if err != nil || !enabled {
		return nil, err
	}
	return nil, fmt.Errorf("the admin API configured by %s is unavailable in a nogui build", adminAddrEnv)
}
