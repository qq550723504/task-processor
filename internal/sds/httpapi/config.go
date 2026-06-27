package httpapi

import (
	"strings"

	"task-processor/internal/core/config"
	sdsclient "task-processor/internal/sds/client"
)

func BuildClientConfig(cfg *config.Config) *sdsclient.Config {
	clientCfg := sdsclient.DefaultConfig()
	if cfg == nil {
		return clientCfg
	}
	authBootstrap := cfg.Platforms.SDS.AuthBootstrap
	if value := strings.TrimSpace(authBootstrap.StaticAccessToken); value != "" {
		clientCfg.AuthBootstrap.StaticAccessToken = value
	}
	if value := strings.TrimSpace(authBootstrap.StaticOutToken); value != "" {
		clientCfg.AuthBootstrap.StaticOutToken = value
	}
	if authBootstrap.StaticMerchantID > 0 {
		clientCfg.AuthBootstrap.StaticMerchantID = authBootstrap.StaticMerchantID
	}
	if value := strings.TrimSpace(authBootstrap.StaticCookie); value != "" {
		clientCfg.AuthBootstrap.StaticCookie = value
	}
	if value := strings.TrimSpace(authBootstrap.LoginDomainName); value != "" {
		clientCfg.AuthBootstrap.LoginDomainName = value
	}
	if value := strings.TrimSpace(authBootstrap.LoginVerifyCaptchaParam); value != "" {
		clientCfg.AuthBootstrap.LoginVerifyCaptchaParam = value
	}
	if value := strings.TrimSpace(authBootstrap.LoginExtraInfo); value != "" {
		clientCfg.AuthBootstrap.LoginExtraInfo = value
	}
	loginService := cfg.Platforms.SDS.LoginService
	clientCfg.LoginService = loginService
	if value := strings.TrimSpace(loginService.BaseURL); value != "" {
		clientCfg.AuthBootstrap.LoginServiceBaseURL = value
	}
	if value := strings.TrimSpace(loginService.SharedKey); value != "" {
		clientCfg.AuthBootstrap.LoginServiceSharedKey = value
	}
	if value := strings.TrimSpace(loginService.TenantID); value != "" {
		clientCfg.AuthBootstrap.LoginServiceTenantID = value
	}
	if value := strings.TrimSpace(loginService.Identifier); value != "" {
		clientCfg.AuthBootstrap.LoginServiceIdentifier = value
	}
	if value := strings.TrimSpace(loginService.MerchantName); value != "" {
		clientCfg.AuthBootstrap.LoginMerchantName = value
	}
	if value := strings.TrimSpace(loginService.Username); value != "" {
		clientCfg.AuthBootstrap.LoginUsername = value
	}
	if value := strings.TrimSpace(loginService.Password); value != "" {
		clientCfg.AuthBootstrap.LoginPassword = value
	}
	return clientCfg
}
