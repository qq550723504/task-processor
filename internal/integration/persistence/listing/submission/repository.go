package submissionpersistence

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listing/submission"
)

const (
	attemptTable = "public." + AttemptTable
	targetTable  = "public." + TargetFenceTable
)

type Repository struct {
	db    *gorm.DB
	fault func(string) error
}

func NewRepository(db *gorm.DB) (*Repository, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, submission.ErrExecutionUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), submission.ExecutionTimeout)
	defer cancel()
	if err := VerifySchema(ctx, db); err != nil {
		return nil, fmt.Errorf("%w: %v", submission.ErrExecutionUnavailable, err)
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Acquire(ctx context.Context, reservation submission.ExecutionReservation) (submission.ExecutionAttempt, bool, error) {
	if r == nil || r.db == nil {
		return submission.ExecutionAttempt{}, false, submission.ErrExecutionUnavailable
	}
	if existing, found, err := r.findByIntent(ctx, r.db, reservation.Attempt.OrganizationID, reservation.Attempt.IntentKey, false); err != nil {
		return submission.ExecutionAttempt{}, false, err
	} else if found {
		if err := submission.ValidateExecutionReplay(existing, reservation.Attempt); err != nil {
			return submission.ExecutionAttempt{}, false, err
		}
		if existing.Status != submission.ExecutionClaimed && existing.Status != submission.ExecutionOutcomeUnknown {
			return existing, false, nil
		}
		if matches, err := r.activeAttemptMatchesTarget(ctx, r.db, existing); err != nil {
			return submission.ExecutionAttempt{}, false, err
		} else if matches {
			return existing, false, nil
		}
	}

	var acquired bool
	result, err := r.write(ctx, func(tx *gorm.DB) (mutation, error) {
		if err := advisoryLock(tx, "intent", reservation.Attempt.OrganizationID, reservation.Attempt.IntentKey); err != nil {
			return mutation{}, err
		}
		if existing, found, err := r.findByIntent(ctx, tx, reservation.Attempt.OrganizationID, reservation.Attempt.IntentKey, true); err != nil {
			return mutation{}, err
		} else if found {
			if err := submission.ValidateExecutionReplay(existing, reservation.Attempt); err != nil {
				return mutation{}, err
			}
			if existing.Status == submission.ExecutionClaimed || existing.Status == submission.ExecutionOutcomeUnknown {
				matches, err := r.activeAttemptMatchesTarget(ctx, tx, existing)
				if err != nil {
					return mutation{}, err
				}
				if !matches {
					return mutation{}, submission.ErrExecutionUnavailable
				}
			}
			return mutation{attempt: existing}, nil
		}
		attempt := reservation.Attempt
		if err := advisoryLock(tx, "target", attempt.OrganizationID, attempt.Target.Platform, attempt.Target.StoreID, attempt.Target.SubjectID); err != nil {
			return mutation{}, err
		}
		fence, found, err := r.findTarget(ctx, tx, attempt.OrganizationID, attempt.Target, true)
		if err != nil {
			return mutation{}, err
		}
		if found {
			current, currentFound, currentErr := r.findByID(ctx, tx, attempt.OrganizationID, fence.CurrentAttemptID, false)
			if currentErr != nil {
				return mutation{}, currentErr
			}
			if !currentFound || current.Target != attempt.Target || current.FenceEpoch != fence.Epoch || string(current.Status) != fence.CurrentStatus {
				return mutation{}, submission.ErrExecutionUnavailable
			}
		}
		if found && (fence.CurrentStatus == string(submission.ExecutionClaimed) || fence.CurrentStatus == string(submission.ExecutionOutcomeUnknown)) {
			return mutation{}, submission.ErrExecutionTargetClaimed
		}
		if found {
			if fence.Epoch <= 0 || fence.CurrentAttemptID == "" {
				return mutation{}, submission.ErrExecutionUnavailable
			}
			attempt.FenceEpoch = fence.Epoch + 1
		} else {
			attempt.FenceEpoch = 1
		}
		row := attemptRowFrom(attempt, reservation.ClaimTokenHash)
		if err := tx.WithContext(ctx).Table(attemptTable).Create(&row).Error; err != nil {
			return mutation{}, err
		}
		fence = targetFenceRow{
			OrganizationID: attempt.OrganizationID, Platform: attempt.Target.Platform, StoreID: attempt.Target.StoreID,
			SubjectID: attempt.Target.SubjectID, Epoch: attempt.FenceEpoch, CurrentAttemptID: attempt.AttemptID,
			CurrentStatus: string(attempt.Status), UpdatedAt: attempt.UpdatedAt,
		}
		if found {
			updated := tx.WithContext(ctx).Table(targetTable).
				Where("organization_id = ? AND platform = ? AND store_id = ? AND subject_id = ? AND epoch = ?", attempt.OrganizationID, attempt.Target.Platform, attempt.Target.StoreID, attempt.Target.SubjectID, attempt.FenceEpoch-1).
				Updates(map[string]any{"epoch": fence.Epoch, "current_attempt_id": fence.CurrentAttemptID, "current_status": fence.CurrentStatus, "updated_at": fence.UpdatedAt})
			if updated.Error != nil {
				return mutation{}, updated.Error
			}
			if updated.RowsAffected != 1 {
				return mutation{}, submission.ErrExecutionUnavailable
			}
		} else if err := tx.WithContext(ctx).Table(targetTable).Create(&fence).Error; err != nil {
			return mutation{}, err
		}
		acquired = true
		return mutation{attempt: attempt}, nil
	})
	return result, acquired, err
}

func (r *Repository) activeAttemptMatchesTarget(ctx context.Context, db *gorm.DB, attempt submission.ExecutionAttempt) (bool, error) {
	fence, found, err := r.findTarget(ctx, db, attempt.OrganizationID, attempt.Target, false)
	if err != nil || !found {
		return false, err
	}
	return fence.CurrentAttemptID == attempt.AttemptID && fence.Epoch == attempt.FenceEpoch && fence.CurrentStatus == string(attempt.Status), nil
}

func (r *Repository) Get(ctx context.Context, scope submission.ExecutionScope, attemptID string) (submission.ExecutionAttempt, error) {
	if r == nil || r.db == nil {
		return submission.ExecutionAttempt{}, submission.ErrExecutionUnavailable
	}
	attempt, found, err := r.findByID(ctx, r.db, scope.OrganizationID, attemptID, false)
	if err != nil {
		return submission.ExecutionAttempt{}, err
	}
	if !found {
		return submission.ExecutionAttempt{}, submission.ErrExecutionNotFound
	}
	return attempt, nil
}

func (r *Repository) Renew(ctx context.Context, claim submission.ExecutionClaim, leaseExpiresAt, now time.Time) (submission.ExecutionAttempt, error) {
	return r.claimMutation(ctx, claim, now, func(attempt submission.ExecutionAttempt) (submission.ExecutionAttempt, error, error) {
		if attempt.Status != submission.ExecutionClaimed {
			return attempt, nil, submission.ErrExecutionClaimRejected
		}
		if !now.Before(attempt.LeaseExpiresAt) {
			unknown, err := submission.TransitionExecutionToUnknown(attempt, submission.UnknownLeaseExpired, now)
			return unknown, err, submission.ErrExecutionClaimRejected
		}
		if !leaseExpiresAt.After(attempt.LeaseExpiresAt) {
			return attempt, submission.ErrExecutionInvalid, nil
		}
		attempt.LeaseExpiresAt = leaseExpiresAt
		attempt.UpdatedAt = now
		return attempt, nil, nil
	})
}

func (r *Repository) MarkUnknown(ctx context.Context, claim submission.ExecutionClaim, reason submission.UnknownReason, now time.Time) (submission.ExecutionAttempt, error) {
	return r.claimMutation(ctx, claim, now, func(attempt submission.ExecutionAttempt) (submission.ExecutionAttempt, error, error) {
		updated, err := submission.TransitionExecutionToUnknown(attempt, reason, now)
		return updated, err, nil
	})
}

func (r *Repository) Complete(ctx context.Context, claim submission.ExecutionClaim, evidence submission.ExecutionEvidence, now time.Time) (submission.ExecutionAttempt, error) {
	return r.write(ctx, func(tx *gorm.DB) (mutation, error) {
		attempt, tokenHash, err := r.lockAttempt(ctx, tx, claim.Scope.OrganizationID, claim.AttemptID)
		if err != nil {
			return mutation{}, err
		}
		if attempt.FenceEpoch != claim.FenceEpoch || attempt.ClaimOwnerID != claim.OwnerID || subtle.ConstantTimeCompare([]byte(tokenHash), []byte(submission.ExecutionClaimTokenHash(claim.Token))) != 1 {
			return mutation{}, submission.ErrExecutionClaimRejected
		}
		if attempt.Status == submission.ExecutionSucceeded || attempt.Status == submission.ExecutionFailedDefinitive || attempt.Status == submission.ExecutionCancelled {
			replayed, err := submission.CompleteClaimedExecution(attempt, evidence, now)
			return mutation{attempt: replayed}, err
		}
		if attempt.Status == submission.ExecutionOutcomeUnknown {
			return mutation{attempt: attempt, afterCommitErr: submission.ErrExecutionClaimRejected}, nil
		}
		if _, err := r.lockCurrentFence(ctx, tx, attempt); err != nil {
			return mutation{}, err
		}
		if attempt.Status == submission.ExecutionClaimed && !now.Before(attempt.LeaseExpiresAt) {
			unknown, err := submission.TransitionExecutionToUnknown(attempt, submission.UnknownLeaseExpired, now)
			if err != nil {
				return mutation{}, err
			}
			if err := r.persistAttemptAndFence(ctx, tx, unknown); err != nil {
				return mutation{}, err
			}
			return mutation{attempt: unknown, afterCommitErr: submission.ErrExecutionClaimRejected}, nil
		}
		updated, err := submission.CompleteClaimedExecution(attempt, evidence, now)
		if err != nil {
			return mutation{}, err
		}
		if err := r.persistAttemptAndFence(ctx, tx, updated); err != nil {
			return mutation{}, err
		}
		return mutation{attempt: updated}, nil
	})
}

func (r *Repository) ResolveUnknown(ctx context.Context, scope submission.ExecutionScope, attemptID string, fenceEpoch int64, evidence submission.ExecutionEvidence, now time.Time) (submission.ExecutionAttempt, error) {
	return r.write(ctx, func(tx *gorm.DB) (mutation, error) {
		attempt, _, err := r.lockAttempt(ctx, tx, scope.OrganizationID, attemptID)
		if err != nil {
			return mutation{}, err
		}
		if attempt.FenceEpoch != fenceEpoch {
			return mutation{}, submission.ErrExecutionClaimRejected
		}
		if attempt.Status == submission.ExecutionSucceeded || attempt.Status == submission.ExecutionFailedDefinitive || attempt.Status == submission.ExecutionCancelled {
			replayed, err := submission.ResolveUnknownExecution(attempt, evidence, now)
			return mutation{attempt: replayed}, err
		}
		if _, err := r.lockCurrentFence(ctx, tx, attempt); err != nil {
			return mutation{}, err
		}
		updated, err := submission.ResolveUnknownExecution(attempt, evidence, now)
		if err != nil {
			return mutation{}, err
		}
		if updated == attempt {
			return mutation{attempt: attempt}, nil
		}
		if err := r.persistAttemptAndFence(ctx, tx, updated); err != nil {
			return mutation{}, err
		}
		return mutation{attempt: updated}, nil
	})
}

func (r *Repository) Expire(ctx context.Context, scope submission.ExecutionScope, attemptID string, now time.Time) (submission.ExecutionAttempt, error) {
	return r.write(ctx, func(tx *gorm.DB) (mutation, error) {
		attempt, _, _, err := r.lockCurrentAttempt(ctx, tx, scope.OrganizationID, attemptID)
		if err != nil {
			return mutation{}, err
		}
		if attempt.Status == submission.ExecutionOutcomeUnknown {
			return mutation{attempt: attempt}, nil
		}
		if attempt.Status != submission.ExecutionClaimed || now.Before(attempt.LeaseExpiresAt) {
			return mutation{}, submission.ErrExecutionInvalidTransition
		}
		updated, err := submission.TransitionExecutionToUnknown(attempt, submission.UnknownLeaseExpired, now)
		if err != nil {
			return mutation{}, err
		}
		if err := r.persistAttemptAndFence(ctx, tx, updated); err != nil {
			return mutation{}, err
		}
		return mutation{attempt: updated}, nil
	})
}

func (r *Repository) claimMutation(ctx context.Context, claim submission.ExecutionClaim, now time.Time, transition func(submission.ExecutionAttempt) (submission.ExecutionAttempt, error, error)) (submission.ExecutionAttempt, error) {
	return r.write(ctx, func(tx *gorm.DB) (mutation, error) {
		attempt, _, tokenHash, err := r.lockCurrentAttempt(ctx, tx, claim.Scope.OrganizationID, claim.AttemptID)
		if err != nil {
			return mutation{}, err
		}
		if attempt.FenceEpoch != claim.FenceEpoch || attempt.ClaimOwnerID != claim.OwnerID || subtle.ConstantTimeCompare([]byte(tokenHash), []byte(submission.ExecutionClaimTokenHash(claim.Token))) != 1 {
			return mutation{}, submission.ErrExecutionClaimRejected
		}
		updated, transitionErr, afterCommitErr := transition(attempt)
		if transitionErr != nil {
			return mutation{}, transitionErr
		}
		if updated != attempt {
			if err := r.persistAttemptAndFence(ctx, tx, updated); err != nil {
				return mutation{}, err
			}
		}
		return mutation{attempt: updated, afterCommitErr: afterCommitErr}, nil
	})
}

type mutation struct {
	attempt        submission.ExecutionAttempt
	afterCommitErr error
}

func (r *Repository) write(ctx context.Context, fn func(*gorm.DB) (mutation, error)) (submission.ExecutionAttempt, error) {
	if r == nil || r.db == nil || fn == nil {
		return submission.ExecutionAttempt{}, submission.ErrExecutionUnavailable
	}
	tx := r.db.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if tx.Error != nil {
		return submission.ExecutionAttempt{}, mapError(ctx, tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()
	if r.fault != nil {
		if err := r.fault("synchronous_commit"); err != nil {
			return submission.ExecutionAttempt{}, mapError(ctx, err)
		}
	}
	if err := tx.Exec("SET LOCAL synchronous_commit = on").Error; err != nil {
		return submission.ExecutionAttempt{}, mapError(ctx, err)
	}
	result, err := fn(tx)
	if err != nil {
		return submission.ExecutionAttempt{}, mapError(ctx, err)
	}
	if r.fault != nil {
		if err := r.fault("before_commit"); err != nil {
			return submission.ExecutionAttempt{}, mapError(ctx, err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return submission.ExecutionAttempt{}, fmt.Errorf("%w: %v", submission.ErrExecutionOutcomeUnknown, err)
	}
	committed = true
	if r.fault != nil {
		if err := r.fault("after_commit"); err != nil {
			return submission.ExecutionAttempt{}, fmt.Errorf("%w: %v", submission.ErrExecutionOutcomeUnknown, err)
		}
	}
	return result.attempt, result.afterCommitErr
}

func (r *Repository) lockCurrentAttempt(ctx context.Context, tx *gorm.DB, organizationID, attemptID string) (submission.ExecutionAttempt, targetFenceRow, string, error) {
	attempt, tokenHash, err := r.lockAttempt(ctx, tx, organizationID, attemptID)
	if err != nil {
		return submission.ExecutionAttempt{}, targetFenceRow{}, "", err
	}
	fence, err := r.lockCurrentFence(ctx, tx, attempt)
	if err != nil {
		return submission.ExecutionAttempt{}, targetFenceRow{}, "", err
	}
	return attempt, fence, tokenHash, nil
}

func (r *Repository) lockAttempt(ctx context.Context, tx *gorm.DB, organizationID, attemptID string) (submission.ExecutionAttempt, string, error) {
	var row executionAttemptRow
	err := tx.WithContext(ctx).Table(attemptTable).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("organization_id = ? AND attempt_id = ?", organizationID, attemptID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return submission.ExecutionAttempt{}, "", submission.ErrExecutionNotFound
	}
	if err != nil {
		return submission.ExecutionAttempt{}, "", err
	}
	attempt, err := row.attempt()
	if err != nil {
		return submission.ExecutionAttempt{}, "", err
	}
	return attempt, row.ClaimTokenHash, nil
}

func (r *Repository) lockCurrentFence(ctx context.Context, tx *gorm.DB, attempt submission.ExecutionAttempt) (targetFenceRow, error) {
	fence, found, err := r.findTarget(ctx, tx, attempt.OrganizationID, attempt.Target, true)
	if err != nil {
		return targetFenceRow{}, err
	}
	if !found || fence.CurrentAttemptID != attempt.AttemptID || fence.Epoch != attempt.FenceEpoch || fence.CurrentStatus != string(attempt.Status) {
		return targetFenceRow{}, submission.ErrExecutionClaimRejected
	}
	return fence, nil
}

func (r *Repository) persistAttemptAndFence(ctx context.Context, tx *gorm.DB, attempt submission.ExecutionAttempt) error {
	row := attemptRowFrom(attempt, "")
	updated := tx.WithContext(ctx).Table(attemptTable).
		Where("organization_id = ? AND attempt_id = ? AND fence_epoch = ?", attempt.OrganizationID, attempt.AttemptID, attempt.FenceEpoch).
		Updates(row.mutableValues())
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return submission.ErrExecutionClaimRejected
	}
	updated = tx.WithContext(ctx).Table(targetTable).
		Where("organization_id = ? AND platform = ? AND store_id = ? AND subject_id = ? AND epoch = ? AND current_attempt_id = ?", attempt.OrganizationID, attempt.Target.Platform, attempt.Target.StoreID, attempt.Target.SubjectID, attempt.FenceEpoch, attempt.AttemptID).
		Updates(map[string]any{"current_status": string(attempt.Status), "updated_at": attempt.UpdatedAt})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return submission.ErrExecutionClaimRejected
	}
	return nil
}

func (r *Repository) findByIntent(ctx context.Context, db *gorm.DB, organizationID, intentKey string, lock bool) (submission.ExecutionAttempt, bool, error) {
	query := db.WithContext(ctx).Table(attemptTable)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row executionAttemptRow
	err := query.Where("organization_id = ? AND intent_key = ?", organizationID, intentKey).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return submission.ExecutionAttempt{}, false, nil
	}
	if err != nil {
		return submission.ExecutionAttempt{}, false, mapError(ctx, err)
	}
	attempt, err := row.attempt()
	return attempt, err == nil, err
}

func (r *Repository) findByID(ctx context.Context, db *gorm.DB, organizationID, attemptID string, lock bool) (submission.ExecutionAttempt, bool, error) {
	query := db.WithContext(ctx).Table(attemptTable)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row executionAttemptRow
	err := query.Where("organization_id = ? AND attempt_id = ?", organizationID, attemptID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return submission.ExecutionAttempt{}, false, nil
	}
	if err != nil {
		return submission.ExecutionAttempt{}, false, mapError(ctx, err)
	}
	attempt, err := row.attempt()
	return attempt, err == nil, err
}

func (r *Repository) findTarget(ctx context.Context, db *gorm.DB, organizationID string, target submission.ExecutionTarget, lock bool) (targetFenceRow, bool, error) {
	query := db.WithContext(ctx).Table(targetTable)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row targetFenceRow
	err := query.Where("organization_id = ? AND platform = ? AND store_id = ? AND subject_id = ?", organizationID, target.Platform, target.StoreID, target.SubjectID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return targetFenceRow{}, false, nil
	}
	if err != nil {
		return targetFenceRow{}, false, mapError(ctx, err)
	}
	return row, true, nil
}

type executionAttemptRow struct {
	OrganizationID       string     `gorm:"column:organization_id"`
	AttemptID            string     `gorm:"column:attempt_id"`
	IntentKey            string     `gorm:"column:intent_key"`
	Platform             string     `gorm:"column:platform"`
	StoreID              string     `gorm:"column:store_id"`
	SubjectID            string     `gorm:"column:subject_id"`
	Action               string     `gorm:"column:action"`
	PayloadFingerprint   string     `gorm:"column:payload_fingerprint"`
	ProviderExecutionKey string     `gorm:"column:provider_execution_key"`
	Status               string     `gorm:"column:status"`
	FenceEpoch           int64      `gorm:"column:fence_epoch"`
	ClaimTokenHash       string     `gorm:"column:claim_token_hash"`
	ClaimOwnerID         string     `gorm:"column:claim_owner_id"`
	LeaseExpiresAt       time.Time  `gorm:"column:lease_expires_at"`
	UnknownReason        *string    `gorm:"column:unknown_reason"`
	EvidenceKind         *string    `gorm:"column:evidence_kind"`
	EvidenceOutcome      *string    `gorm:"column:evidence_outcome"`
	EvidenceReference    *string    `gorm:"column:evidence_reference"`
	EvidenceFingerprint  *string    `gorm:"column:evidence_fingerprint"`
	EvidenceReason       *string    `gorm:"column:evidence_reason"`
	EvidenceAuthorizedBy *string    `gorm:"column:evidence_authorized_by"`
	EvidenceObservedAt   *time.Time `gorm:"column:evidence_observed_at"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
	FinishedAt           *time.Time `gorm:"column:finished_at"`
}

func attemptRowFrom(attempt submission.ExecutionAttempt, claimTokenHash string) executionAttemptRow {
	row := executionAttemptRow{
		OrganizationID: attempt.OrganizationID, AttemptID: attempt.AttemptID, IntentKey: attempt.IntentKey,
		Platform: attempt.Target.Platform, StoreID: attempt.Target.StoreID, SubjectID: attempt.Target.SubjectID,
		Action: attempt.Action, PayloadFingerprint: attempt.PayloadFingerprint, ProviderExecutionKey: attempt.ProviderExecutionKey,
		Status: string(attempt.Status), FenceEpoch: attempt.FenceEpoch, ClaimTokenHash: claimTokenHash,
		ClaimOwnerID:   attempt.ClaimOwnerID,
		LeaseExpiresAt: attempt.LeaseExpiresAt, CreatedAt: attempt.CreatedAt, UpdatedAt: attempt.UpdatedAt, FinishedAt: attempt.FinishedAt,
	}
	if attempt.UnknownReason != "" {
		value := string(attempt.UnknownReason)
		row.UnknownReason = &value
	}
	if attempt.Evidence != nil {
		kind, outcome := string(attempt.Evidence.Kind), string(attempt.Evidence.Outcome)
		row.EvidenceKind, row.EvidenceOutcome = &kind, &outcome
		row.EvidenceReference, row.EvidenceFingerprint = nullable(attempt.Evidence.Reference), nullable(attempt.Evidence.Fingerprint)
		row.EvidenceReason, row.EvidenceAuthorizedBy = nullable(attempt.Evidence.Reason), nullable(attempt.Evidence.AuthorizedBy)
		observed := attempt.Evidence.ObservedAt
		row.EvidenceObservedAt = &observed
	}
	return row
}

func (r executionAttemptRow) mutableValues() map[string]any {
	return map[string]any{
		"status": r.Status, "lease_expires_at": r.LeaseExpiresAt, "unknown_reason": r.UnknownReason,
		"evidence_kind": r.EvidenceKind, "evidence_outcome": r.EvidenceOutcome, "evidence_reference": r.EvidenceReference,
		"evidence_fingerprint": r.EvidenceFingerprint, "evidence_reason": r.EvidenceReason,
		"evidence_authorized_by": r.EvidenceAuthorizedBy, "evidence_observed_at": r.EvidenceObservedAt,
		"updated_at": r.UpdatedAt, "finished_at": r.FinishedAt,
	}
}

func (r executionAttemptRow) attempt() (submission.ExecutionAttempt, error) {
	attempt := submission.ExecutionAttempt{
		OrganizationID: r.OrganizationID, AttemptID: r.AttemptID, IntentKey: r.IntentKey,
		Target: submission.ExecutionTarget{Platform: r.Platform, StoreID: r.StoreID, SubjectID: r.SubjectID},
		Action: r.Action, PayloadFingerprint: r.PayloadFingerprint, ProviderExecutionKey: r.ProviderExecutionKey,
		ClaimOwnerID: r.ClaimOwnerID,
		Status:       submission.ExecutionStatus(r.Status), FenceEpoch: r.FenceEpoch, LeaseExpiresAt: r.LeaseExpiresAt.UTC(),
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(), FinishedAt: utcPointer(r.FinishedAt),
	}
	if r.UnknownReason != nil {
		attempt.UnknownReason = submission.UnknownReason(*r.UnknownReason)
	}
	if r.EvidenceKind != nil || r.EvidenceOutcome != nil || r.EvidenceReference != nil || r.EvidenceFingerprint != nil || r.EvidenceObservedAt != nil {
		if r.EvidenceKind == nil || r.EvidenceOutcome == nil || r.EvidenceReference == nil || r.EvidenceFingerprint == nil || r.EvidenceObservedAt == nil {
			return submission.ExecutionAttempt{}, submission.ErrExecutionUnavailable
		}
		attempt.Evidence = &submission.ExecutionEvidence{
			Kind: submission.EvidenceKind(*r.EvidenceKind), Outcome: submission.ExecutionStatus(*r.EvidenceOutcome),
			Reference: *r.EvidenceReference, Fingerprint: *r.EvidenceFingerprint, Reason: stringValue(r.EvidenceReason),
			AuthorizedBy: stringValue(r.EvidenceAuthorizedBy), ObservedAt: r.EvidenceObservedAt.UTC(),
		}
	}
	if err := submission.ValidatePersistedExecutionAttempt(attempt); err != nil {
		return submission.ExecutionAttempt{}, submission.ErrExecutionUnavailable
	}
	return attempt, nil
}

type targetFenceRow struct {
	OrganizationID   string    `gorm:"column:organization_id"`
	Platform         string    `gorm:"column:platform"`
	StoreID          string    `gorm:"column:store_id"`
	SubjectID        string    `gorm:"column:subject_id"`
	Epoch            int64     `gorm:"column:epoch"`
	CurrentAttemptID string    `gorm:"column:current_attempt_id"`
	CurrentStatus    string    `gorm:"column:current_status"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func advisoryLock(db *gorm.DB, parts ...string) error {
	hash := sha256.New()
	var length [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(part))
	}
	return db.Exec("SELECT pg_advisory_xact_lock(?)", int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))).Error
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	for _, stable := range []error{
		context.Canceled, context.DeadlineExceeded, submission.ErrExecutionInvalid, submission.ErrExecutionNotFound,
		submission.ErrExecutionIntentConflict, submission.ErrExecutionTargetClaimed, submission.ErrExecutionClaimRejected,
		submission.ErrExecutionInvalidTransition, submission.ErrExecutionEvidenceRequired, submission.ErrExecutionUnavailable,
		submission.ErrExecutionOutcomeUnknown,
	} {
		if errors.Is(err, stable) {
			return err
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return submission.ErrExecutionNotFound
	}
	return fmt.Errorf("%w: %v", submission.ErrExecutionUnavailable, err)
}

var _ submission.ExecutionRepository = (*Repository)(nil)
