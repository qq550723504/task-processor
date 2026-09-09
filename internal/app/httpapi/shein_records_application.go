package httpapi

import (
	"context"
	"errors"
	"net/http"
	recordstore "task-processor/internal/app/listingrecordstore"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listing/record"
	"task-processor/internal/marketplace/shein/draft"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"time"

	"gorm.io/gorm"
)

// NewSheinRecordApplication assembles an explicit application instance for the
// #376 DRAFT-S1 contract. currentProductDB MUST be the explicitly admitted
// Product/Asset, Store Center and Listing storage boundary with known writers.
// This is deliberately absent from default runtime composition/configuration:
// there is no boolean that admits the shared historical Catalog, no implicit
// database opening and no migration. Production source binding requires a
// separate rollout decision. Both domains share this single database boundary.
func NewSheinRecordApplication(currentProductDB *gorm.DB, verifier zitadelruntime.Verifier, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer) (*http.Server, record.Reader, error) {
	if currentProductDB == nil || verifier == nil || resolver == nil || authorizer == nil {
		return nil, nil, record.ErrUnavailable
	}
	source, err := catalogstore.NewBoundedSnapshotReader(currentProductDB, record.MaxPayloadBytes)
	if err != nil {
		return nil, nil, err
	}
	assets, err := assetstore.NewBoundedApprovedInventoryReader(currentProductDB, record.MaxPayloadBytes)
	if err != nil {
		return nil, nil, err
	}
	stores, err := storecenter.NewGormStoreRepository(currentProductDB)
	if err != nil {
		return nil, nil, err
	}
	repository, err := recordstore.NewRepository(currentProductDB, authorizer)
	if err != nil {
		return nil, nil, err
	}
	service, err := record.NewService(record.ServiceDependencies{
		Products: source, Assets: assets, Stores: sheinRecordStoreReader{repository: stores}, Records: repository,
		Builder: draft.Builder{}, Evaluator: sheinvalidator.ExactApprovedAssetValidator{}, Authorizer: authorizer,
		Now: time.Now, RuleRevision: sheinvalidator.DiagnosticRuleVersion, PolicyRevision: sheinvalidator.BindingVersion,
	})
	if err != nil {
		return nil, nil, err
	}
	collection, err := record.NewCollectionService(repository, authorizer)
	if err != nil {
		return nil, nil, err
	}
	diagnostic, err := record.NewDiagnosticService(repository, sheinvalidator.DiagnosticValidator{}, authorizer, sheinvalidator.DiagnosticRuleVersion, sheinvalidator.BindingVersion)
	if err != nil {
		return nil, nil, err
	}
	routes := append(sheinRecordRoutes(service), sheinRecordCollectionRoutes(collection)...)
	routes = append(routes, sheinDiagnosticRoutes(diagnostic)...)
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, routes, routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer})
	server.ReadTimeout = record.Timeout
	// The transport deadline starts before the application deadline. Reserve
	// bounded headroom so an expired operation can still return its HTTP 504.
	server.WriteTimeout = record.Timeout + 2*time.Second
	handler := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set the protected-metadata response policy before auth and organization
		// middleware so their early failures cannot be cached or content-sniffed.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(r.Context(), record.Timeout)
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	return server, repository, nil
}

type sheinRecordStoreReader struct{ repository storecenter.Repository }

func (r sheinRecordStoreReader) GetStoreReference(ctx context.Context, organizationID, storeID string) (record.StoreReference, error) {
	stored, err := r.repository.Get(ctx, organizationID, storeID)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return record.StoreReference{}, err
	}
	if errors.Is(err, storecenter.ErrNotFound) {
		return record.StoreReference{}, record.ErrNotFound
	}
	if err != nil {
		return record.StoreReference{}, record.ErrUnavailable
	}
	if stored == nil || stored.OrganizationID() != organizationID || stored.ID() != storeID {
		return record.StoreReference{}, record.ErrNotFound
	}
	return record.StoreReference{OrganizationID: stored.OrganizationID(), StoreID: stored.ID(), Platform: string(stored.Platform())}, nil
}
