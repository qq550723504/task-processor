package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

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
