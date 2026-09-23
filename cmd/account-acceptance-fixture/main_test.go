package main

import (
	"strings"
	"testing"
)

func TestEncodeManifestContainsOnlySanitizedFixtureFacts(t *testing.T) {
	data, err := encodeManifest(manifest{
		SchemaVersion:      1,
		Status:             "ready",
		IssuerURL:          "https://localhost:19443",
		HomeOrganizationID: "home-org",
		ProjectID:          "project",
		OperatorUserID:     "operator",
		Organizations:      []manifestOrganization{{Name: acceptanceOrganizationA, ID: "org-a", RoleKeys: []string{"listingkit_admin"}}},
		Identities:         []manifestIdentity{{Name: "viewer", Login: viewerLogin, UserID: "viewer", OrganizationName: acceptanceOrganizationA, RoleKeys: []string{"listingkit_viewer"}}},
	})
	if err != nil {
		t.Fatalf("encodeManifest() error = %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{"iam-owner.pat", "clientSecret", "password", "bearer", "token"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("manifest contains forbidden secret material %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"status", "project", "org-a", viewerLogin} {
		if !strings.Contains(text, required) {
			t.Fatalf("manifest is missing sanitized fact %q: %s", required, text)
		}
	}
}
