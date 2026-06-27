package httpapi

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestBuildClientConfigUsesLoginServiceFromConfig(t *testing.T) {
	cfg := &config.Config{
		Management: config.ManagementConfig{
			TenantID: "286",
			StoreIDs: []int64{869},
		},
		Platforms: config.PlatformsConfig{
			SDS: config.SDSPlatformConfig{
				LoginService: config.SDSLoginServiceConfig{
					BaseURL:      "http://login:8000",
					SharedKey:    "shared-key",
					MerchantName: "merchant",
					Username:     "tester",
					Password:     "secret",
				},
			},
		},
	}

	clientCfg := BuildClientConfig(cfg)
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

func TestBuildClientConfigUsesAuthBootstrapFromConfig(t *testing.T) {
	cfg := &config.Config{
		Management: config.ManagementConfig{
			BaseURL:      "https://api.example.test",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			TenantID:     "286",
		},
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

	clientCfg := BuildClientConfig(cfg)
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
		t.Fatalf("captcha param = %q", clientCfg.AuthBootstrap.LoginVerifyCaptchaParam)
	}
	if clientCfg.AuthBootstrap.LoginExtraInfo != "{\"risk\":1}" {
		t.Fatalf("extra info = %q", clientCfg.AuthBootstrap.LoginExtraInfo)
	}
}
