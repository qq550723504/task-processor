package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
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

func newProductHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "product",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(productenrichhttpapi.AppendProductRouteDescriptors(nil, handlers.product, handlers.image)...)
			return nil
		},
	}
}

func newAmazonListingHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "amazon-listing",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(amazonlistinghttpapi.AppendRouteDescriptors(nil, handlers.amazonListing)...)
			return nil
		},
	}
}

func newListingKitHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "listing-kit",
		register: func(reg *kernelmodule.Registry) error {
			routes := listingkithttpapi.AppendRouteDescriptors(nil, handlers.listingKit)
			routes = listingkithttpapi.AppendPromptTemplateRouteDescriptors(routes, handlers.promptTemplate)
			routes = listingkithttpapi.AppendStudioSessionRouteDescriptors(routes, handlers.studioSession)
			reg.AddRoutes(routes...)
			return nil
		},
	}
}

func newOpsHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "ops",
		register: func(reg *kernelmodule.Registry) error {
			routes := appendSDSCatalogRouteDescriptors(nil, handlers.sdsCatalog)
			routes = appendTaskRPCRouteDescriptors(routes, handlers.taskRPC)
			routes = appendSheinLoginRouteDescriptors(routes, handlers.sheinLogin)
			routes = appendSDSLoginRouteDescriptors(routes, handlers.sdsLogin)
			reg.AddRoutes(routes...)
			return nil
		},
	}
}
