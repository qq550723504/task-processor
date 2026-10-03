package httpapi

import (
	"context"
	"net/http"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/product/review"
	"task-processor/internal/workbenchcontext"
)

// productReviewOnlyRoutes retains human review of an already admitted candidate.
// The fixed-generator proposal creation endpoint is intentionally unavailable.
func productReviewOnlyRoutes(service *review.Service, bind func(context.Context, string) (context.Context, error), projections ...productReviewContextProjector) []httproute.Descriptor {
	all := productReviewRoutes(service, bind, projections...)
	routes := make([]httproute.Descriptor, 0, len(all)-1)
	for _, route := range all {
		if route.Method == http.MethodPost && route.Path == "/api/product/text-proposals" {
			continue
		}
		routes = append(routes, route)
	}
	return routes
}

// buildLocalTrialReviewRoutes binds the current Review owner to an explicitly
// admitted local trial database. It has no candidate generation capability.
func buildLocalTrialReviewRoutes(db *gorm.DB, resolver *workbenchcontext.Resolver, auth *authz.ListingKitAuthorizer) ([]httproute.Descriptor, error) {
	core, err := buildProductReviewCore(db, resolver, auth)
	if err != nil {
		return nil, err
	}
	service, err := review.NewCandidateService(core.reader, core.sourceReader, core.store, auth)
	if err != nil {
		return nil, err
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	routes := productReviewOnlyRoutes(service, binder.Bind)
	for i := range routes {
		routes[i].RequestTimeout = review.Timeout + 2*time.Second
	}
	return routes, nil
}
