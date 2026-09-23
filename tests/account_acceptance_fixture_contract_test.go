package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountAcceptanceViewerHasNoHomeOrganizationGrant(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", "terraform", "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if strings.Contains(text, `resource "zitadel_user_grant" "viewer"`) {
		t.Fatal("the acceptance viewer must not receive a home-organization user grant")
	}
	for _, required := range []string{
		`resource "zitadel_human_user" "viewer"`,
		`output "viewer_user_id"`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("Terraform contract is missing %q", required)
		}
	}
}

func TestAccountMembershipDirectoryReaderHasReadOnlyInstanceScope(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", "terraform", "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, required := range []string{
		`resource "zitadel_instance_member" "membership_read"`,
		`user_id = zitadel_machine_user.membership_read.id`,
		`roles   = ["IAM_OWNER_VIEWER"]`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("membership directory reader must have read-only instance permission for arbitrary organization grants; missing %q", required)
		}
	}
	if strings.Contains(text, `resource "zitadel_org_member" "membership_read"`) {
		t.Fatal("an organization-scoped viewer cannot read member grants in other organizations")
	}
}
