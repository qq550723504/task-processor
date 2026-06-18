package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated(t *testing.T) {
	typesSource := readHTTPAPIBoundaryFile(t, "types.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"type sharedRuntimeDeps struct",
		"openaiMgr",
		"aiCredentialStore",
	} {
		if strings.Contains(typesSource, marker) {
			t.Fatalf("types.go should keep concrete external-client runtime deps in runtime_shared_deps.go; found %s", marker)
		}
	}

	runtimeDepsSource := readHTTPAPIBoundaryFile(t, "runtime_shared_deps.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"type sharedRuntimeDeps struct",
		"openaiMgr",
		"aiCredentialStore",
	} {
		if !strings.Contains(runtimeDepsSource, marker) {
			t.Fatalf("runtime_shared_deps.go missing %s", marker)
		}
	}
}

func readHTTPAPIBoundaryFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
