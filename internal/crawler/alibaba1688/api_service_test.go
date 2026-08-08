package alibaba1688

import (
	"testing"

	"task-processor/internal/core/config"
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
