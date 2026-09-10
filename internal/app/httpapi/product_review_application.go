package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"

	"gorm.io/gorm"
)

// InstallProductReviewSchema atomically initializes all three current owners
// in one empty task schema. It is explicit startup work, never request DDL.
func InstallProductReviewSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return review.ErrUnavailable
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := productsourcing.InstallSchema(tx); err != nil {
			return err
		}
		return reviewstore.InstallSchema(tx)
	})
}

func productReviewSchemaReady(db *gorm.DB) bool {
	if db == nil || db.Dialector.Name() != "postgres" {
		return false
	}
	for _, table := range []string{
		"product_snapshot_versions", "product_snapshot_heads",
		"product_source_publications", "product_source_publication_receipts",
		"product_title_proposals", "product_title_operations",
	} {
		if !db.Migrator().HasTable(table) {
			return false
		}
	}
	return true
}

// NewProductReviewApplication is explicitly assembled only for an admitted
// isolated current-product database. No default runtime registration, schema
// migration, historical-data switch or provider fallback.
func NewProductReviewApplication(db *gorm.DB, verifier zitadelruntime.Verifier, resolver *workbenchcontext.Resolver, auth *authz.ListingKitAuthorizer, generator enrichment.CandidateGenerator) (*http.Server, error) {
	if db == nil || verifier == nil || resolver == nil || auth == nil {
		return nil, review.ErrUnavailable
	}
	if !productReviewSchemaReady(db) {
		return nil, review.ErrUnavailable
	}
	reader, err := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
	if err != nil {
		return nil, err
	}
	live := &productReviewLiveOrganizationAccess{resolver: resolver, now: time.Now}
	store, err := reviewstore.NewRepository(db, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	if err != nil {
		return nil, err
	}
	proposer, err := enrichment.NewProposer(enrichment.Dependencies{Generator: generator})
	if err != nil {
		return nil, err
	}
	sourceReader, err := productsourcing.NewInternalProducer(db, live, auth)
	if err != nil {
		return nil, err
	}
	service, err := review.NewService(reader, sourceReader, store, proposer, auth)
	if err != nil {
		return nil, err
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, productReviewRoutes(service, binder.Bind), routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: auth})
	server.ReadTimeout = review.Timeout
	server.WriteTimeout = review.Timeout + 2*time.Second
	handler := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(r.Context(), review.Timeout)
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	return server, nil
}

type productReviewCapabilityContextKey struct{}

type productReviewRequestCapability struct {
	bearerToken             string
	actorID                 string
	homeOrganizationID      string
	effectiveOrganizationID string
	tokenExpiresAt          time.Time
}

type productReviewCapabilityBinder struct{ now func() time.Time }

func (binder productReviewCapabilityBinder) Bind(ctx context.Context, authorization string) (context.Context, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	bearer := requestBearerToken(authorization)
	now := binder.now
	if now == nil {
		now = time.Now
	}
	if !ok || bearer == "" || identity.UserID == "" || identity.EffectiveOrganizationID == "" ||
		identity.TenantID != identity.EffectiveOrganizationID || identity.TokenExpiresAt.IsZero() || !now().Before(identity.TokenExpiresAt) {
		return ctx, review.ErrForbidden
	}
	capability := productReviewRequestCapability{
		bearerToken: bearer, actorID: identity.UserID, homeOrganizationID: identity.HomeOrganizationID,
		effectiveOrganizationID: identity.EffectiveOrganizationID, tokenExpiresAt: identity.TokenExpiresAt,
	}
	return context.WithValue(ctx, productReviewCapabilityContextKey{}, capability), nil
}

type productReviewLiveOrganizationAccess struct {
	resolver organizationIdentityResolver
	now      func() time.Time
}

func (access *productReviewLiveOrganizationAccess) ResolveLiveRoles(ctx context.Context, organizationID, actorID string) ([]string, error) {
	if access == nil || access.resolver == nil {
		return nil, sourcing.ErrSourcePublicationUnavailable
	}
	capability, ok := ctx.Value(productReviewCapabilityContextKey{}).(productReviewRequestCapability)
	identity, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	now := access.now
	if now == nil {
		now = time.Now
	}
	if !ok || !authenticated || capability.bearerToken == "" ||
		capability.actorID != actorID || capability.effectiveOrganizationID != organizationID ||
		identity.UserID != actorID || identity.EffectiveOrganizationID != organizationID || identity.TenantID != organizationID ||
		capability.tokenExpiresAt.IsZero() || !now().Before(capability.tokenExpiresAt) {
		return nil, sourcing.ErrPublicationForbidden
	}
	resolved, err := access.resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{
		Identity: authidentity.AuthenticatedIdentity{
			UserID: actorID, HomeOrganizationID: capability.homeOrganizationID, TokenExpiresAt: capability.tokenExpiresAt,
		},
		BearerToken: capability.bearerToken, RequestedOrganizationID: organizationID,
	})
	if err != nil {
		switch {
		case errors.Is(err, workbenchcontext.ErrAuthenticationRequired),
			errors.Is(err, workbenchcontext.ErrOrganizationAccessDenied),
			errors.Is(err, workbenchcontext.ErrOrganizationAccessRevoked),
			errors.Is(err, workbenchcontext.ErrOrganizationSuspended):
			return nil, sourcing.ErrPublicationForbidden
		default:
			return nil, sourcing.ErrSourcePublicationUnavailable
		}
	}
	if resolved.UserID != actorID || resolved.EffectiveOrganizationID != organizationID || resolved.TenantID != organizationID {
		return nil, sourcing.ErrPublicationForbidden
	}
	return append([]string(nil), resolved.Roles...), nil
}

var _ sourcing.LiveOrganizationAccess = (*productReviewLiveOrganizationAccess)(nil)
