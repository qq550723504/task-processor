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
			zitadel: ListingKitZitadelConfig{IssuerURL: "https://issuer.example", AuthorizationAPIURL: "https://issuer.example", ClientID: "client-1"},
			field:   "listingkit.zitadel.projectID",
		},
		{
			name:    "missing issuer URL",
			zitadel: ListingKitZitadelConfig{ProjectID: "project-1", AuthorizationAPIURL: "https://authorization.example", ClientID: "client-1"},
			field:   "listingkit.zitadel.issuerURL",
		},
		{
			name:    "missing authorization API URL",
			zitadel: ListingKitZitadelConfig{ProjectID: "project-1", IssuerURL: "https://issuer.example", ClientID: "client-1"},
			field:   "listingkit.zitadel.authorizationAPIURL",
		},
		{
			name: "missing client ID",
			zitadel: ListingKitZitadelConfig{
				ProjectID:           "project-1",
				IssuerURL:           "https://issuer.example",
				AuthorizationAPIURL: "https://authorization.example",
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
		},
	))
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
