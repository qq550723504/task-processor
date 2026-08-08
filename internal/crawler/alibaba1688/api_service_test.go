package alibaba1688

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

func TestNewAPIServiceWithStoreRepositoryWiresAccountProfileResolver(t *testing.T) {
	cfg := config.NewDefaultConfig()
	repository := &accountProfileStoreRepository{}

	service := NewAPIServiceWithStoreRepository(cfg, nil, 8080, repository)

	if service == nil || service.crawlerService == nil {
		t.Fatal("expected API service and crawler service")
	}
	if service.crawlerService.accountProfileResolver == nil {
		t.Fatal("expected repository-backed account profile resolver")
	}
	resolver, ok := service.crawlerService.accountProfileResolver.(*repositoryAccountProfileResolver)
	if !ok {
		t.Fatalf("resolver type = %T, want repositoryAccountProfileResolver", service.crawlerService.accountProfileResolver)
	}
	if resolver.repository != repository || resolver.profileRootDir != cfg.Platforms.Alibaba1688.ProfileRootDir {
		t.Fatal("repository-backed constructor did not preserve repository and configured profile root")
	}
}

func TestNewAPIServiceRemainsResolverFreeWithoutRepository(t *testing.T) {
	service := NewAPIService(config.NewDefaultConfig(), nil, 8080)

	if service == nil || service.crawlerService == nil {
		t.Fatal("expected API service and crawler service")
	}
	if service.crawlerService.accountProfileResolver != nil {
		t.Fatal("expected repository-free constructor to leave account profile resolver unset")
	}
}

func TestNewAPIServiceBuildsAndClosesAccountRepositoryWhenDatabaseConfigured(t *testing.T) {
	previousBuilder := buildListingAdminStoreRepository
	t.Cleanup(func() { buildListingAdminStoreRepository = previousBuilder })

	repository := &accountProfileStoreRepository{}
	closerCalls := 0
	buildListingAdminStoreRepository = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return repository, []func() error{func() error {
			closerCalls++
			return nil
		}}, nil
	}
	cfg := config.NewDefaultConfig()
	cfg.Database = &config.DatabaseConfig{Host: "configured-db"}
	service := NewAPIService(cfg, logrus.New(), 8080)

	resolver, ok := service.crawlerService.accountProfileResolver.(*repositoryAccountProfileResolver)
	if !ok || resolver.repository != repository {
		t.Fatalf("resolver = %T, want repository-backed resolver", service.crawlerService.accountProfileResolver)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if closerCalls != 1 {
		t.Fatalf("closer calls = %d, want 1", closerCalls)
	}
}

func TestNewAPIServiceDoesNotLogRepositoryBuilderErrorDetails(t *testing.T) {
	previousBuilder := buildListingAdminStoreRepository
	t.Cleanup(func() { buildListingAdminStoreRepository = previousBuilder })
	const secret = "database-password-must-not-leak"
	buildListingAdminStoreRepository = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		return nil, nil, errors.New(secret)
	}
	var logs bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logs)
	cfg := config.NewDefaultConfig()
	cfg.Database = &config.DatabaseConfig{Host: "configured-db"}

	service := NewAPIService(cfg, logger, 8080)

	if service.crawlerService.accountProfileResolver != nil {
		t.Fatal("expected resolver-free fallback when repository builder fails")
	}
	if bytes.Contains(logs.Bytes(), []byte(secret)) {
		t.Fatal("repository builder error leaked secret text")
	}
}

func TestWrapZitadelAuthMiddlewareForwardsVerifiedIdentity(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := listingkit.AuthenticatedIdentityFromContext(r.Context())
		if !ok || identity.TenantID != "101" {
			http.Error(w, "verified identity missing", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	middleware := func(c *gin.Context) {
		c.Request = c.Request.WithContext(listingkit.WithAuthenticatedIdentity(c.Request.Context(), listingkit.AuthenticatedIdentity{TenantID: "101", UserID: "user-101"}))
		c.Next()
	}

	response := httptest.NewRecorder()
	wrapZitadelAuthMiddleware(next, middleware).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/crawl", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
