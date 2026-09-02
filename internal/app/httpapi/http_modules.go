package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

func newCoreHTTPModule() httpModule {
	return httpModule{
		name: "system",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(httproute.Descriptor{
				Method: http.MethodGet,
				Path:   "/health",
				Module: "system",
				Handler: func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"status": "ok"})
				},
			})
			return nil
		},
	}
}

func (c httpFeatureComposition) amazonListingHTTPModule() kernelmodule.Module {
	if c.amazonListingModule != nil {
		return amazonlistinghttpapi.NewRuntimeModule(c.amazonListingModule)
	}
	return amazonlistinghttpapi.NewHTTPModule(nil)
}

func (c httpFeatureComposition) listingKitHTTPModule() kernelmodule.Module {
	if c.listingKitModule != nil {
		return listingkithttpapi.NewRuntimeModule(c.listingKitModule)
	}
	return listingkithttpapi.NewHTTPModule(nil)
}

func (c httpFeatureComposition) listingKitStudioHTTPModule() kernelmodule.Module {
	if c.listingKitModule != nil {
		return nil
	}
	return listingkithttpapi.NewStudioHTTPModule(nil)
}
