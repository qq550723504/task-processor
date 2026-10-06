package validator

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	contract "task-processor/internal/marketplace/validator"
)

func diagnosticRequest(raw string) contract.BoundRequest[[]byte] {
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	return contract.BoundRequest[[]byte]{Input: []byte(raw), Target: contract.Target{Marketplace: "shein"}, Action: contract.Publish, RuleVersion: DiagnosticRuleVersion, BindingVersion: BindingVersion, ReadAt: now, EvaluatedAt: now, Freshness: contract.ExternalFreshness{Status: contract.NotEvaluated}}
}

func TestMissingFinalDraftBlocksBothInputContractsAndActions(t *testing.T) {
	for _, action := range []contract.Action{contract.Publish, contract.SaveDraft} {
		t.Run(string(action), func(t *testing.T) {
			pkg := readyPackage()
			pkg.FinalSubmissionDraft = nil
			raw, err := json.Marshal(pkg)
			if err != nil {
				t.Fatal(err)
			}
			typedRequest := request(pkg)
			typedRequest.Action = action
			typed, err := (Validator{}).Validate(typedRequest)
			if err != nil {
				t.Fatal(err)
			}
			boundRequest := diagnosticRequest(string(raw))
			boundRequest.Action = action
			bound, err := (DiagnosticValidator{}).Validate(boundRequest)
			if err != nil {
				t.Fatal(err)
			}
			if !hasRule(typed.Blockers, "final_images") || !hasRule(bound.OfflineChecks.Blockers, "final_images") {
				t.Fatalf("missing final draft was not blocked: typed=%+v bound=%+v", typed.Blockers, bound.OfflineChecks.Blockers)
			}
			if !reflect.DeepEqual(typed.Checks, bound.OfflineChecks.Checks) || !bound.DiagnosticOnly || typed.Ready || typed.Status != contract.Blocked || bound.OfflineChecks.Status != contract.Blocked {
				t.Fatal("input contracts lost diagnostic rule parity")
			}
			if typed.ReadinessBlockersAllowed != (action == contract.SaveDraft) || bound.ActionPolicy.ReadinessBlockersAllowed != (action == contract.SaveDraft) {
				t.Fatal("image blocker changed the action policy")
			}
			after, err := json.Marshal(pkg)
			if err != nil || string(after) != string(raw) {
				t.Fatal("diagnostic changed the package facts", err)
			}
		})
	}
}

func TestCorrectedImageRuleVersionsRejectFormerRevisions(t *testing.T) {
	if RuleVersion != "shein.offline_package.v1.1" || DiagnosticRuleVersion != "shein.offline_package.v2.1" || BindingVersion != "shein.persisted-input.go-json.v1" {
		t.Fatal("corrected rule versions or unchanged binding version drifted")
	}
	typedRequest := request(readyPackage())
	typedRequest.RuleVersion = "shein.offline_package.v1"
	_, err := (Validator{}).Validate(typedRequest)
	var typedError *contract.Error
	if !errors.As(err, &typedError) || typedError.Code != contract.UnsupportedVersion {
		t.Fatalf("former typed revision accepted: %v", err)
	}
	boundRequest := diagnosticRequest(`{}`)
	boundRequest.RuleVersion = "shein.offline_package.v2"
	_, err = (DiagnosticValidator{}).Validate(boundRequest)
	if !errors.As(err, &typedError) || typedError.Code != contract.UnsupportedVersion {
		t.Fatalf("former persisted revision accepted: %v", err)
	}
}

func TestExactApprovedAssetValidatorDischargesOnlyExactAssetConcern(t *testing.T) {
	request := diagnosticRequest(`{"spu_name":"controlled"}`)
	want, err := (ExactApprovedAssetValidator{}).Validate(request)
	if err != nil {
		t.Fatal(err)
	}
	again, err := (ExactApprovedAssetValidator{}).Validate(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, again) {
		t.Fatal("exact-asset composition is not deterministic")
	}
	if containsDiagnosticConcern(want.NotEvaluated, "approved_asset_provenance_and_consent") {
		t.Fatal("exact approved asset concern remained unevaluated")
	}
	for _, concern := range []string{"online_template_freshness", "store_authorization", "cookie", "pod", "human_review", "submission_gate"} {
		if !containsDiagnosticConcern(want.NotEvaluated, concern) {
			t.Fatalf("remote concern %q was incorrectly discharged", concern)
		}
	}
}

func containsDiagnosticConcern(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestDiagnosticFixedEncodingVectors(t *testing.T) {
	for _, raw := range []string{`{}`, `{"review_notes":null}`, `{"review_notes":[]}`} {
		got, err := (DiagnosticValidator{}).Validate(diagnosticRequest(raw))
		if err != nil {
			t.Fatal(err)
		}
		const emptyDigest = "sha256:fcc2aaeb1c7bda2d707d423cbc296d9a2793a77f5e7c1275cabc08604f8e79fa"
		if got.Input.Digest != emptyDigest {
			t.Errorf("empty normalized vector: %s", got.Input.Digest)
		}
	}
	for _, raw := range []string{`{"preview_payload":{"spu_name":"sku"}}`, `{"preview_product":{"spu_name":"sku"}}`, `{"preview_payload":{"spu_name":"sku"},"preview_product":{"spu_name":"sku"}}`} {
		got, err := (DiagnosticValidator{}).Validate(diagnosticRequest(raw))
		if err != nil {
			t.Fatal(err)
		}
		const sdkDigest = "sha256:f5318507e79ca0f9f79488cd1c7af8698717b950bfb9a7a50473ad42cfb650ea"
		if got.Input.Digest != sdkDigest {
			t.Errorf("SDK alias vector: %s", got.Input.Digest)
		}
	}
}

func TestDiagnosticReusesRulesAndEvidenceDoesNotChangeContent(t *testing.T) {
	uuid.SetRand(forbiddenRandomReader{})
	defer uuid.SetRand(nil)
	for _, warn := range []bool{false, true} {
		pkg := readyPackage()
		if warn {
			pkg.ReviewNotes = []string{"manual review"}
		}
		raw, err := json.Marshal(pkg)
		if err != nil {
			t.Fatal(err)
		}
		req := diagnosticRequest(string(raw))
		got, err := (DiagnosticValidator{}).Validate(req)
		if err != nil {
			t.Fatal(err)
		}
		old, err := (Validator{}).Validate(request(pkg))
		if err != nil {
			t.Fatal(err)
		}
		if got.OfflineChecks.Status != old.Status || !reflect.DeepEqual(got.OfflineChecks.Checks, old.Checks) {
			t.Fatal("rule parity drift")
		}
		req.ExpectedDigest = got.Input.Digest
		req.ReadAt = req.ReadAt.Add(time.Second)
		req.EvaluatedAt = req.ReadAt
		req.Freshness = contract.ExternalFreshness{Status: contract.FreshnessValid, Coverage: []string{contract.ExternalPackageFreshness}, Evidence: &contract.FreshnessEvidence{SubjectDigest: got.Input.Digest, PolicyVersion: "owner.v1", Source: "owner", ObservedAt: req.ReadAt.Add(-time.Minute), ValidUntil: req.ReadAt.Add(time.Minute)}}
		fresh, err := (DiagnosticValidator{}).Validate(req)
		if err != nil || fresh.Input.Digest != got.Input.Digest {
			t.Fatal("time/evidence changed digest", err)
		}
		req.Freshness.Evidence.SubjectDigest = "sha256:" + strings.Repeat("b", 64)
		failed, err := (DiagnosticValidator{}).Validate(req)
		if err == nil || !reflect.DeepEqual(failed, contract.DiagnosticResult{}) {
			t.Fatal("adverse evidence lost")
		}
	}
}

func TestDiagnosticPersistentInputBinding(t *testing.T) {
	req := diagnosticRequest(`{"metadata":{"b":"2","a":"1"},"review_notes":["x","y"]}`)
	before := string(req.Input)
	got, err := (DiagnosticValidator{}).Validate(req)
	if err != nil {
		t.Fatal(err)
	}
	if !got.DiagnosticOnly || got.Freshness.Status != contract.NotEvaluated || len(got.NotEvaluated) == 0 || got.OfflineChecks.Status != contract.Blocked {
		t.Fatalf("bad scope: %+v", got)
	}
	reordered := diagnosticRequest(`{"review_notes":["x","y"],"metadata":{"a":"1","b":"2"}}`)
	same, err := (DiagnosticValidator{}).Validate(reordered)
	if err != nil || !reflect.DeepEqual(same, got) {
		t.Fatal("unstable map encoding", err)
	}
	for _, raw := range []string{`{"metadata":{"b":"3","a":"1"},"review_notes":["x","y"]}`, `{"metadata":{"b":"2","a":"1"},"review_notes":["y","x"]}`} {
		changed, err := (DiagnosticValidator{}).Validate(diagnosticRequest(raw))
		if err != nil || changed.Input.Digest == got.Input.Digest {
			t.Fatal("lost input change", err)
		}
	}
	req.Action = contract.SaveDraft
	draft, err := (DiagnosticValidator{}).Validate(req)
	if err != nil || draft.Input.Digest == got.Input.Digest || len(draft.OfflineChecks.Blockers) == 0 || !draft.ActionPolicy.ReadinessBlockersAllowed {
		t.Fatal("draft policy lost", err)
	}
	if string(req.Input) != before {
		t.Fatal("mutated bytes")
	}
	wire, _ := json.Marshal(got)
	var obj map[string]any
	_ = json.Unmarshal(wire, &obj)
	if _, ok := obj["ready"]; ok {
		t.Fatal("top level permit")
	}
}

func TestDiagnosticErrorsNeverReturnReport(t *testing.T) {
	for _, change := range []func(*contract.BoundRequest[[]byte]){
		func(r *contract.BoundRequest[[]byte]) { r.Action = contract.Preview }, func(r *contract.BoundRequest[[]byte]) { r.Target.Site = "us" },
		func(r *contract.BoundRequest[[]byte]) { r.BindingVersion = "bad" }, func(r *contract.BoundRequest[[]byte]) { r.RuleVersion = "bad" },
		func(r *contract.BoundRequest[[]byte]) { r.ReadAt = time.Time{} }, func(r *contract.BoundRequest[[]byte]) { r.EvaluatedAt = r.ReadAt.Add(-time.Second) },
		func(r *contract.BoundRequest[[]byte]) {
			r.ExpectedDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		func(r *contract.BoundRequest[[]byte]) { r.Freshness.Status = "" }, func(r *contract.BoundRequest[[]byte]) { r.Freshness.Status = contract.FreshnessExpired },
		func(r *contract.BoundRequest[[]byte]) { r.Input = []byte(`{"unknown":1}`) },
	} {
		req := diagnosticRequest(`{}`)
		change(&req)
		got, err := (DiagnosticValidator{}).Validate(req)
		var typed *contract.Error
		if !errors.As(err, &typed) || !reflect.DeepEqual(got, contract.DiagnosticResult{}) {
			t.Fatalf("bad failure: %+v %v", got, err)
		}
	}
}

func TestDiagnosticConcurrentDeterminism(t *testing.T) {
	req := diagnosticRequest(`{"spu_name":"same"}`)
	want, err := (DiagnosticValidator{}).Validate(req)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := (DiagnosticValidator{}).Validate(req)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Error("nondeterministic", err)
			}
		}()
	}
	wg.Wait()
}

func TestDiagnosticEncodingResourceFailuresAreZero(t *testing.T) {
	for name, raw := range map[string]string{
		"normalized aliases": `{"preview_payload":{"spu_name":"` + strings.Repeat("x", 1100000) + `"}}`,
		"report":             `{"metadata":{"variant_image_coverage_status":"blocked","variant_image_coverage_message":"` + strings.Repeat("x", 1100000) + `"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := (DiagnosticValidator{}).Validate(diagnosticRequest(raw))
			var typed *contract.Error
			if !errors.As(err, &typed) || typed.Code != contract.EvaluationFailed || !reflect.DeepEqual(got, contract.DiagnosticResult{}) {
				t.Fatalf("resource failure returned result: %v %s", err, got.Scope)
			}
		})
	}
}

func TestDiagnosticUnknownFreshnessReason(t *testing.T) {
	req := diagnosticRequest(`{}`)
	got, err := (DiagnosticValidator{}).Validate(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	reasons, ok := wire["not_evaluated_reasons"].(map[string]any)
	if !ok || reasons[contract.ExternalPackageFreshness] != "no_authoritative_package_freshness" {
		t.Fatal("missing authoritative freshness absence reason")
	}
	req.Freshness = contract.ExternalFreshness{Status: contract.FreshnessValid, Coverage: []string{contract.ExternalPackageFreshness}, Evidence: &contract.FreshnessEvidence{SubjectDigest: got.Input.Digest, Source: "owner", PolicyVersion: "v1", ObservedAt: req.ReadAt, ValidUntil: req.ReadAt.Add(time.Minute)}}
	fresh, err := (DiagnosticValidator{}).Validate(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(fresh)
	if strings.Contains(string(raw), "no_authoritative_package_freshness") {
		t.Fatal("valid evidence labeled absent")
	}
}

func TestDiagnosticAdverseEvidenceDetails(t *testing.T) {
	for _, scenario := range []string{"stale", "expired", "expired_at_evaluation", "subject_mismatch", "partial"} {
		t.Run(scenario, func(t *testing.T) {
			req := diagnosticRequest(`{}`)
			initial, err := (DiagnosticValidator{}).Validate(req)
			if err != nil {
				t.Fatal(err)
			}
			req.Freshness = contract.ExternalFreshness{Status: contract.FreshnessValid, Coverage: []string{contract.ExternalPackageFreshness}, Evidence: &contract.FreshnessEvidence{SubjectDigest: initial.Input.Digest, Source: "owner", PolicyVersion: "v1", ObservedAt: req.ReadAt.Add(-time.Minute), ValidUntil: req.ReadAt.Add(time.Minute)}}
			cause := scenario
			switch scenario {
			case "stale":
				req.Freshness.Status = contract.FreshnessStale
			case "expired":
				req.Freshness.Status = contract.FreshnessExpired
			case "partial":
				req.Freshness.Status = contract.FreshnessStale
				req.Freshness.Coverage = []string{"template_only"}
				cause = "stale"
			case "expired_at_evaluation":
				req.Freshness.Evidence.ValidUntil = req.EvaluatedAt
			case "subject_mismatch":
				req.Freshness.Evidence.SubjectDigest = "sha256:" + strings.Repeat("b", 64)
			}
			got, err := (DiagnosticValidator{}).Validate(req)
			var typed *contract.Error
			if !errors.As(err, &typed) || typed.Code != contract.StaleInput || !reflect.DeepEqual(got, contract.DiagnosticResult{}) {
				t.Fatal("bad rejection", err)
			}
			raw, _ := json.Marshal(typed)
			var wire map[string]any
			_ = json.Unmarshal(raw, &wire)
			detail, ok := wire["freshness"].(map[string]any)
			if !ok {
				t.Fatal("missing structured stale details")
			}
			if detail["status"] != string(req.Freshness.Status) || !reflect.DeepEqual(detail["coverage"], []any{req.Freshness.Coverage[0]}) || !reflect.DeepEqual(detail["causes"], []any{cause}) {
				t.Fatalf("lost cause/coverage: %s", raw)
			}
			req.Freshness.Coverage[0] = "caller changed"
			after, _ := json.Marshal(typed)
			if string(after) != string(raw) {
				t.Fatal("error aliases caller evidence")
			}
		})
	}
}

func TestDiagnosticClockRollbackIsEvaluationFailure(t *testing.T) {
	req := diagnosticRequest(`{}`)
	req.EvaluatedAt = req.ReadAt.Add(-time.Second)
	got, err := (DiagnosticValidator{}).Validate(req)
	var typed *contract.Error
	if !errors.As(err, &typed) || typed.Code != contract.EvaluationFailed || !reflect.DeepEqual(got, contract.DiagnosticResult{}) {
		t.Fatal("clock rollback code", err)
	}
}
