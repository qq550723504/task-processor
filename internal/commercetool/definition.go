package commercetool

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

type RiskLevel string

const (
	RiskRead    RiskLevel = "read"
	RiskCompute RiskLevel = "compute"
	RiskPropose RiskLevel = "propose"
	RiskWrite   RiskLevel = "write"
	RiskPublish RiskLevel = "publish"
)

type SideEffectMode string

const (
	SideEffectNone             SideEffectMode = "none"
	SideEffectBusinessMutation SideEffectMode = "business_mutation"
	SideEffectExternalMutation SideEffectMode = "external_mutation"
)

type IdempotencyMode string

const (
	IdempotencyNotApplicable IdempotencyMode = "not_applicable"
	IdempotencyDeterministic IdempotencyMode = "deterministic"
	IdempotencyRequiredKey   IdempotencyMode = "required_key"
)

type RetryOwner string

const (
	RetryOwnerNone           RetryOwner = "none"
	RetryOwnerCaller         RetryOwner = "caller"
	RetryOwnerAICapability   RetryOwner = "ai_capability"
	RetryOwnerDomainWorkflow RetryOwner = "domain_workflow"
)

type UsageOwner string

const (
	UsageOwnerUnmetered    UsageOwner = "unmetered"
	UsageOwnerAICapability UsageOwner = "ai_capability"
	UsageOwnerDomainLedger UsageOwner = "domain_ledger"
)

type ToolRef struct {
	ID      string
	Version string
}

type AgentRef struct {
	ID      string
	Version string
}

type PermissionRequirement struct{ Permission string }
type SideEffectPolicy struct{ Mode SideEffectMode }
type IdempotencyPolicy struct{ Mode IdempotencyMode }
type TimeoutPolicy struct{ Duration time.Duration }
type RetryPolicy struct{ Owner RetryOwner }
type UsagePolicy struct{ Owner UsageOwner }

type Definition struct {
	Ref          ToolRef
	Capability   string
	Owner        string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Risk         RiskLevel
	Permission   PermissionRequirement
	SideEffects  SideEffectPolicy
	Idempotency  IdempotencyPolicy
	Timeout      TimeoutPolicy
	Retry        RetryPolicy
	Usage        UsagePolicy
}

var qualifiedIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)*$`)

// Validate verifies that the tool identity is unambiguous and canonically versioned.
func (ref ToolRef) Validate() error {
	return validateRef("tool", ref.ID, ref.Version)
}

// Validate verifies that the agent identity is unambiguous and canonically versioned.
func (ref AgentRef) Validate() error {
	return validateRef("agent", ref.ID, ref.Version)
}

// Validate checks Definition's complete static metadata contract. It intentionally
// only requires non-empty schemas; schema compilation belongs to the registry.
func (definition Definition) Validate() error {
	if err := definition.Ref.Validate(); err != nil {
		return err
	}
	if !isQualifiedID(definition.Capability) {
		return fmt.Errorf("invalid capability %q", definition.Capability)
	}
	if !isQualifiedID(definition.Owner) {
		return fmt.Errorf("invalid owner %q", definition.Owner)
	}
	if strings.TrimSpace(definition.Description) == "" {
		return fmt.Errorf("description must not be blank")
	}
	if len(strings.TrimSpace(string(definition.InputSchema))) == 0 {
		return fmt.Errorf("input schema must not be blank")
	}
	if len(strings.TrimSpace(string(definition.OutputSchema))) == 0 {
		return fmt.Errorf("output schema must not be blank")
	}
	if !isRiskLevel(definition.Risk) {
		return fmt.Errorf("invalid risk %q", definition.Risk)
	}
	if !isQualifiedID(definition.Permission.Permission) {
		return fmt.Errorf("invalid permission %q", definition.Permission.Permission)
	}
	if !isSideEffectMode(definition.SideEffects.Mode) {
		return fmt.Errorf("invalid side effect mode %q", definition.SideEffects.Mode)
	}
	if !isIdempotencyMode(definition.Idempotency.Mode) {
		return fmt.Errorf("invalid idempotency mode %q", definition.Idempotency.Mode)
	}
	if definition.Timeout.Duration <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if !isRetryOwner(definition.Retry.Owner) {
		return fmt.Errorf("invalid retry owner %q", definition.Retry.Owner)
	}
	if !isUsageOwner(definition.Usage.Owner) {
		return fmt.Errorf("invalid usage owner %q", definition.Usage.Owner)
	}
	if !isAllowedRiskPolicy(definition.Risk, definition.SideEffects.Mode, definition.Idempotency.Mode) {
		return fmt.Errorf("invalid risk, side effect, and idempotency combination")
	}
	if definition.Retry.Owner == RetryOwnerAICapability &&
		(definition.Risk != RiskPropose || definition.Usage.Owner != UsageOwnerAICapability) {
		return fmt.Errorf("AI capability retry requires propose risk and AI capability usage")
	}
	if definition.Usage.Owner == UsageOwnerAICapability && definition.Risk != RiskPropose {
		return fmt.Errorf("AI capability usage requires propose risk")
	}

	return nil
}

func validateRef(kind, id, version string) error {
	if !isQualifiedID(id) {
		return fmt.Errorf("invalid %s ID %q", kind, id)
	}
	if !semver.IsValid(version) || semver.Canonical(version) != version {
		return fmt.Errorf("invalid %s version %q", kind, version)
	}

	return nil
}

func isQualifiedID(value string) bool {
	return qualifiedIDPattern.MatchString(value)
}

func isRiskLevel(value RiskLevel) bool {
	switch value {
	case RiskRead, RiskCompute, RiskPropose, RiskWrite, RiskPublish:
		return true
	default:
		return false
	}
}

func isSideEffectMode(value SideEffectMode) bool {
	switch value {
	case SideEffectNone, SideEffectBusinessMutation, SideEffectExternalMutation:
		return true
	default:
		return false
	}
}

func isIdempotencyMode(value IdempotencyMode) bool {
	switch value {
	case IdempotencyNotApplicable, IdempotencyDeterministic, IdempotencyRequiredKey:
		return true
	default:
		return false
	}
}

func isRetryOwner(value RetryOwner) bool {
	switch value {
	case RetryOwnerNone, RetryOwnerCaller, RetryOwnerAICapability, RetryOwnerDomainWorkflow:
		return true
	default:
		return false
	}
}

func isUsageOwner(value UsageOwner) bool {
	switch value {
	case UsageOwnerUnmetered, UsageOwnerAICapability, UsageOwnerDomainLedger:
		return true
	default:
	}
	return false
}

func isAllowedRiskPolicy(risk RiskLevel, sideEffects SideEffectMode, idempotency IdempotencyMode) bool {
	switch risk {
	case RiskRead, RiskCompute, RiskPropose:
		return sideEffects == SideEffectNone &&
			(idempotency == IdempotencyNotApplicable || idempotency == IdempotencyDeterministic || idempotency == IdempotencyRequiredKey)
	case RiskWrite:
		return sideEffects == SideEffectBusinessMutation && idempotency == IdempotencyRequiredKey
	case RiskPublish:
		return sideEffects == SideEffectExternalMutation && idempotency == IdempotencyRequiredKey
	default:
		return false
	}
}
