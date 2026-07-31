package tenantdirectory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Tenant is the safe subset of ZITADEL organization data used by ListingKit.
type Tenant struct {
	ID            string `json:"tenant_id"`
	DisplayName   string `json:"tenant_display_name"`
	PrimaryDomain string `json:"primary_domain,omitempty"`
	State         string `json:"state,omitempty"`
}

// Directory supplies the tenant directory to platform administration handlers.
type Directory interface {
	List(ctx context.Context) ([]Tenant, error)
}

type ClientConfig struct {
	IssuerURL  string
	Token      string
	HTTPClient *http.Client
}

type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(cfg ClientConfig) (Directory, error) {
	issuerURL := strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	if issuerURL == "" {
		return nil, errors.New("ZITADEL issuer URL is required for the tenant directory")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("ZITADEL tenant directory token is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &client{baseURL: issuerURL, token: strings.TrimSpace(cfg.Token), http: httpClient}, nil
}

func (c *client) List(ctx context.Context) ([]Tenant, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/organizations/_search", bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("create ZITADEL organization list request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list ZITADEL organizations: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("list ZITADEL organizations: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Result []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			PrimaryDomain string `json:"primaryDomain"`
			State         string `json:"state"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode ZITADEL organization list: %w", err)
	}

	tenants := make([]Tenant, 0, len(payload.Result))
	for _, organization := range payload.Result {
		if strings.TrimSpace(organization.ID) == "" {
			continue
		}
		tenants = append(tenants, Tenant{
			ID:            organization.ID,
			DisplayName:   organization.Name,
			PrimaryDomain: organization.PrimaryDomain,
			State:         organization.State,
		})
	}
	return tenants, nil
}
