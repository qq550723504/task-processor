package openmeter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

var pocRunIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const pocBaseURL = "http://127.0.0.1:48888/api/v3"

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
	if environment.BaseURL != pocBaseURL {
		return pocEnvironment{}, fmt.Errorf("OPENMETER_POC_URL must be exactly %s when OPENMETER_POC=1", pocBaseURL)
	}
	if len(environment.RunID) > 40 || !pocRunIDPattern.MatchString(environment.RunID) {
		return pocEnvironment{}, fmt.Errorf("OPENMETER_POC_RUN_ID must match %s and contain at most 40 characters", pocRunIDPattern.String())
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
	resourcePrefix := "poc_" + strings.ReplaceAll(runID, "-", "_")
	subjectPrefix := "poc-" + runID
	return pocNames{
		StudioMeterKey:    resourcePrefix + "_studio_meter",
		SheinMeterKey:     resourcePrefix + "_shein_meter",
		StorageMeterKey:   resourcePrefix + "_storage_meter",
		StudioFeatureKey:  resourcePrefix + "_studio_feature",
		SheinFeatureKey:   resourcePrefix + "_shein_feature",
		StorageFeatureKey: resourcePrefix + "_storage_feature",
		CustomerAKey:      resourcePrefix + "_customer_a",
		CustomerBKey:      resourcePrefix + "_customer_b",
		SubjectA:          "tenant:" + subjectPrefix + "-a",
		SubjectB:          "tenant:" + subjectPrefix + "-b",
		PlanKey:           resourcePrefix + "_plan",
		PhaseKey:          resourcePrefix + "_phase",
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
		{name: "repeated run ID separator", url: "http://127.0.0.1:48888/api/v3", runID: "run--42", wantError: "OPENMETER_POC_RUN_ID"},
		{name: "trailing run ID separator", url: "http://127.0.0.1:48888/api/v3", runID: "run-", wantError: "OPENMETER_POC_RUN_ID"},
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

func TestLoadPoCEnvironmentRejectsEveryNonExactLocalEndpoint(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "remote host", url: "http://openmeter.example:48888/api/v3"},
		{name: "non-loopback address", url: "http://192.0.2.10:48888/api/v3"},
		{name: "localhost alias", url: "http://localhost:48888/api/v3"},
		{name: "IPv6 loopback variant", url: "http://[::1]:48888/api/v3"},
		{name: "HTTPS", url: "https://127.0.0.1:48888/api/v3"},
		{name: "alternate port", url: "http://127.0.0.1:48889/api/v3"},
		{name: "alternate path", url: "http://127.0.0.1:48888/api/v2"},
		{name: "trailing slash", url: "http://127.0.0.1:48888/api/v3/"},
		{name: "userinfo", url: "http://user:password@127.0.0.1:48888/api/v3"},
		{name: "query", url: "http://127.0.0.1:48888/api/v3?target=remote"},
		{name: "fragment", url: "http://127.0.0.1:48888/api/v3#remote"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearPoCEnvironment(t)
			t.Setenv("OPENMETER_POC", "1")
			t.Setenv("OPENMETER_POC_URL", test.url)
			t.Setenv("OPENMETER_POC_RUN_ID", "run-42")

			environment, err := loadPoCEnvironment()
			if err == nil {
				t.Fatalf("loadPoCEnvironment() = %+v, want OPENMETER_POC_URL error", environment)
			}
			if !strings.Contains(err.Error(), "OPENMETER_POC_URL") {
				t.Fatalf("loadPoCEnvironment() error = %q, want OPENMETER_POC_URL error", err)
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
		StudioMeterKey:    "poc_run_42_studio_meter",
		SheinMeterKey:     "poc_run_42_shein_meter",
		StorageMeterKey:   "poc_run_42_storage_meter",
		StudioFeatureKey:  "poc_run_42_studio_feature",
		SheinFeatureKey:   "poc_run_42_shein_feature",
		StorageFeatureKey: "poc_run_42_storage_feature",
		CustomerAKey:      "poc_run_42_customer_a",
		CustomerBKey:      "poc_run_42_customer_b",
		SubjectA:          "tenant:poc-run-42-a",
		SubjectB:          "tenant:poc-run-42-b",
		PlanKey:           "poc_run_42_plan",
		PhaseKey:          "poc_run_42_phase",
	}
	if first != want {
		t.Fatalf("pocNamesForRunID() = %+v, want %+v", first, want)
	}
}

func TestPoCNamesUseServerCompatibleResourceKeys(t *testing.T) {
	names := pocNamesForRunID("om-20260813-220837")
	plan, err := pocPlanRequest(names, "feature-studio", "feature-shein", "feature-storage")
	if err != nil {
		t.Fatalf("pocPlanRequest() error = %v", err)
	}

	keys := []string{
		names.StudioMeterKey,
		names.SheinMeterKey,
		names.StorageMeterKey,
		names.StudioFeatureKey,
		names.SheinFeatureKey,
		names.StorageFeatureKey,
		names.CustomerAKey,
		names.CustomerBKey,
		names.PlanKey,
		names.PhaseKey,
	}
	for _, phase := range plan.Phases {
		for _, rateCard := range phase.RateCards {
			keys = append(keys, rateCard.Key)
		}
	}

	serverPattern := regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)
	for _, key := range keys {
		if !serverPattern.MatchString(key) {
			t.Errorf("OpenMeter resource key %q does not match %s", key, serverPattern)
		}
		if len(key) > 64 {
			t.Errorf("OpenMeter resource key %q is %d bytes, want at most 64", key, len(key))
		}
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
