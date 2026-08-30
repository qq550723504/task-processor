package sdslogin

import (
	"path/filepath"

	"task-processor/internal/core/config"
	sdsclient "task-processor/internal/sds/client"
)

type BuildResult struct {
	Handler HTTPRouteHandler
	Service *Service
}

func BuildHandler(cfg *config.Config) (*BuildResult, error) {
	if cfg == nil {
		return nil, nil
	}
	redisCfg := cfg.EffectiveSDSAuthRedis()
	clientCfg := sdsclient.DefaultConfig()
	stateRoot := filepath.Dir(clientCfg.AuthFile)
	stateFiles := StateFiles{
		AuthStatePath:      clientCfg.AuthFile,
		SessionCookiesPath: clientCfg.CookieFile,
		BrowserStatePath:   filepath.Join(stateRoot, "browser_state.json"),
		LoginPayloadPath:   filepath.Join(stateRoot, "login_state.json"),
	}
	svc, err := NewService(cfg.Platforms.SDS.LoginService, redisCfg, cfg.Browser, stateFiles)
	if err != nil {
		return nil, err
	}
	return &BuildResult{
		Handler: NewHandler(svc),
		Service: svc,
	}, nil
}
