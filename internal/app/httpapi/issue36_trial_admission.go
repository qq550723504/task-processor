package httpapi

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/httproute"
	"task-processor/internal/listing/record"
	"task-processor/internal/product/review"
)

// WithIssue36Trial admits only the Review and Listing routes of the isolated
// local trial. The caller retains ownership of its separate narrow-role pool.
func WithIssue36Trial(db *gorm.DB) CurrentApplicationOption {
	return func(options *currentApplicationOptions) {
		options.localTrials++
		options.localTrialDB = db
	}
}

func trialRouteDescriptorsForAdmission() []httproute.Descriptor {
	routes := productReviewOnlyRoutes(nil, nil)
	for i := range routes {
		routes[i].RequestTimeout = review.Timeout + 2*time.Second
	}
	listing := append(sheinRecordRoutes(&record.Service{}), sheinRecordCollectionRoutes(&record.CollectionService{})...)
	listing = append(listing, sheinDiagnosticRoutes(&record.DiagnosticService{})...)
	for i := range listing {
		listing[i].RequestTimeout = record.Timeout + 2*time.Second
	}
	return append(routes, listing...)
}

func isIssue36TrialRoute(method, path string) bool {
	return strings.HasPrefix(path, "/api/product/text-proposals") ||
		path == sheinRecordPath || strings.HasPrefix(path, sheinRecordPath+"/") ||
		path == sheinDiagnosticPath
}

func validIssue36TrialDescriptor(route httproute.Descriptor) bool {
	for _, expected := range trialRouteDescriptorsForAdmission() {
		if route.Method == expected.Method && route.Path == expected.Path {
			return route.Module == expected.Module && route.Permission == expected.Permission &&
				route.AuthPolicy == expected.AuthPolicy && route.OrganizationAccessPolicy == expected.OrganizationAccessPolicy &&
				route.RequestTimeout == expected.RequestTimeout && route.RejectUnreadRequestBody == expected.RejectUnreadRequestBody &&
				(route.OrganizationTargetResolver == nil) == (expected.OrganizationTargetResolver == nil) && route.Handler != nil
		}
	}
	return false
}
