package listing

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

type stubListingLocalRuntimeValidator struct {
	fields map[string]bool
	err    error
}

func (s stubListingLocalRuntimeValidator) ValidateLocalListingRuntimeFields() (map[string]bool, error) {
	return s.fields, s.err
}

func TestListingRuntimeHealthValidatorPortIsOwnedByAppPorts(t *testing.T) {
	content, err := os.ReadFile("local_runtime_health.go")
	if err != nil {
		t.Fatalf("read local_runtime_health.go: %v", err)
	}

	if strings.Contains(string(content), "type ListingRuntimeHealthValidator interface {") {
		t.Fatalf("local_runtime_health.go defines ListingRuntimeHealthValidator; keep the shared port in app/ports")
	}
	if !strings.Contains(string(content), "ports.ListingRuntimeHealthValidator") {
		t.Fatalf("local_runtime_health.go should accept ports.ListingRuntimeHealthValidator")
	}
}

func TestValidateListingRuntimeHealthRequiresSheinLocalRuntime(t *testing.T) {
	validator := stubListingLocalRuntimeValidator{
		fields: map[string]bool{"ready": false},
		err:    errors.New("local runtime unavailable"),
	}

	err := ValidateListingRuntimeHealth("shein", validator, logrus.New())
	if err == nil {
		t.Fatal("ValidateListingRuntimeHealth() error = nil, want local runtime error")
	}
	if !strings.Contains(err.Error(), "SHEIN listing local runtime is not ready") {
		t.Fatalf("error = %v, want SHEIN local runtime message", err)
	}
}

func TestValidateListingRuntimeHealthSkipsOtherPlatforms(t *testing.T) {
	if err := ValidateListingRuntimeHealth("temu", nil, logrus.New()); err != nil {
		t.Fatalf("ValidateListingRuntimeHealth(temu) error = %v", err)
	}
}

func TestValidateListingRuntimeHealthReportsMissingValidator(t *testing.T) {
	err := ValidateListingRuntimeHealth("shein", nil, logrus.New())
	if err == nil {
		t.Fatal("ValidateListingRuntimeHealth() error = nil, want missing validator error")
	}
	if strings.Contains(err.Error(), "management client") {
		t.Fatalf("error = %v, should not expose retired management service as the runtime health dependency", err)
	}
	if !strings.Contains(err.Error(), "health validator is not configured") {
		t.Fatalf("error = %v, want health validator message", err)
	}
}
