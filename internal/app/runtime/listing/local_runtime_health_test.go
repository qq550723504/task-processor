package listing

import (
	"errors"
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
		t.Fatalf("error = %v, should not expose Management Client as the runtime health dependency", err)
	}
	if !strings.Contains(err.Error(), "health validator is not configured") {
		t.Fatalf("error = %v, want health validator message", err)
	}
}
