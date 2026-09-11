package currentapplication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	IntrospectionEndpoint string `json:"introspection_endpoint"`
}

func VerifyIdentityProvider(ctx context.Context, cfg IdentityConfig) error {
	return verifyIdentityProvider(ctx, cfg, &http.Client{Timeout: 5 * time.Second})
}

func verifyIdentityProvider(ctx context.Context, cfg IdentityConfig, client *http.Client) error {
	if ctx == nil || client == nil {
		return errors.New("identity readiness dependencies unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.IssuerURL, "/")+"/.well-known/openid-configuration", nil)
	if err != nil {
		return fmt.Errorf("build identity readiness request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("identity provider unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("identity provider unavailable: discovery status %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64*1024+1))
	var discovery oidcDiscovery
	if err := decoder.Decode(&discovery); err != nil {
		return fmt.Errorf("decode identity discovery: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode identity discovery: %w", err)
	}
	if discovery.Issuer != cfg.IssuerURL {
		return errors.New("identity discovery issuer mismatch")
	}
	origin, err := url.Parse(cfg.IssuerURL)
	if err != nil {
		return errors.New("identity issuer invalid")
	}
	for name, raw := range map[string]string{
		"authorization": discovery.AuthorizationEndpoint,
		"token":         discovery.TokenEndpoint,
		"userinfo":      discovery.UserInfoEndpoint,
		"introspection": discovery.IntrospectionEndpoint,
	} {
		endpoint, parseErr := url.Parse(raw)
		if parseErr != nil || endpoint.Scheme != origin.Scheme || endpoint.Host != origin.Host || endpoint.User != nil {
			return fmt.Errorf("identity discovery %s endpoint is outside the admitted origin", name)
		}
	}
	return nil
}
