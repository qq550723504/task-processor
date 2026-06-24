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

func TestValidateListingLocalRuntimeRequiresSheinLocalRuntime(t *testing.T) {
	validator := stubListingLocalRuntimeValidator{
		fields: map[string]bool{"ready": false},
		err:    errors.New("local runtime unavailable"),
	}

	err := validateListingLocalRuntime("shein", validator, logrus.New())
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
