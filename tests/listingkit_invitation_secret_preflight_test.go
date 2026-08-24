package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestListingKitInvitationSecretPreflight(t *testing.T) {
	script := filepath.ToSlash(filepath.Join("..", "scripts", "validate-listingkit-invitation-secret.sh"))
	for _, tc := range []struct {
		name           string
		kubectlMode    string
		tokenValue     string
		projectIDValue string
		wantErr        string
		secretValue    string
	}{
		{
			name:        "rejects missing Secret",
			kubectlMode: "missing",
			wantErr:     "Missing required ListingKit invitation Secret: listingkit-member-invitation-secret",
		},
		{
			name:           "rejects missing token",
			projectIDValue: "sensitive-project-id",
			wantErr:        "Missing required ListingKit invitation Secret key: TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
			secretValue:    "sensitive-project-id",
		},
		{
			name:        "rejects empty project ID",
			tokenValue:  "sensitive-token",
			wantErr:     "Missing required ListingKit invitation Secret key: TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
			secretValue: "sensitive-token",
		},
		{
			name:        "rejects empty project ID despite kubectl warning",
			kubectlMode: "warning-empty-project",
			tokenValue:  "present",
			wantErr:     "Missing required ListingKit invitation Secret key: TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
		},
		{
			name:           "accepts complete Secret",
			tokenValue:     "present",
			projectIDValue: "present",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := runListingKitInvitationSecretPreflight(t, script, tc.kubectlMode, tc.tokenValue, tc.projectIDValue)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("preflight failed: %v\n%s", err, output)
				}
				return
			}
			if err == nil {
				t.Fatalf("preflight succeeded, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(output, tc.wantErr) {
				t.Fatalf("preflight output = %q, want %q", output, tc.wantErr)
			}
			if tc.secretValue != "" && strings.Contains(output, tc.secretValue) {
				t.Fatalf("preflight output leaked Secret data %q", tc.secretValue)
			}
		})
	}
}

func runListingKitInvitationSecretPreflight(t *testing.T, script, kubectlMode, tokenValue, projectIDValue string) (string, error) {
	t.Helper()
	binDir := t.TempDir()
	writePreflightFake(t, filepath.Join(binDir, "kubectl"), `#!/usr/bin/env bash
if [[ "${FAKE_KUBECTL_MODE:-ready}" == "missing" ]]; then
  printf 'Error from server (NotFound): secrets "listingkit-member-invitation-secret" not found\n' >&2
  exit 1
fi
case "$*" in
	*"jsonpath={.data.TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID}"*)
		if [[ "${FAKE_KUBECTL_MODE:-ready}" == "warning-empty-project" ]]; then
			printf 'Warning: deprecated API response field\n' >&2
			exit 0
		fi
		printf '%s' "${FAKE_PROJECT_ID_VALUE}"
		;;
	*"jsonpath={.data.TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN}"*)
	    printf '%s' "${FAKE_TOKEN_VALUE}"
	    ;;
esac
`)

	cmd := exec.Command(preflightBash(t), script, "task-processor")
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_KUBECTL_MODE="+kubectlMode,
		"FAKE_TOKEN_VALUE="+tokenValue,
		"FAKE_PROJECT_ID_VALUE="+projectIDValue,
	)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func preflightBash(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := `C:\Program Files\Git\bin\bash.exe`
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if path, err := exec.LookPath("bash"); err == nil {
		return path
	}
	t.Fatal("bash is required to execute the ListingKit invitation Secret preflight")
	return ""
}

func writePreflightFake(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write fake %s: %v", filepath.Base(path), err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatalf("chmod fake %s: %v", filepath.Base(path), err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if _, err := exec.Command("/usr/bin/env", "bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("validate fake %s: %v", filepath.Base(path), err)
	}
}
