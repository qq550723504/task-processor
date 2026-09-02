package httpapi

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	productasset "task-processor/internal/product/asset"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsclient "task-processor/internal/sds/client"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdsusecase "task-processor/internal/sds/usecase"
)

type stubHTTPAPIApprovedAssetReader struct{}

func (*stubHTTPAPIApprovedAssetReader) GetApprovedInventory(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
}

type stubHTTPAPISDSSyncService struct{}

func (s *stubHTTPAPISDSSyncService) SyncFromApprovedAssets(context.Context, sdsusecase.ApprovedAssetsInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

func TestBuildSDSSyncServiceReturnsNilWithoutApprovedAssetReader(t *testing.T) {
	logger := logrus.New()
	if svc := buildSDSSyncService(logger, &runtimeDeps{}); svc != nil {
		t.Fatalf("buildSDSSyncService() = %v, want nil", svc)
	}
}

func TestEnsureApprovedAssetReaderUsesSharedPersistentDatabaseWithoutAnotherCloser(t *testing.T) {
	db := openHTTPAPICatalogTestDB(t)
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{cfg: &config.Config{}, productCatalogDB: db},
		features: &featureRuntimeState{},
	}

	first := ensureApprovedAssetReader(logrus.New(), deps)
	second := ensureApprovedAssetReader(logrus.New(), deps)
	if first == nil || second != first {
		t.Fatalf("approved asset readers = (%v, %v), want one shared reader", first, second)
	}
	if len(deps.shared.closers) != 0 || len(deps.constructionClosers) != 0 {
		t.Fatalf("registered closers = shared:%d construction:%d, want database owner to remain unique", len(deps.shared.closers), len(deps.constructionClosers))
	}
}

func TestBuildSDSSyncServiceReturnsServiceWithoutAuthState(t *testing.T) {
	logger := logrus.New()
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	expected := &stubHTTPAPISDSSyncService{}
	var received sdsadapter.ApprovedAssetReader
	newSDSSyncServiceForHTTPAPI = func(approvedAssets sdsadapter.ApprovedAssetReader, cfg *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
		received = approvedAssets
		return expected, nil, nil
	}
	approvedAssets := &stubHTTPAPIApprovedAssetReader{}

	svc := buildSDSSyncService(logger, &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg: &config.Config{},
		},
		features: &featureRuntimeState{listingKitSupport: &listingKitSupport{approvedAssetReader: approvedAssets}},
	})
	if svc != expected {
		t.Fatalf("buildSDSSyncService() = %v, want %v", svc, expected)
	}
	if received != approvedAssets {
		t.Fatalf("SDS approved asset reader = %v, want shared reader %v", received, approvedAssets)
	}
}

func TestNewSDSSyncServiceForHTTPAPIReturnsServiceWithoutAuthState(t *testing.T) {
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})

	cfg := sdsclient.DefaultConfig()
	cfg.AuthFile = t.TempDir() + "/missing-auth.json"
	cfg.CookieFile = t.TempDir() + "/missing-cookie.json"
	cfg.BaseURL = "http://127.0.0.1:1"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}

	svc, authState, err := previousFactory(&stubHTTPAPIApprovedAssetReader{}, cfg)
	if err != nil {
		t.Fatalf("newSDSSyncServiceForHTTPAPI() error = %v", err)
	}
	if svc == nil {
		t.Fatal("newSDSSyncServiceForHTTPAPI() returned nil service without auth state")
	}
	if authState != nil {
		t.Fatalf("authState = %+v, want nil without bootstrap state", authState)
	}
}

func TestBuildSDSSyncServiceReturnsNilOnFactoryError(t *testing.T) {
	logger := logrus.New()
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	newSDSSyncServiceForHTTPAPI = func(approvedAssets sdsadapter.ApprovedAssetReader, cfg *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return nil, nil, fmt.Errorf("boom")
	}

	svc := buildSDSSyncService(logger, &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg: &config.Config{},
		},
		features: &featureRuntimeState{listingKitSupport: &listingKitSupport{approvedAssetReader: &stubHTTPAPIApprovedAssetReader{}}},
	})
	if svc != nil {
		t.Fatalf("buildSDSSyncService() = %v, want nil", svc)
	}
}

func TestBuildSDSSyncServiceReturnsServiceWithAuthState(t *testing.T) {
	logger := logrus.New()
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	expected := &stubHTTPAPISDSSyncService{}
	newSDSSyncServiceForHTTPAPI = func(approvedAssets sdsadapter.ApprovedAssetReader, cfg *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return expected, &sdsclient.AuthState{AccessToken: "token"}, nil
	}

	svc := buildSDSSyncService(logger, &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg: &config.Config{},
		},
		features: &featureRuntimeState{listingKitSupport: &listingKitSupport{approvedAssetReader: &stubHTTPAPIApprovedAssetReader{}}},
	})
	if svc != expected {
		t.Fatalf("buildSDSSyncService() = %v, want %v", svc, expected)
	}
}

func TestBuildSDSClientConfigUsesLoginServiceFromConfig(t *testing.T) {
	cfg := &config.Config{
		Platforms: config.PlatformsConfig{
			SDS: config.SDSPlatformConfig{
				LoginService: config.SDSLoginServiceConfig{
					BaseURL:      "http://login:8000",
					SharedKey:    "shared-key",
					TenantID:     "286",
					Identifier:   "869",
					MerchantName: "merchant",
					Username:     "tester",
					Password:     "secret",
				},
			},
		},
	}

	clientCfg := sdshttpapi.BuildClientConfig(cfg)
	if clientCfg.AuthBootstrap.LoginServiceBaseURL != "http://login:8000" {
		t.Fatalf("base URL = %q", clientCfg.AuthBootstrap.LoginServiceBaseURL)
	}
	if clientCfg.AuthBootstrap.LoginServiceSharedKey != "shared-key" {
		t.Fatalf("shared key = %q", clientCfg.AuthBootstrap.LoginServiceSharedKey)
	}
	if clientCfg.AuthBootstrap.LoginServiceTenantID != "286" {
		t.Fatalf("tenant = %q", clientCfg.AuthBootstrap.LoginServiceTenantID)
	}
	if clientCfg.AuthBootstrap.LoginServiceIdentifier != "869" {
		t.Fatalf("identifier = %q", clientCfg.AuthBootstrap.LoginServiceIdentifier)
	}
	if !clientCfg.AuthBootstrap.HasSource() {
		t.Fatal("expected login service config to be a refresh source")
	}
	if clientCfg.AuthBootstrap.LoginMerchantName != "merchant" {
		t.Fatalf("merchant name = %q", clientCfg.AuthBootstrap.LoginMerchantName)
	}
	if clientCfg.AuthBootstrap.LoginUsername != "tester" {
		t.Fatalf("username = %q", clientCfg.AuthBootstrap.LoginUsername)
	}
	if clientCfg.AuthBootstrap.LoginPassword != "secret" {
		t.Fatalf("password = %q", clientCfg.AuthBootstrap.LoginPassword)
	}
}

func TestBuildSDSClientConfigUsesAuthBootstrapFromConfig(t *testing.T) {
	cfg := &config.Config{
		Platforms: config.PlatformsConfig{
			SDS: config.SDSPlatformConfig{
				AuthBootstrap: config.SDSAuthBootstrapConfig{
					StaticAccessToken:       "access-token",
					StaticOutToken:          "out-token",
					StaticMerchantID:        12345,
					StaticCookie:            "cookie=value",
					LoginDomainName:         "www.sdsdiy.com",
					LoginVerifyCaptchaParam: "captcha-param",
					LoginExtraInfo:          "{\"risk\":1}",
				},
			},
		},
	}

	clientCfg := sdshttpapi.BuildClientConfig(cfg)
	if clientCfg.AuthBootstrap.StaticAccessToken != "access-token" {
		t.Fatalf("access token = %q", clientCfg.AuthBootstrap.StaticAccessToken)
	}
	if clientCfg.AuthBootstrap.StaticOutToken != "out-token" {
		t.Fatalf("out token = %q", clientCfg.AuthBootstrap.StaticOutToken)
	}
	if clientCfg.AuthBootstrap.StaticMerchantID != 12345 {
		t.Fatalf("merchant id = %d", clientCfg.AuthBootstrap.StaticMerchantID)
	}
	if clientCfg.AuthBootstrap.StaticCookie != "cookie=value" {
		t.Fatalf("cookie = %q", clientCfg.AuthBootstrap.StaticCookie)
	}
	if clientCfg.AuthBootstrap.LoginDomainName != "www.sdsdiy.com" {
		t.Fatalf("domain name = %q", clientCfg.AuthBootstrap.LoginDomainName)
	}
	if clientCfg.AuthBootstrap.LoginVerifyCaptchaParam != "captcha-param" {
		t.Fatalf("verify captcha param = %q", clientCfg.AuthBootstrap.LoginVerifyCaptchaParam)
	}
	if clientCfg.AuthBootstrap.LoginExtraInfo != "{\"risk\":1}" {
		t.Fatalf("extra info = %q", clientCfg.AuthBootstrap.LoginExtraInfo)
	}
}
