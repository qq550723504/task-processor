package client

import (
	"strings"

	"task-processor/internal/core/config"
)

func ConfigureLoginAccountFromConfig(cfg *config.Config) {
	if cfg == nil {
		ConfigureLoginAccount("", "")
		return
	}

	loginService := cfg.Platforms.Shein.LoginService
	ConfigureLoginAccount("", strings.TrimSpace(loginService.Identifier))
}
