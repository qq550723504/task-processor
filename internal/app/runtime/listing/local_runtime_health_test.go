package listing

import (
	"strings"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

func TestValidateListingLocalRuntimeRequiresSheinLocalRuntime(t *testing.T) {
	client := management.NewClientManager(&config.ManagementConfig{BaseURL: "http://127.0.0.1:1"})

	err := validateListingLocalRuntime("shein", client, logrus.New())
	if err == nil {
		t.Fatal("validateListingLocalRuntime() error = nil, want local runtime error")
	}
	if !strings.Contains(err.Error(), "SHEIN listing local runtime is not ready") {
		t.Fatalf("error = %v, want SHEIN local runtime message", err)
	}
}

func TestValidateListingLocalRuntimeSkipsOtherPlatforms(t *testing.T) {
	if err := validateListingLocalRuntime("temu", nil, logrus.New()); err != nil {
		t.Fatalf("validateListingLocalRuntime(temu) error = %v", err)
	}
}
