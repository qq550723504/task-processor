package httpapi

import (
	"task-processor/internal/amazonlisting"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func (c httpFeatureComposition) productHandler() productenrich.ProductHandler {
	if c.productModule == nil {
		return nil
	}
	return c.productModule.Handler
}

func (c httpFeatureComposition) imageHandler() productimagehttpapi.RouteHandler {
	if c.imageModule == nil {
		return nil
	}
	return c.imageModule.Handler
}

func (c httpFeatureComposition) amazonListingHandler() amazonlisting.Handler {
	if c.amazonListingModule == nil {
		return nil
	}
	return c.amazonListingModule.Handler
}

func (c httpFeatureComposition) listingKitHandler() listingkithttpapi.RouteHandler {
	if c.listingKitModule == nil {
		return nil
	}
	return c.listingKitModule.Handler
}

func (c httpFeatureComposition) studioSessionHandler() listingkit.StudioSessionHandler {
	if c.listingKitModule == nil {
		return nil
	}
	return c.listingKitModule.StudioSessionHandler
}
