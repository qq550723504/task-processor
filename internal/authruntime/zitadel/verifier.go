package zitadel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"task-processor/internal/authidentity"
)

var (
	errResourceOwnerMissing = errors.New("ZITADEL resource owner is required")
	errSubjectMissing       = errors.New("ZITADEL subject is required")
)

type Verifier interface {
	Verify(context.Context, string) (authidentity.AuthenticatedIdentity, error)
}

type verifier struct {
	cfg       Config
	mu        sync.Mutex
	discovery discoveryDocument
}

func NewVerifier(cfg Config) Verifier {
	return newVerifier(normalizeConfig(cfg))
}

func newVerifier(cfg Config) *verifier {
	return &verifier{cfg: cfg}
}

func (v *verifier) Verify(ctx context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return authidentity.AuthenticatedIdentity{}, errors.New("ZITADEL bearer token is required")
	}

	payload, err := v.introspect(ctx, token)
	if err != nil {
		return authidentity.AuthenticatedIdentity{}, err
	}

	tenantID := strings.TrimSpace(payload.ResourceID)
	if tenantID == "" {
		return authidentity.AuthenticatedIdentity{}, errResourceOwnerMissing
	}
	userID := strings.TrimSpace(payload.Subject)
	if userID == "" {
		return authidentity.AuthenticatedIdentity{}, errSubjectMissing
	}

	return authidentity.AuthenticatedIdentity{
		TenantID: tenantID,
		UserID:   userID,
		Roles:    append([]string(nil), payload.Roles...),
	}, nil
}

func (v *verifier) introspect(ctx context.Context, token string) (*IntrospectionResponse, error) {
	discovery, err := v.getDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	if discovery.IntrospectionEndpoint == "" {
		return nil, errors.New("ZITADEL introspection endpoint is not available")
	}

	form := url.Values{}
	form.Set("token", token)
	form.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.IntrospectionEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if v.cfg.ClientSecret != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(v.cfg.ClientID+":"+v.cfg.ClientSecret)))
	}

	resp, err := v.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ZITADEL token introspection failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ZITADEL token introspection response is invalid: %w", err)
	}

	var payload IntrospectionResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("ZITADEL token introspection response is invalid: %w", err)
	}
	payload.Extra = data
	payload.Roles = ParseRoles(data)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ZITADEL token introspection failed: %d", resp.StatusCode)
	}
	if !payload.Active {
		return nil, errors.New("ZITADEL token introspection returned an inactive token; check whether the UI and API are using the same ZITADEL issuer/client configuration")
	}
	return &payload, nil
}

func (v *verifier) getDiscovery(ctx context.Context) (discoveryDocument, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.discovery.IntrospectionEndpoint != "" {
		return v.discovery, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.IssuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return discoveryDocument{}, err
	}

	resp, err := v.cfg.HTTPClient.Do(req)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("ZITADEL discovery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discoveryDocument{}, fmt.Errorf("ZITADEL discovery failed: %d", resp.StatusCode)
	}

	var discovery discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return discoveryDocument{}, fmt.Errorf("ZITADEL discovery response is invalid: %w", err)
	}

	v.discovery = discovery
	return discovery, nil
}
