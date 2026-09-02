package bootstrap

import (
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingadmin"
	sheinclient "task-processor/internal/shein/client"
	"task-processor/internal/sheinlogin"
	sheinloginmanaged "task-processor/internal/sheinloginmanaged"
)

type BuildInput struct {
	Config            *config.Config
	StoreAPI          listingadmin.StoreAPI
	AccountRepository listingadmin.StoreRepository
}

type BuildResult struct {
	Handler sheinlogin.HTTPRouteHandler
	Module  kernelmodule.Module
	Service *sheinlogin.Service
	Close   func() error
}

func BuildHandler(input BuildInput) (*BuildResult, error) {
	if input.Config == nil {
		return nil, nil
	}

	if !HasRedisStoreConfig(input.Config) {
		return nil, nil
	}

	provider := buildAccountProvider(input.AccountRepository)
	if provider == nil {
		return nil, nil
	}

	svc, err := sheinlogin.NewService(input.Config.Platforms.Shein.LoginService, input.Config.EffectiveSheinCookieRedis(), input.Config.Browser, provider)
	if err != nil {
		return nil, err
	}
	svc.ConfigureRuntimeSheinAPIClients()
	if input.StoreAPI != nil {
		svc.ConfigureStoreSyncClientFactory(sheinloginmanaged.NewStoreSyncClientFactoryWithStoreAPI(input.StoreAPI))
		svc.ConfigureDuplicateStoreLookup(sheinloginmanaged.NewDuplicateStoreLookupWithStoreAPI(input.StoreAPI))
	}
	sheinclient.ConfigureLocalLoginRefresher(svc)
	handler := sheinlogin.NewHandler(svc)

	return &BuildResult{
		Handler: handler,
		Module:  sheinlogin.NewHTTPModule(handler),
		Service: svc,
		Close: func() error {
			return svc.Close()
		},
	}, nil
}

func buildAccountProvider(repository listingadmin.StoreRepository) sheinlogin.AccountProvider {
	if repository == nil {
		return nil
	}
	return sheinlogin.NewListingAdminAccountProvider(repository)
}
