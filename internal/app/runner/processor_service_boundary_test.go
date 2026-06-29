package runner

import (
	"os"
	"strings"
	"testing"
)

func TestProcessorServiceDoesNotAcceptRetiredManagementAuthClient(t *testing.T) {
	t.Parallel()

	files := []string{"processor_service.go", "processor_lifecycle.go"}
	for _, name := range files {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		for _, marker := range []string{
			`"task-processor/internal/infra/auth"`,
			"auth.ClientCredentialsAuthClient",
			"authClient",
		} {
			if strings.Contains(string(content), marker) {
				t.Fatalf("%s mentions %q; processor service startup should not accept retired management auth", name, marker)
			}
		}
	}
}
