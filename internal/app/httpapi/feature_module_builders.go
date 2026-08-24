package httpapi

import (
	"github.com/sirupsen/logrus"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/core/config"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	"task-processor/internal/sourceaccount"
)

type sourceAccountRepositoryBuilder func(*config.Config, *logrus.Logger) (sourceaccount.Repository, []func() error, error)

var buildSourceAccountRepository = listingkithttpapi.BuildSourceAccountRepository

type productModuleBuilder func(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error)

type imageModuleBuilder func(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error)

type amazonListingModuleBuilder func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)

type listingKitModuleBuilder func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error)

func buildProductModuleResult(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error) {
	return productenrichhttpapi.BuildRuntimeModule(input)
}

func buildImageModuleResult(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error) {
	return productimagehttpapi.BuildRuntimeModule(input)
}

func buildAmazonListingModuleResult(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error) {
	return amazonlistinghttpapi.BuildRuntimeModule(input)
}

func buildListingKitModuleResult(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
	return listingkithttpapi.BuildRuntimeModule(input)
}
