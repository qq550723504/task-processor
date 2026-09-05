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

func TestDiagnosticFixedEncodingVectors(t *testing.T) {
	for _, raw := range []string{`{}`, `{"review_notes":null}`, `{"review_notes":[]}`} {
		got, err := (DiagnosticValidator{}).Validate(diagnosticRequest(raw))
		if err != nil {
			t.Fatal(err)
		}
		const emptyDigest = "sha256:5040e66f7fef34038805993a390b33bee3d409e1e07dd652402bdfebc483b6d1"
		if got.Input.Digest != emptyDigest {
			t.Errorf("empty normalized vector: %s", got.Input.Digest)
		}
	}
	for _, raw := range []string{`{"preview_payload":{"spu_name":"sku"}}`, `{"preview_product":{"spu_name":"sku"}}`, `{"preview_payload":{"spu_name":"sku"},"preview_product":{"spu_name":"sku"}}`} {
		got, err := (DiagnosticValidator{}).Validate(diagnosticRequest(raw))
		if err != nil {
			t.Fatal(err)
		}
		const sdkDigest = "sha256:86e32324068a6515b6d6e22d17e00f090ee3a0a2def733d2bbcd3517f3b3e701"
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
