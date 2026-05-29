package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	"task-processor/internal/sdslogin"
	"task-processor/internal/sheinlogin"
	"task-processor/internal/taskrpcapi"
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

func newProductHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	return productenrichhttpapi.NewHTTPModule(handlers.product, handlers.image)
}

func newAmazonListingHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	return amazonlistinghttpapi.NewHTTPModule(handlers.amazonListing)
}

func newListingKitHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	return listingkithttpapi.NewHTTPModule(handlers.listingKit, handlers.promptTemplate, handlers.studioSession)
}

func newOpsHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "ops",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(appendSDSCatalogRouteDescriptors(nil, handlers.sdsCatalog)...)
			for _, module := range []kernelmodule.Module{
				taskrpcapi.NewHTTPModule(handlers.taskRPC),
				sheinlogin.NewHTTPModule(handlers.sheinLogin),
				sdslogin.NewHTTPModule(handlers.sdsLogin),
			} {
				if err := module.Register(reg); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
