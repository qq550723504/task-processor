package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestBootstrapDoesNotThreadRetiredManagementAuthClient(t *testing.T) {
	t.Parallel()

	files := []string{"app.go", "component_adapters.go"}
	for _, name := range files {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		for _, marker := range []string{
			`"task-processor/internal/infra/auth"`,
			"authClient",
			"ClientCredentialsAuthClient",
			"StartProcessors(ctx, t.cfg, t.authClient)",
		} {
			if strings.Contains(string(content), marker) {
				t.Fatalf("%s mentions %q; retired management auth should not be threaded through application bootstrap", name, marker)
			}
		}
	}
}
