package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAccountComposeCommercialDatabaseRejectsCollisionsBeforeProvisioning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell is required to execute account-compose installers")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, installer := range []string{"bootstrap.sh", "business-db-init.sh"} {
		for _, name := range []string{"notification_center", "agent_customization", "ai_projects", "reports", "tool_market", "ecoservices", "postgres", "product_acquisition", "knowledge"} {
			t.Run(installer+"/rejects/"+name, func(t *testing.T) {
				output, err, provisioned := runAccountComposeDatabaseNameValidation(t, sh, installer, name)
				if err == nil || !strings.Contains(output, "commercial database must have its own name") {
					t.Fatalf("collision was not rejected by name validation: err=%v, output=%s", err, output)
				}
				if provisioned {
					t.Fatal("name collision reached secret generation or database provisioning")
				}
			})
		}
		for _, name := range []string{"9bad", "Bad", "has-dash", strings.Repeat("a", 64)} {
			t.Run(installer+"/invalid/"+name, func(t *testing.T) {
				output, err, provisioned := runAccountComposeDatabaseNameValidation(t, sh, installer, name)
				if err == nil || provisioned || !strings.Contains(output, "invalid commercial database name") {
					t.Fatalf("invalid name reached provisioning: err=%v, output=%s", err, output)
				}
			})
		}
		for _, name := range []string{"", "commercial", "isolated_commercial_trial"} {
			t.Run(installer+"/accepts/"+name, func(t *testing.T) {
				output, err, provisioned := runAccountComposeDatabaseNameValidation(t, sh, installer, name)
				if err == nil || !provisioned || !strings.Contains(output, "provisioning sentinel") {
					t.Fatalf("valid/default name did not reach provisioning: err=%v, output=%s", err, output)
				}
			})
		}
	}
}

func runAccountComposeDatabaseNameValidation(t *testing.T, sh, installer, name string) (string, error, bool) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	mutation := filepath.Join(root, "provisioned")
	// Stop at the first existing provisioning operation. Invalid names must never
	// reach mkdir (bootstrap secrets) or cat (database-owner credentials).
	for _, command := range []string{"mkdir", "cat", "psql"} {
		writeAccountComposeExecutable(t, filepath.Join(bin, command), "#!/bin/sh\nprintf called > "+shellQuote(mutation)+"\necho 'provisioning sentinel' >&2\nexit 97\n")
	}
	source := filepath.Join("..", "deployments", "docker", "account-compose")
	script, err := os.ReadFile(filepath.Join(source, installer))
	if err != nil {
		t.Fatal(err)
	}
	// The actual shared validation is copied beside this isolated script; no
	// installer secret paths or real database services are available to the test.
	validator := filepath.Join(source, "commercial-database-name.sh")
	validation, err := os.ReadFile(validator)
	if err != nil {
		t.Fatal(err)
	}
	validator = filepath.Join(root, "commercial-database-name.sh")
	if err := os.WriteFile(validator, validation, 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, installer)
	contents := strings.NewReplacer(
		"state=/state", "state="+shellQuote(filepath.Join(root, "state")),
		". /usr/local/lib/account-compose/commercial-database-name.sh", ". "+shellQuote(validator),
	).Replace(string(script))
	if err := os.WriteFile(scriptPath, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(sh, scriptPath)
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACCOUNT_COMMERCIAL_DATABASE="+name,
		"ACCOUNT_NATIVE_MODULES_ENABLED=1", "ACCOUNT_DATA_SERVICES_ENABLED=1", "ACCOUNT_KNOWLEDGE_ENABLED=1",
		"ACCOUNT_IMAGE_AGENT_TRIAL=", "ACCOUNT_ISSUE36_LOCAL_TRIAL=",
	)
	output, err := cmd.CombinedOutput()
	_, statErr := os.Stat(mutation)
	return string(output), err, statErr == nil
}
