package openmeter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

var pocRunIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

type pocEnvironment struct {
	Enabled bool
	BaseURL string
	RunID   string
	APIKey  string
	Phase   string
}

type pocNames struct {
	StudioMeterKey    string
	SheinMeterKey     string
	StorageMeterKey   string
	StudioFeatureKey  string
	SheinFeatureKey   string
	StorageFeatureKey string
	CustomerAKey      string
	CustomerBKey      string
	SubjectA          string
	SubjectB          string
	PlanKey           string
	PhaseKey          string
}

func loadPoCEnvironment() (pocEnvironment, error) {
	if os.Getenv("OPENMETER_POC") != "1" {
		return pocEnvironment{}, nil
	}

	environment := pocEnvironment{
		Enabled: true,
		BaseURL: os.Getenv("OPENMETER_POC_URL"),
		RunID:   os.Getenv("OPENMETER_POC_RUN_ID"),
		APIKey:  os.Getenv("OPENMETER_API_KEY"),
		Phase:   os.Getenv("OPENMETER_POC_PHASE"),
	}
	if environment.BaseURL == "" {
		return pocEnvironment{}, fmt.Errorf("OPENMETER_POC_URL is required when OPENMETER_POC=1")
	}
	if !pocRunIDPattern.MatchString(environment.RunID) {
		return pocEnvironment{}, fmt.Errorf("OPENMETER_POC_RUN_ID must match %s", pocRunIDPattern.String())
	}
	if !isPoCPhase(environment.Phase) {
		return pocEnvironment{}, fmt.Errorf("OPENMETER_POC_PHASE must be empty, contract, seed, unavailable, or replay")
	}

	return environment, nil
}

func isPoCPhase(phase string) bool {
	switch phase {
	case "", "contract", "seed", "unavailable", "replay":
		return true
	default:
		return false
	}
}

func pocNamesForRunID(runID string) pocNames {
	prefix := "poc-" + runID
	return pocNames{
		StudioMeterKey:    prefix + "-studio-meter",
		SheinMeterKey:     prefix + "-shein-meter",
		StorageMeterKey:   prefix + "-storage-meter",
		StudioFeatureKey:  prefix + "-studio-feature",
		SheinFeatureKey:   prefix + "-shein-feature",
		StorageFeatureKey: prefix + "-storage-feature",
		CustomerAKey:      prefix + "-customer-a",
		CustomerBKey:      prefix + "-customer-b",
		SubjectA:          "tenant:" + prefix + "-a",
		SubjectB:          "tenant:" + prefix + "-b",
		PlanKey:           prefix + "-plan",
		PhaseKey:          prefix + "-phase",
	}
}

func TestLoadPoCEnvironmentSkipsOnlyWhenNotEnabled(t *testing.T) {
	clearPoCEnvironment(t)

	environment, err := loadPoCEnvironment()
	if err != nil {
		t.Fatalf("loadPoCEnvironment() error = %v", err)
	}
	if environment.Enabled {
		t.Fatal("loadPoCEnvironment() enabled = true, want false")
	}

	t.Setenv("OPENMETER_POC", "0")
	environment, err = loadPoCEnvironment()
	if err != nil {
		t.Fatalf("loadPoCEnvironment() with disabled flag error = %v", err)
	}
	if environment.Enabled {
		t.Fatal("loadPoCEnvironment() with disabled flag enabled = true, want false")
	}
}

func TestLoadPoCEnvironmentFailsWhenEnabledWithoutURLOrRunID(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		runID     string
		phase     string
		wantError string
	}{
		{name: "missing URL", runID: "run-42", wantError: "OPENMETER_POC_URL"},
		{name: "missing run ID", url: "http://127.0.0.1:48888/api/v3", wantError: "OPENMETER_POC_RUN_ID"},
		{name: "unsanitized run ID", url: "http://127.0.0.1:48888/api/v3", runID: "Run_42", wantError: "OPENMETER_POC_RUN_ID"},
		{name: "overlong run ID", url: "http://127.0.0.1:48888/api/v3", runID: strings.Repeat("a", 41), wantError: "OPENMETER_POC_RUN_ID"},
		{name: "unsupported phase", url: "http://127.0.0.1:48888/api/v3", runID: "run-42", phase: "cleanup", wantError: "OPENMETER_POC_PHASE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearPoCEnvironment(t)
			t.Setenv("OPENMETER_POC", "1")
			t.Setenv("OPENMETER_POC_URL", test.url)
			t.Setenv("OPENMETER_POC_RUN_ID", test.runID)
			t.Setenv("OPENMETER_POC_PHASE", test.phase)

			environment, err := loadPoCEnvironment()
			if err == nil {
				t.Fatalf("loadPoCEnvironment() = %+v, want error containing %q", environment, test.wantError)
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("loadPoCEnvironment() error = %q, want error containing %q", err, test.wantError)
			}
		})
	}
}

func TestLoadPoCEnvironmentAcceptsOptionalAPIKey(t *testing.T) {
	for _, phase := range []string{"", "contract", "seed", "unavailable", "replay"} {
		t.Run("phase_"+phase, func(t *testing.T) {
			clearPoCEnvironment(t)
			t.Setenv("OPENMETER_POC", "1")
			t.Setenv("OPENMETER_POC_URL", "http://127.0.0.1:48888/api/v3")
			t.Setenv("OPENMETER_POC_RUN_ID", "run-42")
			t.Setenv("OPENMETER_API_KEY", "test-token")
			t.Setenv("OPENMETER_POC_PHASE", phase)

			environment, err := loadPoCEnvironment()
			if err != nil {
				t.Fatalf("loadPoCEnvironment() error = %v", err)
			}
			if !environment.Enabled {
				t.Fatal("loadPoCEnvironment() enabled = false, want true")
			}
			if environment.BaseURL != "http://127.0.0.1:48888/api/v3" {
				t.Fatalf("loadPoCEnvironment() base URL = %q", environment.BaseURL)
			}
			if environment.RunID != "run-42" {
				t.Fatalf("loadPoCEnvironment() run ID = %q", environment.RunID)
			}
			if environment.APIKey != "test-token" {
				t.Fatalf("loadPoCEnvironment() API key = %q", environment.APIKey)
			}
			if environment.Phase != phase {
				t.Fatalf("loadPoCEnvironment() phase = %q, want %q", environment.Phase, phase)
			}
		})
	}
}

func TestPoCNamesDeriveOnlyFromSanitizedRunID(t *testing.T) {
	first := pocNamesForRunID("run-42")
	second := pocNamesForRunID("run-42")
	other := pocNamesForRunID("run-43")

	if first != second {
		t.Fatalf("pocNamesForRunID() is not deterministic: first = %+v, second = %+v", first, second)
	}
	if first == other {
		t.Fatalf("pocNamesForRunID() did not isolate runs: run-42 = %+v, run-43 = %+v", first, other)
	}

	want := pocNames{
		StudioMeterKey:    "poc-run-42-studio-meter",
		SheinMeterKey:     "poc-run-42-shein-meter",
		StorageMeterKey:   "poc-run-42-storage-meter",
		StudioFeatureKey:  "poc-run-42-studio-feature",
		SheinFeatureKey:   "poc-run-42-shein-feature",
		StorageFeatureKey: "poc-run-42-storage-feature",
		CustomerAKey:      "poc-run-42-customer-a",
		CustomerBKey:      "poc-run-42-customer-b",
		SubjectA:          "tenant:poc-run-42-a",
		SubjectB:          "tenant:poc-run-42-b",
		PlanKey:           "poc-run-42-plan",
		PhaseKey:          "poc-run-42-phase",
	}
	if first != want {
		t.Fatalf("pocNamesForRunID() = %+v, want %+v", first, want)
	}
}

func clearPoCEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"OPENMETER_POC",
		"OPENMETER_POC_URL",
		"OPENMETER_POC_RUN_ID",
		"OPENMETER_API_KEY",
		"OPENMETER_POC_PHASE",
	} {
		t.Setenv(name, "")
	}
}
