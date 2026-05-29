package bootstrap

import (
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingadmin"
	sheinclient "task-processor/internal/shein/client"
	"task-processor/internal/sheinlogin"
	sheinloginmanaged "task-processor/internal/sheinloginmanaged"
)

type AccountRepositoryBuilder func(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error)

type BuildInput struct {
	Config                   *config.Config
	ManagementClient         *management.ClientManager
	AccountRepositoryBuilder AccountRepositoryBuilder
}

type BuildResult struct {
	Handler sheinlogin.HTTPRouteHandler
	Module  kernelmodule.Module
	Service *sheinlogin.Service
	Close   func() error
}

func BuildHandler(input BuildInput) (*BuildResult, error) {
	if input.Config == nil || input.ManagementClient == nil {
		return nil, nil
	}

	redisCfg := input.Config.EffectiveSheinCookieRedis()
	if strings.TrimSpace(redisCfg.Host) == "" {
		return nil, nil
	}

	provider, repoCloser, err := buildAccountProvider(input.Config, input.AccountRepositoryBuilder)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		if repoCloser != nil {
			_ = repoCloser()
		}
		return nil, nil
	}

	svc, err := sheinlogin.NewService(input.Config.Platforms.Shein.LoginService, redisCfg, input.Config.Browser, provider)
	if err != nil {
		if repoCloser != nil {
			_ = repoCloser()
		}
		return nil, err
	}
	svc.ConfigureRuntimeSheinAPIClients()
	svc.ConfigureStoreSyncClientFactory(sheinloginmanaged.NewStoreSyncClientFactory(input.ManagementClient))
	svc.ConfigureDuplicateStoreLookup(sheinloginmanaged.NewDuplicateStoreLookup(input.ManagementClient))
	sheinclient.ConfigureLocalLoginRefresher(svc)
	handler := sheinlogin.NewHandler(svc)

	return &BuildResult{
		Handler: handler,
		Module:  sheinlogin.NewHTTPModule(handler),
		Service: svc,
		Close: func() error {
			closeErr := svc.Close()
			if repoCloser != nil {
				if err := repoCloser(); err != nil && closeErr == nil {
					closeErr = err
				}
			}
			return closeErr
		},
	}, nil
}

func buildAccountProvider(cfg *config.Config, builder AccountRepositoryBuilder) (sheinlogin.AccountProvider, func() error, error) {
	if cfg == nil {
		return nil, nil, nil
	}
	if builder == nil {
		return nil, nil, nil
	}

	localLogger := logrus.New()
	repo, closers, err := builder(cfg, localLogger)
	if err != nil {
		return nil, nil, err
	}
	if repo == nil {
		return nil, nil, nil
	}
	return sheinlogin.NewListingAdminAccountProvider(repo), joinClosers(closers), nil
}

func joinClosers(closers []func() error) func() error {
	if len(closers) == 0 {
		return nil
	}
	return func() error {
		var closeErr error
		for i := len(closers) - 1; i >= 0; i-- {
			if closers[i] == nil {
				continue
			}
			if err := closers[i](); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		return closeErr
	}
}
