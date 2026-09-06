package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	zitadel "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	imagehttp "task-processor/internal/imageagent/httpapi"
	imagestore "task-processor/internal/imageagent/store"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/workbenchcontext"
)

// OrganizationImageBinding is an explicitly admitted owner-qualified immutable
// current Catalog input. ContextID uses the existing BusinessTaskID transport
// field; it does not introduce a Task service or adopt historical task data.
type OrganizationImageBinding struct {
	ContextID     string
	OwnerUserID   string
	Identity      catalog.SnapshotIdentity
	Version       uint64
	PublicationID string
}

type organizationImageCatalog struct {
	reader   catalog.CompleteSnapshotReader
	bindings map[[3]string]OrganizationImageBinding
}

func (c *organizationImageCatalog) Resolve(ctx context.Context, scope imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.EffectiveOrganizationID == "" || identity.EffectiveOrganizationID != scope.TenantID || identity.TenantID != scope.TenantID || identity.UserID != scope.OwnerUserID {
		return imageagent.AssetCatalog{}, imageagent.ErrIdentityRequired
	}
	binding, ok := c.bindings[[3]string{scope.TenantID, scope.OwnerUserID, scope.BusinessTaskID}]
	if !ok {
		return imageagent.AssetCatalog{}, imageagent.ErrIdentityRequired
	}
	published, err := c.reader.GetSnapshot(ctx, binding.Identity, binding.Version)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	if published.Identity != binding.Identity || published.Version != binding.Version || published.PublicationID != binding.PublicationID {
		return imageagent.AssetCatalog{}, imageagent.ErrIdentityRequired
	}
	assets, err := selectAuthorizedAssets(buildAuthorizedAssetsFromCatalogImages(published.Snapshot.Images), scope.PrimarySourceAssetID, scope.StyleReferenceIDs, true)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	result, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{
		Assets:         assets,
		ProductContext: imageagent.ProductContextRef{ProductID: binding.Identity.ProductKey, Title: published.Snapshot.Title, ProductType: strings.Join(published.Snapshot.CategoryPath, " / "), SourceSnapshotVersion: binding.Version, Attributes: catalogAttributes(published.Snapshot.Attributes)},
	})
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	result.Manifest.Hash = imageagent.CatalogSnapshotHash(result.Assets, result.ProductContext)
	return result, nil
}

// NewImageAgentOrganizationApplication is an opt-in assembly for an isolated,
// admitted database and runtime. It performs no migration and is not registered
// by the default HTTP application. Every control uses the existing service.
func NewImageAgentOrganizationApplication(db *gorm.DB, verifier zitadel.Verifier, resolver *workbenchcontext.Resolver, auth *authz.ListingKitAuthorizer, workflows imageagent.WorkflowClient, gate imageagent.TenantStartGate, bindings []OrganizationImageBinding) (*http.Server, error) {
	if db == nil || verifier == nil || resolver == nil || auth == nil || workflows == nil || gate == nil {
		return nil, imageagent.ErrIdentityRequired
	}
	reader, err := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
	if err != nil {
		return nil, err
	}
	source := &organizationImageCatalog{reader: reader, bindings: make(map[[3]string]OrganizationImageBinding, len(bindings))}
	for _, binding := range bindings {
		for _, value := range []string{binding.ContextID, binding.OwnerUserID, binding.Identity.TenantID, binding.Identity.ProductKey, binding.PublicationID} {
			if value == "" || value != strings.TrimSpace(value) {
				return nil, fmt.Errorf("invalid organization image binding")
			}
		}
		key := [3]string{binding.Identity.TenantID, binding.OwnerUserID, binding.ContextID}
		if _, exists := source.bindings[key]; exists || binding.Version == 0 {
			return nil, fmt.Errorf("duplicate or invalid organization image binding")
		}
		source.bindings[key] = binding
	}
	service, err := imageagent.NewService(imagestore.NewOrganizationRepository(db), workflows, source, imageagent.WithOrganizationScope(), imageagent.WithTenantStartGate(gate))
	if err != nil {
		return nil, err
	}
	handler, err := imagehttp.NewHandler(service)
	if err != nil {
		return nil, err
	}
	routes := imagehttp.AppendRouteDescriptors(nil, handler)
	selected := make([]httproute.Descriptor, 0, len(routes))
	for _, route := range routes {
		if strings.Contains(route.Path, "/task-runs") {
			continue
		}
		route.Path = strings.Replace(route.Path, "/api/v1/image-agent", "/api/organization/image-agent", 1)
		route.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyLiveWrite
		if route.Method == http.MethodGet {
			route.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
		}
		selected = append(selected, route)
	}
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, selected, routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: auth})
	server.ReadTimeout = 30 * time.Second
	server.WriteTimeout = 32 * time.Second
	base := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		base.ServeHTTP(w, r.WithContext(ctx))
	})
	return server, nil
}
