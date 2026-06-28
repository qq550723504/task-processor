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

func (c httpFeatureComposition) productHTTPModule() kernelmodule.Module {
	if c.productModule != nil {
		return productenrichhttpapi.NewRuntimeModule(c.productModule, c.imageModule)
	}
	return productenrichhttpapi.NewHTTPModule(c.productHandler(), c.imageHandler())
}

func (c httpFeatureComposition) amazonListingHTTPModule() kernelmodule.Module {
	if c.amazonListingModule != nil {
		return amazonlistinghttpapi.NewRuntimeModule(c.amazonListingModule)
	}
	return amazonlistinghttpapi.NewHTTPModule(c.amazonListingHandler())
}

func (c httpFeatureComposition) listingKitHTTPModule() kernelmodule.Module {
	if c.listingKitModule != nil {
		return listingkithttpapi.NewRuntimeModule(c.listingKitModule)
	}
	return listingkithttpapi.NewHTTPModule(c.listingKitHandler())
}

func (c httpFeatureComposition) listingKitStudioHTTPModule() kernelmodule.Module {
	if c.listingKitModule != nil {
		return nil
	}
	return listingkithttpapi.NewStudioHTTPModule(c.studioSessionHandler())
}
