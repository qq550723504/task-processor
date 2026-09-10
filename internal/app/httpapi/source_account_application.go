package httpapi

import (
	"context"
	"net/http"
	"time"

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
// feature composition.
func NewSourceAccountApplication(db *gorm.DB, verifier zitadelruntime.Verifier, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer) (*http.Server, error) {
	if db == nil || verifier == nil || resolver == nil || authorizer == nil {
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
	registry := kernelmodule.NewRegistry()
	if err := sourceaccounthttpapi.NewModule(handler).Register(registry); err != nil {
		return nil, err
	}
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, registry.Routes(), routeAuthDependencies{
		workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer,
	})
	server.ReadTimeout = sourceaccountregistry.Timeout
	server.WriteTimeout = sourceaccountregistry.Timeout + 2*time.Second
	inner := server.Handler
	server.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(request.Context(), sourceaccountregistry.Timeout)
		defer cancel()
		inner.ServeHTTP(writer, request.WithContext(ctx))
	})
	return server, nil
}
