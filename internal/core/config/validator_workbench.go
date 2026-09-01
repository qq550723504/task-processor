package config

import (
	"net/url"
	"strings"
)

// ValidateWorkbenchConfig requires the complete ZITADEL read path only when
// the neutral workbench feature is enabled.
func ValidateWorkbenchConfig(workbench *WorkbenchConfig, zitadel *ListingKitZitadelConfig) []error {
	if workbench == nil || !workbench.Enabled {
		return nil
	}

	var errors []error
	if zitadel == nil || strings.TrimSpace(zitadel.ProjectID) == "" {
		errors = append(errors, &ValidationError{
			Field:   "listingkit.zitadel.projectID",
			Message: "is required when workbench is enabled",
			Hint:    "configure the ListingKit ZITADEL project ID or disable workbench",
		})
	}
	if zitadel == nil || strings.TrimSpace(zitadel.IssuerURL) == "" {
		errors = append(errors, &ValidationError{
			Field:   "listingkit.zitadel.issuerURL",
			Message: "is required when workbench is enabled",
			Hint:    "configure the ZITADEL issuer URL or disable workbench",
		})
	}
	if zitadel == nil || strings.TrimSpace(zitadel.ClientID) == "" {
		errors = append(errors, &ValidationError{
			Field:   "listingkit.zitadel.clientID",
			Message: "is required when workbench is enabled",
			Hint:    "configure the ZITADEL client ID or disable workbench",
		})
	}
	authorizationAPIURL := ""
	if zitadel != nil {
		authorizationAPIURL = strings.TrimSpace(zitadel.AuthorizationAPIURL)
	}
	if authorizationAPIURL == "" {
		errors = append(errors, &ValidationError{
			Field:   "listingkit.zitadel.authorizationAPIURL",
			Message: "is required when workbench is enabled",
			Hint:    "configure the ZITADEL authorization API URL or disable workbench",
		})
	} else if !isSupportedAbsoluteHTTPURL(authorizationAPIURL) {
		errors = append(errors, &ValidationError{
			Field:   "listingkit.zitadel.authorizationAPIURL",
			Message: "must be an absolute HTTP(S) URL",
			Hint:    "configure an authorization API URL such as https://authorization.example",
		})
	}
	return errors
}

func isSupportedAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Hostname() == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}
