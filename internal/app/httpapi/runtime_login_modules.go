package httpapi

import (
	"task-processor/internal/listingadmin"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	sheinclient "task-processor/internal/shein/client"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
)

type sheinLoginModuleResult = sheinloginbootstrap.BuildResult

type sdsLoginModuleResult = sdsloginbootstrap.BuildResult

type sheinLoginModuleBuilder func(deps *runtimeDeps) (*sheinLoginModuleResult, func() error, error)

type sdsLoginModuleBuilder func(deps *runtimeDeps) (*sdsLoginModuleResult, func() error, error)

type loginFeatureSet struct {
	sheinLoginResult *sheinLoginModuleResult
	sdsLoginResult   *sdsLoginModuleResult
}

type loginFeatureBuilder struct {
	buildSheinLogin sheinLoginModuleBuilder
	buildSDSLogin   sdsLoginModuleBuilder
}

func (b loginFeatureBuilder) build(deps *runtimeDeps) (loginFeatureSet, error) {
	var features loginFeatureSet

	sheinLoginResult, sheinLoginCloser, err := b.buildSheinLogin(deps)
	if err != nil {
		return features, err
	}
	deps.addClosers(sheinLoginCloser)
	features.sheinLoginResult = sheinLoginResult

	sdsLoginResult, sdsLoginCloser, err := b.buildSDSLogin(deps)
	if err != nil {
		return features, err
	}
	deps.addClosers(sdsLoginCloser)
	deps.attachSDSLoginResult(sdsLoginResult)
	features.sdsLoginResult = sdsLoginResult

	return features, nil
}

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
		StoreAPI:                 sheinLoginStoreAPI(deps),
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

func sheinLoginStoreAPI(deps *runtimeDeps) listingadmin.StoreAPI {
	if deps == nil || deps.shared == nil || deps.shared.sharedResources == nil {
		return nil
	}
	return deps.shared.sharedResources.StoreAPI
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
