package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
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
	return listingkithttpapi.NewHTTPModule(handlers.listingKit)
}

func newListingKitStudioHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	return listingkithttpapi.NewStudioHTTPModule(handlers.studioSession)
}

func newPromptTemplateHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	if handlers.promptModule != nil {
		return handlers.promptModule
	}
	return promptmgmtapi.NewHTTPModule(handlers.promptTemplate)
}

func newSDSCatalogHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	if handlers.sdsModule != nil {
		return handlers.sdsModule
	}
	return sdshttpapi.NewHTTPModule(handlers.sdsCatalog)
}

func newTaskRPCHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	return taskrpcapi.NewHTTPModule(handlers.taskRPC)
}

func newSheinLoginHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	return sheinlogin.NewHTTPModule(handlers.sheinLogin)
}

func newSDSLoginHTTPModule(handlers httpModuleHandlers) kernelmodule.Module {
	return sdslogin.NewHTTPModule(handlers.sdsLogin)
}
