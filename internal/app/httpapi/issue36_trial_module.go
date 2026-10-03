package httpapi

import (
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/workbenchcontext"

	"gorm.io/gorm"
)

type issue36TrialModule struct{ routes []httproute.Descriptor }

func (issue36TrialModule) Name() string                { return "issue36-local-trial" }
func (issue36TrialModule) Enabled(*config.Config) bool { return true }
func (m issue36TrialModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}

// buildIssue36TrialModule borrows one explicitly admitted current Product,
// ApprovedAsset, Store Center and Listing PostgreSQL boundary. It only
// assembles existing owner routes; schema installation and sample preparation
// belong to a separate one-shot local installer.
func buildIssue36TrialModule(db *gorm.DB, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	reviewRoutes, err := buildLocalTrialReviewRoutes(db, resolver, authorizer)
	if err != nil {
		return nil, err
	}
	listingRoutes, _, err := buildLocalTrialListingRoutes(db, authorizer)
	if err != nil {
		return nil, err
	}
	routes := make([]httproute.Descriptor, 0, len(reviewRoutes)+len(listingRoutes))
	routes = append(routes, reviewRoutes...)
	routes = append(routes, listingRoutes...)
	return issue36TrialModule{routes: routes}, nil
}
