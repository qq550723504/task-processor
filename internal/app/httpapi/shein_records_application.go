package httpapi

import (
	"context"
	"net/http"
	recordstore "task-processor/internal/app/listingrecordstore"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listing/record"
	"task-processor/internal/marketplace/shein/draft"
	"task-processor/internal/workbenchcontext"

	"gorm.io/gorm"
)

// NewSheinRecordApplication assembles an explicit application instance for the
// #319 current-source contract. currentProductDB MUST be the separately admitted
// Catalog storage scope with known writers (D1-CURRENT-PRODUCT-INPUT/V1).
// This is deliberately absent from default runtime composition/configuration:
// there is no boolean that admits the shared historical Catalog, no implicit
// database opening and no migration. Production source binding requires a
// separate rollout decision. Both domains share this single database boundary.
func NewSheinRecordApplication(currentProductDB *gorm.DB, verifier zitadelruntime.Verifier, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer) (*http.Server, record.Reader, error) {
	if currentProductDB == nil || verifier == nil || resolver == nil || authorizer == nil {
		return nil, nil, record.ErrUnavailable
	}
	source, err := catalogstore.NewBoundedSnapshotReader(currentProductDB, 8<<20)
	if err != nil {
		return nil, nil, err
	}
	repository, err := recordstore.NewRepository(currentProductDB, authorizer)
	if err != nil {
		return nil, nil, err
	}
	service, err := record.NewService(source, repository, draft.Builder{}, authorizer)
	if err != nil {
		return nil, nil, err
	}
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, sheinRecordRoutes(service), routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer})
	server.ReadTimeout = record.Timeout
	server.WriteTimeout = record.Timeout
	handler := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), record.Timeout)
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	return server, repository, nil
}
