package httpapi

import (
	"context"
	"net/http"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"task-processor/internal/workbenchcontext"
	"time"

	"gorm.io/gorm"
)

// NewProductReviewApplication is explicitly assembled only for an admitted
// isolated current-product database and upstream bindings. No default runtime
// registration, schema migration, historical-data switch or provider fallback.
func NewProductReviewApplication(db *gorm.DB, verifier zitadelruntime.Verifier, resolver *workbenchcontext.Resolver, auth *authz.ListingKitAuthorizer, generator enrichment.CandidateGenerator, bindings []review.Binding) (*http.Server, error) {
	if db == nil || verifier == nil || resolver == nil || auth == nil {
		return nil, review.ErrUnavailable
	}
	reader, err := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
	if err != nil {
		return nil, err
	}
	store, err := reviewstore.NewRepository(db)
	if err != nil {
		return nil, err
	}
	proposer, err := enrichment.NewProposer(enrichment.Dependencies{Generator: generator})
	if err != nil {
		return nil, err
	}
	service, err := review.NewService(reader, store, proposer, auth, bindings)
	if err != nil {
		return nil, err
	}
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, productReviewRoutes(service), routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: auth})
	server.ReadTimeout = review.Timeout
	server.WriteTimeout = review.Timeout + 2*time.Second
	handler := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), review.Timeout)
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	return server, nil
}
