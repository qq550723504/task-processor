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

func TestAccountComposePlatformAdminCallerUsesBootstrapUserOutput(t *testing.T) {
	read := func(relative string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", relative))
		if err != nil {
			t.Fatal(err)
		}
		return string(contents)
	}
	terraform := read(filepath.Join("terraform", "main.tf"))
	if !strings.Contains(terraform, `output "bootstrap_user_id" { value = zitadel_human_user.operator.id }`) {
		t.Fatal("account-compose must source the configured caller from Terraform's authoritative bootstrap user output")
	}
	tofuInit := read("tofu-init.sh")
	if !strings.Contains(tofuInit, `write_output bootstrap_user_id "$runtime/bootstrap-user-id"`) {
		t.Fatal("account-compose must persist the Terraform bootstrap user ID in the private runtime volume")
	}
	initScript := read("init.sh")
	for _, required := range []string{
		`tr -d '\r\n' < "$runtime/bootstrap-user-id"`,
		`"listingKitAuthorization": {"platformAdminUsers": ["${bootstrap_user_id}"], "platformAdminRoles": []}`,
		`chmod 600 "$runtime/current-application.json.tmp"`,
		`install_listingkit_authorization`,
	} {
		if !strings.Contains(initScript, required) {
			t.Fatalf("account-compose narrow admin caller generation is missing %q", required)
		}
	}
	if strings.Contains(initScript, `echo "$bootstrap_user_id"`) || strings.Contains(initScript, `printf "$bootstrap_user_id"`) {
		t.Fatal("account-compose must not print the bootstrap user ID")
	}
	for _, compose := range []string{"referrals-compose", "acquisition-compose"} {
		contents, err := os.ReadFile(filepath.Join("..", "deployments", "docker", compose, "init.sh"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(contents), "bootstrap-user-id") || strings.Contains(string(contents), "listingKitAuthorization") {
			t.Fatalf("%s must not inherit account-compose's platform-admin caller", compose)
		}
	}
}

func TestAccountComposeBackfillsBootstrapUserFromCompletedOpenTofuState(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", "tofu-init.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	marker := strings.Index(text, `if [ -f "$state/.terraform-complete" ]; then`)
	if marker < 0 {
		t.Fatal("account-compose must preserve the completed OpenTofu state fast path")
	}
	branchEnd := strings.Index(text[marker:], "\nif [ -f \"$state/.terraform-started\" ]; then")
	if branchEnd < 0 {
		t.Fatal("completed OpenTofu state fast path is not terminated")
	}
	branch := text[marker : marker+branchEnd]
	if !strings.Contains(branch, `if [ ! -s "$runtime/bootstrap-user-id" ]; then`) || !strings.Contains(branch, `write_output bootstrap_user_id "$runtime/bootstrap-user-id"`) {
		t.Fatal("completed OpenTofu state must backfill a missing bootstrap identity output into the private runtime volume")
	}
	if strings.Contains(branch, "tofu init") || strings.Contains(branch, "tofu apply") {
		t.Fatal("bootstrap identity backfill must read retained state without reinitializing providers or applying infrastructure")
	}
	outputReader := strings.Index(text, `tofu output -state="$state/terraform.tfstate" -raw "$1"`)
	if outputReader < 0 || outputReader > marker {
		t.Fatal("authoritative state output reader must be available before the completed-state fast path")
	}
}

func TestAccountComposeCommercialOverviewUsesCurrentApplication(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "COMMERCIAL_API_ORIGIN: http://127.0.0.1:8085") {
		t.Fatal("account-compose listingkit-ui must route commercial overview to the shared current-application listener")
	}
}

func TestAccountComposeMigratesCommercialOwnerSchemaBeforeApplicationStartup(t *testing.T) {
	initScript, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", "init.sh"))
	if err != nil {
		t.Fatal(err)
	}
	initText := string(initScript)
	if strings.Count(initText, "migrate_commercial_owner_schema\n") != 2 {
		t.Fatal("account-compose must run the commercial owner migrator in both fresh and repeated initialization")
	}
	for _, required := range []string{
		`"user":"postgres"`,
		`"maxConnections":2`,
		`"port":5435`,
		`"database":"referrals"`,
		`commercial-owner-schema-migrate -config "$work/commercial-owner-schema.json" -money-config "$work/canonical-money-schema.json"`,
	} {
		if !strings.Contains(initText, required) {
			t.Fatalf("account-compose commercial owner migration is missing %q", required)
		}
	}
	if strings.Count(initText, `-money-config "$work/canonical-money-schema.json"`) != 1 || strings.Count(initText, "migrate_commercial_owner_schema\n") != 2 {
		t.Fatal("both fresh and repeated account-compose initialization must migrate wallet schema on the canonical money database")
	}
	if strings.Count(initText, `-f "$terraform_source/referral-grants.sql"`+"\n  migrate_commercial_owner_schema") != 1 || strings.Count(initText, `-f "$terraform_source/referral-grants.sql"`+"\nmigrate_commercial_owner_schema") != 1 {
		t.Fatal("canonical money migration must run after referral runtime role and base schema setup in both initialization paths")
	}

	dockerfile, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "Dockerfile.account-compose"))
	if err != nil {
		t.Fatal(err)
	}
	dockerText := string(dockerfile)
	for _, required := range []string{
		`COPY cmd/commercial-owner-schema-migrate ./cmd/commercial-owner-schema-migrate`,
		`-o /out/commercial-owner-schema-migrate ./cmd/commercial-owner-schema-migrate`,
		`COPY --from=go-builder /out/commercial-owner-schema-migrate /usr/local/bin/commercial-owner-schema-migrate`,
	} {
		if !strings.Contains(dockerText, required) {
			t.Fatalf("account-compose schema-init image is missing %q", required)
		}
	}
}
