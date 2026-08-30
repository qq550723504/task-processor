package config

import "strings"

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
	if zitadel == nil || strings.TrimSpace(zitadel.AuthorizationAPIURL) == "" {
		errors = append(errors, &ValidationError{
			Field:   "listingkit.zitadel.authorizationAPIURL",
			Message: "is required when workbench is enabled",
			Hint:    "configure the ZITADEL authorization API URL or disable workbench",
		})
	}
	return errors
}
