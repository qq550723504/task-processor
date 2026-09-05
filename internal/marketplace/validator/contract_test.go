package validator

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSnapshotFreshnessAndErrorSemantics(t *testing.T) {
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	valid := Snapshot{Revision: "draft-1", ExpectedRevision: "draft-1", EvaluatedAt: now, ObservedAt: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)}
	cases := []struct {
		name   string
		change func(*Snapshot)
		code   ErrorCode
	}{
		{"valid", func(*Snapshot) {}, ""},
		{"missing revision", func(s *Snapshot) { s.Revision = "" }, InvalidInput},
		{"revision mismatch", func(s *Snapshot) { s.ExpectedRevision = "draft-2" }, StaleInput},
		{"expired", func(s *Snapshot) { s.ValidUntil = now }, StaleInput},
		{"future observation", func(s *Snapshot) { s.ObservedAt = now.Add(time.Second) }, InvalidInput},
		{"missing evaluation time", func(s *Snapshot) { s.EvaluatedAt = time.Time{} }, InvalidInput},
		{"missing observation time", func(s *Snapshot) { s.ObservedAt = time.Time{} }, InvalidInput},
		{"missing expiry", func(s *Snapshot) { s.ValidUntil = time.Time{} }, InvalidInput},
		{"inverted window", func(s *Snapshot) { s.ValidUntil = s.ObservedAt.Add(-time.Second) }, InvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := valid
			tc.change(&s)
			err := s.Validate()
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != tc.code {
				t.Fatalf("error=%v, want %s", err, tc.code)
			}
		})
	}
}

func TestResultOrdersFindingsWithoutLosingSameRuleAndOwnsSlices(t *testing.T) {
	checks := []Check{{Rule: "variants", Code: "sku_invalid", Category: "sku", Message: "z", Status: CheckBlocking, Paths: []string{"b", "a"}}, {Rule: "variants", Code: "sku_invalid", Category: "sku", Message: "a", Status: CheckBlocking}, {Rule: "notes", Code: "manual_notes", Category: "system", Status: CheckWarning}}
	result, err := BuildResult("v1", "offline", Snapshot{Revision: "1"}, false, checks)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Blocked || result.Ready || len(result.Blockers) != 2 || len(result.Warnings) != 1 {
		t.Fatalf("result=%+v", result)
	}
	reversed := []Check{checks[2], checks[1], checks[0]}
	other, err := BuildResult("v1", "offline", Snapshot{Revision: "1"}, false, reversed)
	if err != nil || !reflect.DeepEqual(result, other) {
		t.Fatalf("unstable result: %+v %v", other, err)
	}
	checks[0].Paths[0] = "changed"
	if result.Blockers[1].Paths[0] != "a" {
		t.Fatal("input aliases result")
	}
	result.Blockers[1].Paths[0] = "changed-again"
	for _, check := range result.Checks {
		if check.Message == "z" && check.Paths[0] != "a" {
			t.Fatal("checks alias blockers")
		}
	}
}

func TestResultFailsClosedAndPreservesWarnings(t *testing.T) {
	for _, checks := range [][]Check{nil, {{Rule: "x", Status: "unexpected"}}, {{Status: CheckReady}}} {
		result, err := BuildResult("v1", "offline", Snapshot{}, false, checks)
		if err == nil || result.Ready || result.Status != "" {
			t.Fatalf("invalid evaluation accepted: %+v %v", result, err)
		}
	}
	for _, tc := range []struct {
		check  Check
		status Status
	}{{Check{Rule: "x", Status: CheckReady}, Ready}, {Check{Rule: "x", Status: CheckWarning}, ReadyWithWarnings}} {
		result, err := BuildResult("v1", "offline", Snapshot{}, false, []Check{tc.check})
		if err != nil || !result.Ready || result.Status != tc.status {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
}
