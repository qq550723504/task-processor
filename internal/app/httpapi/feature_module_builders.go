package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type productModuleResult = productenrichhttpapi.Module

type imageModuleResult = productimagehttpapi.Module

type amazonListingModuleResult = amazonlistinghttpapi.Module

type listingKitModuleResult = listingkithttpapi.Module

type productModuleBuilder func(input productenrichhttpapi.RuntimeBuildInput) (*productModuleResult, error)

type imageModuleBuilder func(input productimagehttpapi.RuntimeBuildInput) (*imageModuleResult, error)

type amazonListingModuleBuilder func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonListingModuleResult, error)

type listingKitModuleBuilder func(input listingkithttpapi.RuntimeBuildInput) (*listingKitModuleResult, error)

func buildProductModuleResult(input productenrichhttpapi.RuntimeBuildInput) (*productModuleResult, error) {
	return productenrichhttpapi.BuildRuntimeModule(input)
}

func buildImageModuleResult(input productimagehttpapi.RuntimeBuildInput) (*imageModuleResult, error) {
	return productimagehttpapi.BuildRuntimeModule(input)
}

func buildAmazonListingModuleResult(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonListingModuleResult, error) {
	return amazonlistinghttpapi.BuildRuntimeModule(input)
}

func buildListingKitModuleResult(input listingkithttpapi.RuntimeBuildInput) (*listingKitModuleResult, error) {
	return listingkithttpapi.BuildRuntimeModule(input)
}
