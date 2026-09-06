package validator

import (
	"strings"
	"testing"
	"time"
)

func TestExternalFreshnessExplicitCoverageAndAdverseEvidence(t *testing.T) {
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	digest := "sha256:" + strings.Repeat("a", 64)
	valid := func() ExternalFreshness {
		return ExternalFreshness{Status: FreshnessValid, Coverage: []string{ExternalPackageFreshness}, Evidence: &FreshnessEvidence{SubjectDigest: digest, PolicyVersion: "owner-policy.v1", Source: "trusted-owner", ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)}}
	}
	if err := valid().RequireFreshness(digest, now); err != nil {
		t.Fatal(err)
	}
	unknown := ExternalFreshness{Status: NotEvaluated}
	if err := unknown.Validate(digest, now); err != nil {
		t.Fatal(err)
	}
	if err := unknown.RequireFreshness(digest, now); err == nil {
		t.Fatal("unknown met freshness requirement")
	}
	for name, change := range map[string]func(*ExternalFreshness){
		"expiry boundary":       func(f *ExternalFreshness) { f.Evidence.ValidUntil = now },
		"stale":                 func(f *ExternalFreshness) { f.Status = FreshnessStale },
		"expired":               func(f *ExternalFreshness) { f.Status = FreshnessExpired },
		"different subject":     func(f *ExternalFreshness) { f.Evidence.SubjectDigest = "sha256:" + strings.Repeat("b", 64) },
		"missing policy":        func(f *ExternalFreshness) { f.Evidence.PolicyVersion = "" },
		"invalid source":        func(f *ExternalFreshness) { f.Evidence.Source = "\xff" },
		"future observation":    func(f *ExternalFreshness) { f.Evidence.ObservedAt = now.Add(time.Second) },
		"invalid interval":      func(f *ExternalFreshness) { f.Evidence.ValidUntil = f.Evidence.ObservedAt },
		"partial":               func(f *ExternalFreshness) { f.Coverage = []string{"template_only"} },
		"missing evidence":      func(f *ExternalFreshness) { f.Evidence = nil },
		"unknown with evidence": func(f *ExternalFreshness) { f.Status = NotEvaluated },
	} {
		t.Run(name, func(t *testing.T) {
			f := valid()
			change(&f)
			if err := f.Validate(digest, now); err == nil {
				t.Fatal("accepted invalid/adverse evidence")
			}
		})
	}
	f := valid()
	copy := f.Clone()
	copy.Coverage[0] = "changed"
	copy.Evidence.Source = "changed"
	if f.Coverage[0] != ExternalPackageFreshness || f.Evidence.Source != "trusted-owner" {
		t.Fatal("evidence alias")
	}
}

func TestPartialAdverseFreshnessRemainsStale(t *testing.T) {
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	digest := "sha256:" + strings.Repeat("a", 64)
	f := ExternalFreshness{Status: FreshnessStale, Coverage: []string{"online_template_freshness"}, Evidence: &FreshnessEvidence{SubjectDigest: digest, Source: "owner", PolicyVersion: "v1", ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)}}
	for _, status := range []FreshnessStatus{FreshnessStale, FreshnessExpired} {
		f.Status = status
		err, ok := f.Validate(digest, now).(*Error)
		if !ok || err.Code != StaleInput {
			t.Fatalf("lost partial stale code: %v", err)
		}
	}

}
