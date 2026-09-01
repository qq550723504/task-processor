package config

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWorkbenchZitadelConfigRequiresCompleteInputsWhenEnabled(t *testing.T) {
	testCases := []struct {
		name    string
		zitadel ListingKitZitadelConfig
		field   string
	}{
		{
			name:    "missing project id",
			zitadel: ListingKitZitadelConfig{IssuerURL: "https://issuer.example", AuthorizationAPIURL: "https://issuer.example", ClientID: "client-1", ClientSecret: "secret-1"},
			field:   "listingkit.zitadel.projectID",
		},
		{
			name:    "missing issuer URL",
			zitadel: ListingKitZitadelConfig{ProjectID: "project-1", AuthorizationAPIURL: "https://authorization.example", ClientID: "client-1", ClientSecret: "secret-1"},
			field:   "listingkit.zitadel.issuerURL",
		},
		{
			name:    "missing authorization API URL",
			zitadel: ListingKitZitadelConfig{ProjectID: "project-1", IssuerURL: "https://issuer.example", ClientID: "client-1", ClientSecret: "secret-1"},
			field:   "listingkit.zitadel.authorizationAPIURL",
		},
		{
			name: "missing client ID",
			zitadel: ListingKitZitadelConfig{
				ProjectID:           "project-1",
				IssuerURL:           "https://issuer.example",
				AuthorizationAPIURL: "https://authorization.example",
				ClientSecret:        "secret-1",
			},
			field: "listingkit.zitadel.clientID",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateWorkbenchConfig(&WorkbenchConfig{Enabled: true}, &tt.zitadel)
			require.Len(t, errors, 1)
			assert.Contains(t, errors[0].Error(), tt.field)
		})
	}
}

func TestValidateWorkbenchZitadelConfigAllowsDisabledOrCompleteConfiguration(t *testing.T) {
	assert.Empty(t, ValidateWorkbenchConfig(
		&WorkbenchConfig{},
		&ListingKitZitadelConfig{},
	))
	assert.Empty(t, ValidateWorkbenchConfig(
		&WorkbenchConfig{Enabled: true},
		&ListingKitZitadelConfig{
			ProjectID:           "project-1",
			IssuerURL:           "https://issuer.example",
			AuthorizationAPIURL: "https://authorization.example",
			ClientID:            "client-1",
			ClientSecret:        "secret-1",
		},
	))
}

func TestValidateWorkbenchZitadelConfigRejectsUnsupportedAuthorizationAPIURL(t *testing.T) {
	for _, raw := range []string{
		"authorization.example",
		"ftp://authorization.example",
		"https://",
		"https://authorization.example/%zz",
	} {
		t.Run(raw, func(t *testing.T) {
			errors := ValidateWorkbenchConfig(
				&WorkbenchConfig{Enabled: true},
				&ListingKitZitadelConfig{
					ProjectID:           "project-1",
					IssuerURL:           "https://issuer.example",
					AuthorizationAPIURL: raw,
					ClientID:            "client-1",
					ClientSecret:        "secret-1",
				},
			)
			require.Len(t, errors, 1)
			assert.Contains(t, errors[0].Error(), "authorizationAPIURL")
			assert.Contains(t, errors[0].Error(), "absolute HTTP(S)")
		})
	}
}

func TestValidateWorkbenchZitadelConfigRejectsUnsupportedIssuerURL(t *testing.T) {
	for _, raw := range []string{"zitadel.internal", "ftp://issuer.example", "https://", "https://issuer.example?tenant=a", "https://issuer.example?", "https://issuer.example#fragment", "https://issuer.example#"} {
		t.Run(raw, func(t *testing.T) {
			errors := ValidateWorkbenchConfig(&WorkbenchConfig{Enabled: true}, &ListingKitZitadelConfig{
				ProjectID: "project-1", IssuerURL: raw, AuthorizationAPIURL: "https://authorization.example", ClientID: "client-1", ClientSecret: "secret-1",
			})
			require.Len(t, errors, 1)
			assert.Contains(t, errors[0].Error(), "issuerURL")
			assert.Contains(t, errors[0].Error(), "absolute HTTP(S)")
		})
	}
}

func TestValidateWorkbenchZitadelConfigRequiresClientSecret(t *testing.T) {
	errors := ValidateWorkbenchConfig(&WorkbenchConfig{Enabled: true}, &ListingKitZitadelConfig{
		ProjectID: "project-1", IssuerURL: "https://issuer.example", AuthorizationAPIURL: "https://authorization.example", ClientID: "client-1",
	})
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].Error(), "clientSecret")
}

func TestConfigValidateWithErrorIncludesWorkbenchZitadelConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.OpenAI.APIKey = "test-openai-key"
	cfg.Workbench.Enabled = true
	cfg.ListingKit.Zitadel.IssuerURL = "https://issuer.example"
	cfg.ListingKit.Zitadel.AuthorizationAPIURL = "https://issuer.example"

	err := cfg.ValidateWithError()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listingkit.zitadel.projectID")
}

func TestConfigManagerValidateIncludesWorkbenchZitadelConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.OpenAI.APIKey = "test-openai-key"
	cfg.Workbench.Enabled = true
	cfg.ListingKit.Zitadel.IssuerURL = "https://issuer.example"
	cfg.ListingKit.Zitadel.AuthorizationAPIURL = "https://issuer.example"
	manager := NewConfigManager(logrus.New())

	err := manager.Validate(cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listingkit.zitadel.projectID")
	require.Error(t, manager.Validate(nil))
}
