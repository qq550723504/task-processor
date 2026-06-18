package httpapi

import (
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	sheinclient "task-processor/internal/shein/client"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
)

type sheinLoginModuleResult = sheinloginbootstrap.BuildResult

type sdsLoginModuleResult = sdsloginbootstrap.BuildResult

type sheinLoginModuleBuilder func(deps *runtimeDeps) (*sheinLoginModuleResult, func() error, error)

type sdsLoginModuleBuilder func(deps *runtimeDeps) (*sdsLoginModuleResult, func() error, error)

func configureSheinLoginAccount(deps *runtimeDeps) {
	if deps == nil || deps.shared == nil {
		return
	}
	sheinclient.ConfigureLoginAccountFromConfig(deps.shared.cfg)
}

func buildSheinLoginModuleResult(deps *runtimeDeps) (*sheinLoginModuleResult, func() error, error) {
	if deps == nil {
		return nil, nil, nil
	}

	result, err := sheinloginbootstrap.BuildHandler(sheinloginbootstrap.BuildInput{
		Config:                   deps.shared.cfg,
		ManagementClient:         deps.managementClient(),
		AccountRepositoryBuilder: listingkithttpapi.BuildListingAdminStoreRepository,
	})
	if err != nil {
		return nil, nil, err
	}
	if result == nil {
		return nil, nil, nil
	}
	return result, result.Close, nil
}

func buildSDSLoginModuleResult(deps *runtimeDeps) (*sdsLoginModuleResult, func() error, error) {
	if deps == nil {
		return nil, nil, nil
	}
	result, err := sdsloginbootstrap.BuildHandler(deps.shared.cfg)
	if err != nil {
		return nil, nil, err
	}
	if result == nil || result.StatusProvider == nil {
		return nil, nil, nil
	}
	return result, nil, nil
}
