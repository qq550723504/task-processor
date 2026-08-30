package imageagentacceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRuntimeConfigParsesOnlyTheAcceptanceRuntimeFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.env")
	content := strings.Join([]string{
		"LISTINGKIT_ACCEPTANCE_DATABASE_DSN=postgres://acceptance@localhost/image_agent_acceptance",
		"LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER=marker-1",
		"LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT=task-processor-image-agent-acceptance",
		"ZITADEL_ISSUER_URL=https://zitadel.example.test",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=project-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID=api-client",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET=api-secret",
		"# comments and blank lines are allowed",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadRuntimeConfig(path)
	if err != nil {
		t.Fatalf("LoadRuntimeConfig() error = %v", err)
	}
	want := RuntimeConfig{
		DatabaseDSN:       "postgres://acceptance@localhost/image_agent_acceptance",
		EnvironmentMarker: "marker-1",
		ComposeProject:    "task-processor-image-agent-acceptance",
		IssuerURL:         "https://zitadel.example.test",
		ProjectID:         "project-1",
		APIClientID:       "api-client",
		APIClientSecret:   "api-secret",
	}
	if got != want {
		t.Fatalf("LoadRuntimeConfig() = %+v, want %+v", got, want)
	}
}

func TestLoadRuntimeConfigRejectsMissingOrUnknownFields(t *testing.T) {
	fields := map[string]string{
		"LISTINGKIT_ACCEPTANCE_DATABASE_DSN":                  "postgres://acceptance@localhost/image_agent_acceptance",
		"LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER":            "marker-1",
		"LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT":               "task-processor-image-agent-acceptance",
		"ZITADEL_ISSUER_URL":                                  "https://zitadel.example.test",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID":        "project-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID":     "api-client",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET": "api-secret",
	}
	for _, field := range []string{
		"LISTINGKIT_ACCEPTANCE_DATABASE_DSN",
		"LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER",
		"LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT",
		"ZITADEL_ISSUER_URL",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET",
	} {
		t.Run("missing "+field, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "runtime.env")
			copy := make(map[string]string, len(fields))
			for key, value := range fields {
				copy[key] = value
			}
			delete(copy, field)
			if err := os.WriteFile(path, []byte(runtimeEnvContent(copy)), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadRuntimeConfig(path); err == nil {
				t.Fatalf("LoadRuntimeConfig() accepted runtime file missing %s", field)
			}
		})
	}

	path := filepath.Join(t.TempDir(), "runtime.env")
	fields["UNSUPPORTED_RUNTIME_FIELD"] = "must-not-be-accepted"
	if err := os.WriteFile(path, []byte(runtimeEnvContent(fields)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntimeConfig(path); err == nil {
		t.Fatal("LoadRuntimeConfig() accepted an unsupported runtime field")
	}
}

func TestLoadRuntimeConfigAcceptsKnownProvisionCompanions(t *testing.T) {
	content := strings.Join([]string{
		"LISTINGKIT_ACCEPTANCE_DATABASE_DSN=postgres://acceptance@localhost/image_agent_acceptance",
		"LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER=marker-1",
		"LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT=task-processor-image-agent-acceptance",
		"ZITADEL_ISSUER_URL=http://localhost:8080",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=project-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID=api-client",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET=api-secret",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_MANAGEMENT_TOKEN=management-secret",
		"ZITADEL_CLIENT_ID=oidc-client",
		"ZITADEL_CLIENT_SECRET=oidc-secret",
	}, "\n")
	got, err := ParseRuntimeConfig([]byte(content))
	if err != nil {
		t.Fatalf("ParseRuntimeConfig() error = %v", err)
	}
	if got.APIClientID != "api-client" || got.APIClientSecret != "api-secret" {
		t.Fatalf("ParseRuntimeConfig() = %+v", got)
	}
}

func runtimeEnvContent(fields map[string]string) string {
	keys := []string{
		"LISTINGKIT_ACCEPTANCE_DATABASE_DSN",
		"LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER",
		"LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT",
		"ZITADEL_ISSUER_URL",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET",
		"UNSUPPORTED_RUNTIME_FIELD",
	}
	var content strings.Builder
	for _, key := range keys {
		if value, ok := fields[key]; ok {
			content.WriteString(key)
			content.WriteByte('=')
			content.WriteString(value)
			content.WriteByte('\n')
		}
	}
	return content.String()
}
