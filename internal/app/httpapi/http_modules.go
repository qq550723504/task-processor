package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func newCoreHTTPModule() httpModule {
	return httpModule{
		name: "system",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(buildCoreRouteDescriptors()...)
			return nil
		},
	}
}

func newProductHTTPModule(handlers httpModuleHandlers, built *productenrichhttpapi.Module, imageBuilt *productimagehttpapi.Module) kernelmodule.Module {
	if built != nil {
		return productenrichhttpapi.NewRuntimeModule(built, imageBuilt)
	}
	return productenrichhttpapi.NewHTTPModule(handlers.product, handlers.image)
}

func newAmazonListingHTTPModule(handlers httpModuleHandlers, built *amazonlistinghttpapi.Module) kernelmodule.Module {
	if built != nil {
		return amazonlistinghttpapi.NewRuntimeModule(built)
	}
	return amazonlistinghttpapi.NewHTTPModule(handlers.amazonListing)
}

func newListingKitHTTPModule(handlers httpModuleHandlers, built *listingkithttpapi.Module) kernelmodule.Module {
	if built != nil {
		return listingkithttpapi.NewRuntimeModule(built)
	}
	return listingkithttpapi.NewHTTPModule(handlers.listingKit)
}

func newListingKitStudioHTTPModule(handlers httpModuleHandlers, built *listingkithttpapi.Module) kernelmodule.Module {
	if built != nil {
		return nil
	}
	return listingkithttpapi.NewStudioHTTPModule(handlers.studioSession)
}
