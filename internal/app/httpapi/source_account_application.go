package httpapi

import (
	"net/http"

	"gorm.io/gorm"

	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/sourceaccountregistry"
	sourceaccounthttpapi "task-processor/internal/sourceaccountregistry/httpapi"
	"task-processor/internal/workbenchcontext"
)

// NewSourceAccountApplication assembles only the admitted SA1 current-resource
// chain. The caller owns db and the returned server lifecycle. This function
// does not load config, open or migrate a database, or invoke default/legacy
// feature composition. The caller initializes the current schema explicitly
// through internal/app/schema/sourceaccountregistry.Migrate before assembly.
func NewSourceAccountApplication(db *gorm.DB, verifier zitadelruntime.Verifier, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer) (*http.Server, error) {
	if db == nil || verifier == nil || resolver == nil || authorizer == nil {
		return nil, sourceaccountregistry.ErrUnavailable
	}
	module, err := buildSourceAccountModule(db, authorizer)
	if err != nil {
		return nil, err
	}
	registry := kernelmodule.NewRegistry()
	if err := module.Register(registry); err != nil {
		return nil, err
	}
	return buildIsolatedApplicationHTTPServer(registry.Routes(), routeAuthDependencies{
		workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer,
	}, sourceaccountregistry.Timeout), nil
}

func buildSourceAccountModule(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	if db == nil || authorizer == nil {
		return nil, sourceaccountregistry.ErrUnavailable
	}
	repository, err := sourceaccountstore.NewRepository(db)
	if err != nil {
		return nil, err
	}
	service, err := sourceaccountregistry.NewService(repository, authorizer)
	if err != nil {
		return nil, err
	}
	handler, err := sourceaccounthttpapi.NewHandler(service)
	if err != nil {
		return nil, err
	}
	return sourceaccounthttpapi.NewModule(handler), nil
}
