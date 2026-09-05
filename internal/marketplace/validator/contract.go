// Package validator defines the platform-neutral, compute-only validation contract.
package validator

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"
)

type Action string

const (
	Preview   Action = "preview"
	SaveDraft Action = "save_draft"
	Publish   Action = "publish"
)

type Target struct {
	Marketplace string `json:"marketplace"`
	Site        string `json:"site"`
}

// Snapshot pins all facts in the adapter input, including embedded templates.
// Callers bind this revision to the loaded facts; this is not an authorization
// proof or a content hash. All times are supplied, never read from a clock.
type Snapshot struct {
	Revision         string    `json:"revision"`
	ExpectedRevision string    `json:"expected_revision,omitempty"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	ObservedAt       time.Time `json:"observed_at"`
	ValidUntil       time.Time `json:"valid_until"`
}

func (s Snapshot) Validate() error {
	if strings.TrimSpace(s.Revision) == "" || s.EvaluatedAt.IsZero() || s.ObservedAt.IsZero() || s.ValidUntil.IsZero() {
		return &Error{Code: InvalidInput, Field: "snapshot", Message: "revision and explicit freshness timestamps are required"}
	}
	if s.ObservedAt.After(s.EvaluatedAt) || !s.ValidUntil.After(s.ObservedAt) {
		return &Error{Code: InvalidInput, Field: "snapshot", Message: "invalid observation or expiry window"}
	}
	if s.ExpectedRevision != "" && s.ExpectedRevision != s.Revision {
		return &Error{Code: StaleInput, Field: "snapshot.revision", Message: "snapshot revision does not match expected revision"}
	}
	if !s.EvaluatedAt.Before(s.ValidUntil) {
		return &Error{Code: StaleInput, Field: "snapshot.valid_until", Message: "snapshot has expired at the supplied evaluation time"}
	}
	return nil
}

type Request[T any] struct {
	Target      Target   `json:"target"`
	Action      Action   `json:"action"`
	RuleVersion string   `json:"rule_version"`
	Snapshot    Snapshot `json:"snapshot"`
	Input       T        `json:"input"`
}

// Validator implementations must not mutate input, acquire facts or perform I/O.
// Rule failures return a report and nil error; evaluation failures return a zero
// Result and typed error. Reports are scoped diagnostics, not submission permits.
type Validator[T any] interface {
	Validate(Request[T]) (Result, error)
}

type ErrorCode string

const (
	InvalidInput       ErrorCode = "invalid_input"
	UnsupportedTarget  ErrorCode = "unsupported_target"
	UnsupportedAction  ErrorCode = "unsupported_action"
	UnsupportedVersion ErrorCode = "unsupported_rule_version"
	StaleInput         ErrorCode = "stale_input"
	EvaluationFailed   ErrorCode = "evaluation_failed"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Field   string    `json:"field"`
	Message string    `json:"message"`
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Field + ": " + e.Message }

type Status string

const (
	Ready             Status = "ready"
	ReadyWithWarnings Status = "ready_with_warnings"
	Blocked           Status = "blocked"
)

type CheckStatus string

const (
	CheckReady    CheckStatus = "ready"
	CheckWarning  CheckStatus = "warning"
	CheckBlocking CheckStatus = "blocking"
)

type Check struct {
	Rule     string      `json:"rule"`
	Code     string      `json:"code"`
	Category string      `json:"category"`
	Status   CheckStatus `json:"status"`
	Paths    []string    `json:"paths,omitempty"`
	Message  string      `json:"message,omitempty"`
	Guidance string      `json:"guidance,omitempty"`
}

type Result struct {
	Target      Target   `json:"target"`
	Action      Action   `json:"action"`
	RuleVersion string   `json:"rule_version"`
	Scope       string   `json:"scope"`
	Snapshot    Snapshot `json:"snapshot"`
	Status      Status   `json:"status"`
	Ready       bool     `json:"ready"`
	// ReadinessBlockersAllowed describes platform action policy only. It never
	// changes Ready, suppresses findings, or authorizes an external action.
	ReadinessBlockersAllowed bool    `json:"readiness_blockers_allowed"`
	Checks                   []Check `json:"checks"`
	Blockers                 []Check `json:"blockers"`
	Warnings                 []Check `json:"warnings"`
}

// BuildResult normalizes evaluated checks; adapters validate request metadata.
// Distinct findings sharing a rule key remain distinct. No business rules live here.
func BuildResult(version, scope string, snapshot Snapshot, allowsBlockers bool, checks []Check) (Result, error) {
	if version == "" || scope == "" || len(checks) == 0 {
		return Result{}, &Error{Code: EvaluationFailed, Field: "checks", Message: "version, scope and evaluated checks are required"}
	}
	result := Result{RuleVersion: version, Scope: scope, Snapshot: snapshot, ReadinessBlockersAllowed: allowsBlockers, Checks: make([]Check, 0, len(checks)), Blockers: []Check{}, Warnings: []Check{}}
	for _, check := range checks {
		if check.Rule == "" || (check.Status != CheckReady && check.Status != CheckWarning && check.Status != CheckBlocking) {
			return Result{}, &Error{Code: EvaluationFailed, Field: "checks", Message: "rule or check status is invalid"}
		}
		result.Checks = append(result.Checks, cloneCheck(check))
	}
	slices.SortFunc(result.Checks, func(a, b Check) int {
		if order := cmp.Compare(a.Rule, b.Rule); order != 0 {
			return order
		}
		// Check contains only strings/slices; JSON encoding cannot fail.
		left, _ := json.Marshal(a)
		right, _ := json.Marshal(b)
		return strings.Compare(string(left), string(right))
	})
	for _, check := range result.Checks {
		switch check.Status {
		case CheckBlocking:
			result.Blockers = append(result.Blockers, cloneCheck(check))
		case CheckWarning:
			result.Warnings = append(result.Warnings, cloneCheck(check))
		}
	}
	result.Ready = len(result.Blockers) == 0
	switch {
	case !result.Ready:
		result.Status = Blocked
	case len(result.Warnings) > 0:
		result.Status = ReadyWithWarnings
	default:
		result.Status = Ready
	}
	return result, nil
}

func cloneCheck(check Check) Check {
	check.Paths = append([]string(nil), check.Paths...)
	slices.Sort(check.Paths)
	check.Paths = slices.Compact(check.Paths)
	return check
}
