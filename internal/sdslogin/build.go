package sdslogin

import "task-processor/internal/core/config"

type BuildResult struct {
	Handler HTTPRouteHandler
	Service *Service
}

func BuildHandler(cfg *config.Config) (*BuildResult, error) {
	if cfg == nil {
		return nil, nil
	}
	redisCfg := cfg.EffectiveSDSAuthRedis()
	svc, err := NewService(cfg.Platforms.SDS.LoginService, redisCfg, cfg.Browser)
	if err != nil {
		return nil, err
	}
	return &BuildResult{
		Handler: NewHandler(svc),
		Service: svc,
	}, nil
}
