package validator

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type FreshnessStatus string

const (
	NotEvaluated             FreshnessStatus = "not_evaluated"
	FreshnessValid           FreshnessStatus = "valid"
	FreshnessStale           FreshnessStatus = "stale"
	FreshnessExpired         FreshnessStatus = "expired"
	ExternalPackageFreshness                 = "external_package_freshness"
)

// Evidence is a trusted caller assertion, not something this pure contract
// acquires or authenticates. PolicyVersion/Source are opaque owner identifiers.
type FreshnessEvidence struct {
	SubjectDigest string    `json:"subject_digest"`
	PolicyVersion string    `json:"policy_version"`
	Source        string    `json:"source"`
	ObservedAt    time.Time `json:"observed_at"`
	ValidUntil    time.Time `json:"valid_until"`
}
type ExternalFreshness struct {
	Status   FreshnessStatus    `json:"status"`
	Coverage []string           `json:"coverage"`
	Evidence *FreshnessEvidence `json:"evidence,omitempty"`
}

type BoundRequest[T any] struct {
	Input          T
	Target         Target
	Action         Action
	RuleVersion    string
	BindingVersion string
	ExpectedDigest string
	ReadAt         time.Time
	EvaluatedAt    time.Time
	Freshness      ExternalFreshness
}
type BoundInput struct {
	Digest         string    `json:"actual_digest"`
	BindingVersion string    `json:"binding_version"`
	ReadAt         time.Time `json:"read_at"`
	EvaluatedAt    time.Time `json:"evaluated_at"`
}
type OfflineChecks struct {
	Status   Status  `json:"status"`
	Checks   []Check `json:"checks"`
	Blockers []Check `json:"blockers"`
	Warnings []Check `json:"warnings"`
}
type DiagnosticActionPolicy struct {
	ReadinessBlockersAllowed bool `json:"readiness_blockers_allowed"`
}
type DiagnosticResult struct {
	DiagnosticOnly bool                   `json:"diagnostic_only"`
	Scope          string                 `json:"scope"`
	Target         Target                 `json:"target"`
	Action         Action                 `json:"action"`
	RuleVersion    string                 `json:"rule_version"`
	Input          BoundInput             `json:"input"`
	Freshness      ExternalFreshness      `json:"external_freshness"`
	NotEvaluated   []string               `json:"not_evaluated"`
	OfflineChecks  OfflineChecks          `json:"offline_checks"`
	ActionPolicy   DiagnosticActionPolicy `json:"action_policy"`
}

func ValidateBoundInput(input BoundInput, expected string) error {
	if !ValidContentDigest(input.Digest) || input.BindingVersion == "" || input.ReadAt.IsZero() || input.EvaluatedAt.IsZero() || input.EvaluatedAt.Before(input.ReadAt) {
		return &Error{Code: InvalidInput, Field: "input", Message: "content identity and ordered explicit read/evaluation times are required"}
	}
	if expected != "" && !ValidContentDigest(expected) {
		return &Error{Code: InvalidInput, Field: "expected_digest", Message: "invalid content digest"}
	}
	if expected != "" && expected != input.Digest {
		return &Error{Code: StaleInput, Field: "expected_digest", Message: "loaded content does not match expected digest"}
	}
	return nil
}

func ValidContentDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Validate preserves known adverse evidence; no missing metadata becomes fresh.
// The only freshness scope in this version is the external package. Other
// authorization/asset/submission checks remain outside this evidence contract.
func (f ExternalFreshness) Validate(digest string, evaluated time.Time) error {
	invalid := func() error {
		return &Error{Code: InvalidInput, Field: "external_freshness", Message: "explicit status and valid scoped evidence are required"}
	}
	if f.Status == NotEvaluated {
		if f.Evidence != nil || len(f.Coverage) != 0 {
			return invalid()
		}
		return nil
	}
	if f.Status != FreshnessValid && f.Status != FreshnessStale && f.Status != FreshnessExpired {
		return invalid()
	}
	e := f.Evidence
	if e == nil || len(f.Coverage) != 1 || !boundedIdentity(f.Coverage[0]) || !ValidContentDigest(e.SubjectDigest) || !boundedIdentity(e.PolicyVersion) || !boundedIdentity(e.Source) || e.ObservedAt.IsZero() || e.ValidUntil.IsZero() || evaluated.IsZero() || e.ObservedAt.After(evaluated) || !e.ValidUntil.After(e.ObservedAt) {
		return invalid()
	}
	if e.SubjectDigest != digest || f.Status == FreshnessStale || f.Status == FreshnessExpired || !evaluated.Before(e.ValidUntil) {
		return &Error{Code: StaleInput, Field: "external_freshness", Message: "freshness evidence is stale, expired or bound to different content"}
	}
	if f.Coverage[0] != ExternalPackageFreshness {
		return invalid()
	}
	return nil
}

// RequireFreshness is for later consumers that explicitly need this coverage;
// unknown or partial evidence must never satisfy such a requirement.
func (f ExternalFreshness) RequireFreshness(digest string, evaluated time.Time) error {
	if err := f.Validate(digest, evaluated); err != nil {
		return err
	}
	if f.Status != FreshnessValid || !slices.Contains(f.Coverage, ExternalPackageFreshness) {
		return &Error{Code: InvalidInput, Field: "external_freshness", Message: "required external package freshness was not evaluated"}
	}
	return nil
}

func boundedIdentity(s string) bool {
	return s != "" && len(s) <= 256 && strings.TrimSpace(s) == s && utf8.ValidString(s) && !strings.ContainsAny(s, "\x00\r\n\t")
}

func (f ExternalFreshness) Clone() ExternalFreshness {
	f.Coverage = append([]string{}, f.Coverage...)
	if f.Evidence != nil {
		e := *f.Evidence
		f.Evidence = &e
	}
	return f
}
