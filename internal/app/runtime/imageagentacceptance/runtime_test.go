package imageagentacceptanceruntime

import (
	"context"
	"io"
	"strings"
	"testing"

	"task-processor/internal/listingkit/imageagentacceptance"
)

func TestSeedCommandRequiresLocalFilesAndPublicSourceURL(t *testing.T) {
	err := Run(context.Background(), []string{
		"-runtime-file", "", "-token-file", "", "-source-url", "http://localhost/a.png",
	}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-runtime-file") {
		t.Fatalf("Run() error = %v, want required local-file flags", err)
	}
}

func TestVerifierConfigCarriesAcceptanceProjectID(t *testing.T) {
	cfg := verifierConfig(imageagentacceptance.RuntimeConfig{
		IssuerURL:       "http://localhost:8080",
		ProjectID:       "project-1",
		APIClientID:     "api-client",
		APIClientSecret: "api-secret",
	})

	if cfg.ProjectID != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", cfg.ProjectID)
	}
}

func TestValidatePostgresBindingsRequiresExactLoopbackAcceptancePort(t *testing.T) {
	tests := []struct {
		name string
		json string
		want bool
	}{
		{name: "exact", json: `{"5432/tcp":[{"HostIp":"127.0.0.1","HostPort":"15433"}]}`, want: true},
		{name: "wildcard", json: `{"5432/tcp":[{"HostIp":"0.0.0.0","HostPort":"15433"}]}`},
		{name: "wrong port", json: `{"5432/tcp":[{"HostIp":"127.0.0.1","HostPort":"5432"}]}`},
		{name: "multiple bindings", json: `{"5432/tcp":[{"HostIp":"127.0.0.1","HostPort":"15433"},{"HostIp":"::1","HostPort":"15433"}]}`},
		{name: "wrong target", json: `{"15433/tcp":[{"HostIp":"127.0.0.1","HostPort":"15433"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validatePostgresBindings([]byte(tt.json)); got != tt.want {
				t.Fatalf("validatePostgresBindings() = %v, want %v", got, tt.want)
			}
		})
	}
}
