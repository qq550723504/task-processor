package submission

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"task-processor/internal/authidentity"
)

const (
	MaxExecutionPayloadBytes = 2 << 20
	// MaxExecutionEvidenceReasonCharacters matches PostgreSQL VARCHAR(512)
	// semantics. It is a Unicode character limit, not a UTF-8 byte limit.
	MaxExecutionEvidenceReasonCharacters = 512
	MinExecutionLease                    = time.Second
	MaxExecutionLease                    = time.Hour
	providerExecutionDomain              = "task-processor/submission/provider-execution/v1"
)

var (
	ErrExecutionInvalid           = errors.New("invalid submission execution")
	ErrExecutionNotFound          = errors.New("submission execution not found")
	ErrExecutionIntentConflict    = errors.New("submission execution intent conflict")
	ErrExecutionTargetClaimed     = errors.New("submission target already claimed")
	ErrExecutionClaimRejected     = errors.New("submission execution claim rejected")
	ErrExecutionInvalidTransition = errors.New("invalid submission execution transition")
	ErrExecutionEvidenceRequired  = errors.New("submission execution evidence required")
	ErrExecutionUnavailable       = errors.New("submission execution store unavailable")
	ErrExecutionOutcomeUnknown    = errors.New("submission execution commit outcome unknown")
)

type ExecutionStatus string

const (
	ExecutionClaimed          ExecutionStatus = "claimed"
	ExecutionOutcomeUnknown   ExecutionStatus = "outcome_unknown"
	ExecutionSucceeded        ExecutionStatus = "succeeded"
	ExecutionFailedDefinitive ExecutionStatus = "failed_definitive"
	ExecutionCancelled        ExecutionStatus = "cancelled"
)

type UnknownReason string

const (
	UnknownResponseLost       UnknownReason = "response_lost"
	UnknownLeaseExpired       UnknownReason = "lease_expired"
	UnknownExecutionCancelled UnknownReason = "execution_cancelled"
)

type EvidenceKind string

const (
	EvidenceProviderResponse EvidenceKind = "provider_response"
	EvidenceProviderReadBack EvidenceKind = "provider_readback"
	EvidenceManualResolution EvidenceKind = "manual_resolution"
)

type ExecutionScope struct {
	OrganizationID string
}

type ExecutionTarget struct {
	Platform  string
	StoreID   string
	SubjectID string
}

type AcquireExecutionCommand struct {
	Scope        ExecutionScope
	IntentKey    string
	Target       ExecutionTarget
	Action       string
	Payload      []byte
	ClaimOwnerID string
	Lease        time.Duration
}

type ExecutionAttempt struct {
	OrganizationID       string
	AttemptID            string
	IntentKey            string
	Target               ExecutionTarget
	Action               string
	PayloadFingerprint   string
	ProviderExecutionKey string
	ClaimOwnerID         string
	Status               ExecutionStatus
	FenceEpoch           int64
	LeaseExpiresAt       time.Time
	UnknownReason        UnknownReason
	Evidence             *ExecutionEvidence
	CreatedAt            time.Time
	UpdatedAt            time.Time
	FinishedAt           *time.Time
}

type ExecutionEvidence struct {
	Kind         EvidenceKind
	Outcome      ExecutionStatus
	Reference    string
	Fingerprint  string
	Reason       string
	AuthorizedBy string
	ObservedAt   time.Time
}

type ExecutionReservation struct {
	Attempt        ExecutionAttempt
	ClaimTokenHash string
}

type ExecutionClaim struct {
	Scope      ExecutionScope
	AttemptID  string
	FenceEpoch int64
	OwnerID    string
	Token      string
}

type SendPermit struct {
	AttemptID            string
	FenceEpoch           int64
	ClaimToken           string
	ClaimOwnerID         string
	ProviderExecutionKey string
	LeaseExpiresAt       time.Time
}

type ExecutionAcquisition struct {
	Attempt  ExecutionAttempt
	Permit   *SendPermit
	Replayed bool
}

func NewExecutionReservation(command AcquireExecutionCommand, attemptID, claimToken string, now time.Time) (ExecutionReservation, error) {
	organizationID := strings.TrimSpace(command.Scope.OrganizationID)
	intentKey := strings.TrimSpace(command.IntentKey)
	target := ExecutionTarget{
		Platform: strings.ToLower(strings.TrimSpace(command.Target.Platform)),
		StoreID:  strings.TrimSpace(command.Target.StoreID), SubjectID: strings.TrimSpace(command.Target.SubjectID),
	}
	action := strings.ToLower(strings.TrimSpace(command.Action))
	claimOwnerID := strings.TrimSpace(command.ClaimOwnerID)
	if !authidentity.IsBoundedIdentifier(organizationID) || !authidentity.IsBoundedIdentifier(intentKey) ||
		!authidentity.IsBoundedIdentifier(target.Platform) || !authidentity.IsBoundedIdentifier(target.StoreID) ||
		!authidentity.IsBoundedIdentifier(target.SubjectID) || !authidentity.IsBoundedIdentifier(action) ||
		!authidentity.IsBoundedIdentifier(claimOwnerID) || !validExecutionAttemptID(attemptID) || claimToken == "" || len(claimToken) > 512 ||
		len(command.Payload) == 0 || len(command.Payload) > MaxExecutionPayloadBytes ||
		command.Lease < MinExecutionLease || command.Lease > MaxExecutionLease || now.IsZero() {
		return ExecutionReservation{}, ErrExecutionInvalid
	}
	now = canonicalExecutionTime(now)
	payloadFingerprint := digestExecutionParts(target.Platform, target.StoreID, target.SubjectID, action, string(command.Payload))
	providerKey := "subk1_v1_" + digestExecutionParts(providerExecutionDomain, organizationID, intentKey)
	return ExecutionReservation{
		Attempt: ExecutionAttempt{
			OrganizationID: organizationID, AttemptID: attemptID, IntentKey: intentKey,
			Target: target, Action: action, PayloadFingerprint: payloadFingerprint,
			ProviderExecutionKey: providerKey, ClaimOwnerID: claimOwnerID, Status: ExecutionClaimed,
			LeaseExpiresAt: canonicalExecutionTime(now.Add(command.Lease)), CreatedAt: now, UpdatedAt: now,
		},
		ClaimTokenHash: ExecutionClaimTokenHash(claimToken),
	}, nil
}

func ValidateExecutionReplay(existing, candidate ExecutionAttempt) error {
	if validateExecutionAttempt(existing) != nil || validateExecutionAttempt(candidate) != nil ||
		existing.OrganizationID != candidate.OrganizationID || existing.IntentKey != candidate.IntentKey ||
		existing.Target != candidate.Target || existing.Action != candidate.Action ||
		existing.PayloadFingerprint != candidate.PayloadFingerprint ||
		existing.ProviderExecutionKey != candidate.ProviderExecutionKey {
		return ErrExecutionIntentConflict
	}
	return nil
}

func TransitionExecutionToUnknown(attempt ExecutionAttempt, reason UnknownReason, now time.Time) (ExecutionAttempt, error) {
	if validateExecutionAttempt(attempt) != nil || !validUnknownReason(reason) || now.IsZero() {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	if attempt.Status == ExecutionOutcomeUnknown {
		if attempt.UnknownReason == reason {
			return attempt, nil
		}
		return ExecutionAttempt{}, ErrExecutionIntentConflict
	}
	if attempt.Status != ExecutionClaimed {
		return ExecutionAttempt{}, ErrExecutionInvalidTransition
	}
	now = canonicalExecutionTime(now)
	if now.Before(attempt.CreatedAt) {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	attempt.Status = ExecutionOutcomeUnknown
	attempt.UnknownReason = reason
	attempt.UpdatedAt = now
	attempt.FinishedAt = nil
	attempt.Evidence = nil
	return attempt, nil
}

func CompleteClaimedExecution(attempt ExecutionAttempt, evidence ExecutionEvidence, now time.Time) (ExecutionAttempt, error) {
	if validateExecutionAttempt(attempt) != nil || now.IsZero() {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	now = canonicalExecutionTime(now)
	evidence.ObservedAt = canonicalExecutionTime(evidence.ObservedAt)
	if (evidence.Kind == EvidenceProviderResponse || evidence.Kind == EvidenceProviderReadBack) && evidence.AuthorizedBy != "" {
		return ExecutionAttempt{}, ErrExecutionEvidenceRequired
	}
	if isExecutionTerminal(attempt.Status) {
		if sameExecutionEvidence(attempt.Evidence, &evidence) && attempt.Status == evidence.Outcome {
			return attempt, nil
		}
		return ExecutionAttempt{}, ErrExecutionIntentConflict
	}
	if attempt.Status != ExecutionClaimed {
		return ExecutionAttempt{}, ErrExecutionInvalidTransition
	}
	if !now.Before(attempt.LeaseExpiresAt) {
		return ExecutionAttempt{}, ErrExecutionClaimRejected
	}
	if evidence.Kind != EvidenceProviderResponse || evidence.Outcome != ExecutionSucceeded && evidence.Outcome != ExecutionFailedDefinitive {
		return ExecutionAttempt{}, ErrExecutionEvidenceRequired
	}
	if err := validateExecutionEvidence(attempt, evidence, now); err != nil {
		return ExecutionAttempt{}, err
	}
	return finishExecution(attempt, evidence, now), nil
}

func ResolveUnknownExecution(attempt ExecutionAttempt, evidence ExecutionEvidence, now time.Time) (ExecutionAttempt, error) {
	if validateExecutionAttempt(attempt) != nil || now.IsZero() {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	now = canonicalExecutionTime(now)
	evidence.ObservedAt = canonicalExecutionTime(evidence.ObservedAt)
	if (evidence.Kind == EvidenceProviderResponse || evidence.Kind == EvidenceProviderReadBack) && evidence.AuthorizedBy != "" {
		return ExecutionAttempt{}, ErrExecutionEvidenceRequired
	}
	if isExecutionTerminal(attempt.Status) {
		if sameExecutionEvidence(attempt.Evidence, &evidence) && attempt.Status == evidence.Outcome {
			return attempt, nil
		}
		return ExecutionAttempt{}, ErrExecutionIntentConflict
	}
	if attempt.Status != ExecutionOutcomeUnknown {
		return ExecutionAttempt{}, ErrExecutionInvalidTransition
	}
	if err := validateExecutionEvidence(attempt, evidence, now); err != nil {
		return ExecutionAttempt{}, err
	}
	switch evidence.Kind {
	case EvidenceProviderReadBack:
		if evidence.Outcome != ExecutionSucceeded && evidence.Outcome != ExecutionFailedDefinitive {
			return ExecutionAttempt{}, ErrExecutionEvidenceRequired
		}
	case EvidenceManualResolution:
		if evidence.Outcome != ExecutionSucceeded && evidence.Outcome != ExecutionFailedDefinitive && evidence.Outcome != ExecutionCancelled ||
			!authidentity.IsBoundedIdentifier(evidence.AuthorizedBy) || strings.TrimSpace(evidence.Reason) == "" {
			return ExecutionAttempt{}, ErrExecutionEvidenceRequired
		}
	default:
		return ExecutionAttempt{}, ErrExecutionEvidenceRequired
	}
	return finishExecution(attempt, evidence, now), nil
}

func validateExecutionAttempt(attempt ExecutionAttempt) error {
	if !authidentity.IsBoundedIdentifier(attempt.OrganizationID) || !validExecutionAttemptID(attempt.AttemptID) ||
		!authidentity.IsBoundedIdentifier(attempt.IntentKey) || !authidentity.IsBoundedIdentifier(attempt.Target.Platform) ||
		!authidentity.IsBoundedIdentifier(attempt.Target.StoreID) || !authidentity.IsBoundedIdentifier(attempt.Target.SubjectID) ||
		!authidentity.IsBoundedIdentifier(attempt.Action) || !isExecutionDigest(attempt.PayloadFingerprint) ||
		!authidentity.IsBoundedIdentifier(attempt.ClaimOwnerID) ||
		!strings.HasPrefix(attempt.ProviderExecutionKey, "subk1_v1_") || !isExecutionDigest(strings.TrimPrefix(attempt.ProviderExecutionKey, "subk1_v1_")) ||
		attempt.Status == "" || attempt.LeaseExpiresAt.IsZero() || attempt.CreatedAt.IsZero() || attempt.UpdatedAt.IsZero() {
		return ErrExecutionInvalid
	}
	return nil
}

func ValidatePersistedExecutionAttempt(attempt ExecutionAttempt) error {
	if validateExecutionAttempt(attempt) != nil || attempt.FenceEpoch <= 0 ||
		attempt.ProviderExecutionKey != "subk1_v1_"+digestExecutionParts(providerExecutionDomain, attempt.OrganizationID, attempt.IntentKey) ||
		attempt.Target.Platform != strings.ToLower(attempt.Target.Platform) || attempt.Action != strings.ToLower(attempt.Action) ||
		!attempt.LeaseExpiresAt.After(attempt.CreatedAt) || attempt.UpdatedAt.Before(attempt.CreatedAt) {
		return ErrExecutionUnavailable
	}
	switch attempt.Status {
	case ExecutionClaimed:
		if attempt.UnknownReason != "" || attempt.Evidence != nil || attempt.FinishedAt != nil {
			return ErrExecutionUnavailable
		}
	case ExecutionOutcomeUnknown:
		if !validUnknownReason(attempt.UnknownReason) || attempt.Evidence != nil || attempt.FinishedAt != nil {
			return ErrExecutionUnavailable
		}
	case ExecutionSucceeded, ExecutionFailedDefinitive, ExecutionCancelled:
		if attempt.UnknownReason != "" || attempt.Evidence == nil || attempt.FinishedAt == nil ||
			attempt.Evidence.Outcome != attempt.Status || attempt.FinishedAt.Before(attempt.CreatedAt) ||
			validatePersistedExecutionEvidence(*attempt.Evidence, attempt.CreatedAt) != nil {
			return ErrExecutionUnavailable
		}
	default:
		return ErrExecutionUnavailable
	}
	return nil
}

func validatePersistedExecutionEvidence(evidence ExecutionEvidence, createdAt time.Time) error {
	if !authidentity.IsBoundedIdentifier(evidence.Reference) || !isExecutionDigest(evidence.Fingerprint) ||
		evidence.ObservedAt.IsZero() || evidence.ObservedAt.Before(createdAt) || !validExecutionEvidenceReason(evidence.Reason) {
		return ErrExecutionUnavailable
	}
	switch evidence.Kind {
	case EvidenceProviderResponse, EvidenceProviderReadBack:
		if evidence.AuthorizedBy != "" || evidence.Outcome != ExecutionSucceeded && evidence.Outcome != ExecutionFailedDefinitive {
			return ErrExecutionUnavailable
		}
		missingReason := evidence.Outcome == ExecutionFailedDefinitive && strings.TrimSpace(evidence.Reason) == ""
		if missingReason {
			return ErrExecutionUnavailable
		}
	case EvidenceManualResolution:
		if !authidentity.IsBoundedIdentifier(evidence.AuthorizedBy) || strings.TrimSpace(evidence.Reason) == "" ||
			evidence.Outcome != ExecutionSucceeded && evidence.Outcome != ExecutionFailedDefinitive && evidence.Outcome != ExecutionCancelled {
			return ErrExecutionUnavailable
		}
	default:
		return ErrExecutionUnavailable
	}
	return nil
}

func validateExecutionEvidence(attempt ExecutionAttempt, evidence ExecutionEvidence, now time.Time) error {
	if !authidentity.IsBoundedIdentifier(evidence.Reference) || !isExecutionDigest(evidence.Fingerprint) ||
		evidence.ObservedAt.IsZero() || evidence.ObservedAt.Before(attempt.CreatedAt) || evidence.ObservedAt.After(now) ||
		!validExecutionEvidenceReason(evidence.Reason) {
		return ErrExecutionEvidenceRequired
	}
	if evidence.Outcome == ExecutionFailedDefinitive && strings.TrimSpace(evidence.Reason) == "" {
		return ErrExecutionEvidenceRequired
	}
	return nil
}

func validExecutionEvidenceReason(reason string) bool {
	return utf8.ValidString(reason) && !strings.ContainsRune(reason, '\x00') &&
		utf8.RuneCountInString(reason) <= MaxExecutionEvidenceReasonCharacters
}

func finishExecution(attempt ExecutionAttempt, evidence ExecutionEvidence, now time.Time) ExecutionAttempt {
	now = canonicalExecutionTime(now)
	evidence.ObservedAt = canonicalExecutionTime(evidence.ObservedAt)
	attempt.Status = evidence.Outcome
	attempt.UnknownReason = ""
	attempt.Evidence = &evidence
	attempt.UpdatedAt = now
	attempt.FinishedAt = &now
	return attempt
}

func sameExecutionEvidence(left, right *ExecutionEvidence) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func isExecutionTerminal(status ExecutionStatus) bool {
	return status == ExecutionSucceeded || status == ExecutionFailedDefinitive || status == ExecutionCancelled
}

func validUnknownReason(reason UnknownReason) bool {
	return reason == UnknownResponseLost || reason == UnknownLeaseExpired || reason == UnknownExecutionCancelled
}

func validExecutionAttemptID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value && id.Variant() == uuid.RFC4122 && id.Version() == 7
}

func isExecutionDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func canonicalExecutionTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func digestExecutionValue(value string) string {
	return digestExecutionParts(value)
}

// ExecutionClaimTokenHash is shared with the PostgreSQL adapter so the raw
// one-time send capability never needs to be persisted.
func ExecutionClaimTokenHash(token string) string {
	return digestExecutionParts(token)
}

func digestExecutionParts(parts ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}
