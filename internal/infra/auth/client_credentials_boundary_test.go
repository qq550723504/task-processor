package auth

import (
	"os"
	"strings"
	"testing"
)

func TestAuthPackageDoesNotCarryRetiredClientCredentialsFlow(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read auth package: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		for _, marker := range []string{
			"ClientCredentialsAuthClient",
			"ClientCredentialsClient",
			"ClientCredentialsTokenResponse",
			"BusinessTokenResponse",
			"client_credentials",
			"fetchAccessToken",
			"grant_type",
		} {
			if strings.Contains(string(content), marker) {
				t.Fatalf("%s mentions %q; retired management client credentials auth should not remain in infra/auth", entry.Name(), marker)
			}
		}
	}
}
