package submissionpersistence

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/listing/submission"
)

func TestRepositoryPostgresExecutionContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)
	require.NoError(t, InstallSchema(db))

	t.Run("first acquisition returns the only permit and replay is read only", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 0, 0, 0, time.UTC)
		kernel := executionKernel(t, db, func() time.Time { return now })
		command := executionCommand("org-a", "intent-a", "listing-a", `{"title":"one"}`)

		first, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)
		require.NotNil(t, first.Permit)
		require.False(t, first.Replayed)
		require.Equal(t, int64(1), first.Attempt.FenceEpoch)
		require.Equal(t, submission.ExecutionClaimed, first.Attempt.Status)

		replay, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)
		require.True(t, replay.Replayed)
		require.Nil(t, replay.Permit, "a replay must never create a second legal sender")
		require.Equal(t, first.Attempt, replay.Attempt)

		changed := command
		changed.Payload = []byte(`{"title":"different"}`)
		_, err = kernel.Acquire(ctx, changed)
		require.ErrorIs(t, err, submission.ErrExecutionIntentConflict)

		rebuilt := executionKernel(t, db, func() time.Time { return now.Add(time.Minute) })
		persisted, err := rebuilt.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, first.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, first.Attempt, persisted)
	})

	t.Run("same target concurrency yields one committed claim", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 10, 0, 0, time.UTC)
		kernel := executionKernel(t, db, func() time.Time { return now })
		commands := []submission.AcquireExecutionCommand{
			executionCommand("org-a", "intent-a", "listing-shared", `{"title":"one"}`),
			executionCommand("org-a", "intent-b", "listing-shared", `{"title":"two"}`),
		}
		start := make(chan struct{})
		type result struct {
			acquisition submission.ExecutionAcquisition
			err         error
		}
		results := make(chan result, len(commands))
		var ready sync.WaitGroup
		ready.Add(len(commands))
		for _, command := range commands {
			command := command
			go func() {
				ready.Done()
				<-start
				acquisition, err := kernel.Acquire(ctx, command)
				results <- result{acquisition, err}
			}()
		}
		ready.Wait()
		close(start)

		permits, busy := 0, 0
		for range commands {
			var result result
			select {
			case result = <-results:
			case <-time.After(5 * time.Second):
				t.Fatal("bounded wait expired while synchronizing target-fence competitors")
			}
			switch {
			case result.err == nil && result.acquisition.Permit != nil:
				permits++
			case errors.Is(result.err, submission.ErrExecutionTargetClaimed):
				busy++
			default:
				t.Fatalf("unexpected concurrent result: acquisition=%+v err=%v", result.acquisition, result.err)
			}
		}
		require.Equal(t, 1, permits)
		require.Equal(t, 1, busy)
	})

	t.Run("same intent concurrency yields one permit and one replay", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 15, 0, 0, time.UTC)
		kernel := executionKernel(t, db, func() time.Time { return now })
		command := executionCommand("org-a", "intent-shared", "listing-shared", `{"title":"one"}`)
		start := make(chan struct{})
		type result struct {
			acquisition submission.ExecutionAcquisition
			err         error
		}
		results := make(chan result, 2)
		var ready sync.WaitGroup
		ready.Add(2)
		for range 2 {
			go func() {
				ready.Done()
				<-start
				acquisition, err := kernel.Acquire(ctx, command)
				results <- result{acquisition, err}
			}()
		}
		ready.Wait()
		close(start)

		permits, replays := 0, 0
		for range 2 {
			var result result
			select {
			case result = <-results:
			case <-time.After(5 * time.Second):
				t.Fatal("bounded wait expired while synchronizing same-intent competitors")
			}
			require.NoError(t, result.err)
			if result.acquisition.Permit != nil {
				permits++
			}
			if result.acquisition.Replayed {
				replays++
				require.Nil(t, result.acquisition.Permit)
			}
		}
		require.Equal(t, 1, permits)
		require.Equal(t, 1, replays)
	})

	t.Run("response loss and lease expiry remain unknown without resend", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 20, 0, 0, time.UTC)
		clock := func() time.Time { return now }
		kernel := executionKernel(t, db, clock)
		command := executionCommand("org-a", "intent-a", "listing-a", `{"title":"one"}`)
		command.Lease = time.Second
		acquired, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)

		now = now.Add(2 * time.Second)
		expired, err := kernel.Expire(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionOutcomeUnknown, expired.Status)
		require.Equal(t, submission.UnknownLeaseExpired, expired.UnknownReason)

		late := providerEvidence(submission.ExecutionSucceeded, "late", now)
		_, err = kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), late)
		require.ErrorIs(t, err, submission.ErrExecutionClaimRejected)

		replay, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)
		require.True(t, replay.Replayed)
		require.Nil(t, replay.Permit)
		_, err = kernel.Acquire(ctx, executionCommand("org-a", "intent-b", "listing-a", `{"title":"two"}`))
		require.ErrorIs(t, err, submission.ErrExecutionTargetClaimed)
	})

	t.Run("claim owner token lease and duplicate completion are fenced", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 25, 0, 0, time.UTC)
		kernel := executionKernel(t, db, func() time.Time { return now })
		command := executionCommand("org-a", "intent-a", "listing-a", `{"title":"one"}`)
		command.Lease = time.Minute
		acquired, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)

		wrongOwner := permitClaim("org-a", acquired.Permit)
		wrongOwner.OwnerID = "worker-other"
		_, err = kernel.MarkUnknown(ctx, wrongOwner, submission.UnknownResponseLost)
		require.ErrorIs(t, err, submission.ErrExecutionClaimRejected)

		now = now.Add(30 * time.Second)
		renewed, err := kernel.Renew(ctx, permitClaim("org-a", acquired.Permit), 2*time.Minute)
		require.NoError(t, err)
		require.Equal(t, now.Add(2*time.Minute), renewed.LeaseExpiresAt)

		now = now.Add(time.Minute)
		now = now.Add(789 * time.Nanosecond)
		evidence := providerEvidence(submission.ExecutionSucceeded, "response-a", now)
		completed, err := kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), evidence)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionSucceeded, completed.Status)
		duplicate, err := kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), evidence)
		require.NoError(t, err)
		require.Equal(t, completed, duplicate)

		next, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-b", "listing-a", `{"title":"two"}`))
		require.NoError(t, err)
		require.Equal(t, int64(2), next.Attempt.FenceEpoch)
		duplicate, err = kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), evidence)
		require.NoError(t, err)
		require.Equal(t, completed, duplicate)
		conflicting := evidence
		conflicting.Fingerprint = executionTestDigest("different")
		conflicting.Reference = "different-response"
		_, err = kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), conflicting)
		require.ErrorIs(t, err, submission.ErrExecutionIntentConflict)
	})

	t.Run("qualified resolution releases target to the next fenced intent", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 30, 0, 0, time.UTC)
		kernel := executionKernel(t, db, func() time.Time { return now })
		first, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-a", "listing-a", `{"title":"one"}`))
		require.NoError(t, err)
		unknown, err := kernel.MarkUnknown(ctx, permitClaim("org-a", first.Permit), submission.UnknownResponseLost)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionOutcomeUnknown, unknown.Status)

		now = now.Add(time.Minute)
		resolved, err := kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, first.Attempt.AttemptID, first.Attempt.FenceEpoch,
			readBackEvidence(submission.ExecutionSucceeded, "readback-a", now))
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionSucceeded, resolved.Status)

		duplicate, err := kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, first.Attempt.AttemptID, first.Attempt.FenceEpoch,
			readBackEvidence(submission.ExecutionSucceeded, "readback-a", now))
		require.NoError(t, err)
		require.Equal(t, resolved, duplicate)

		second, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-b", "listing-a", `{"title":"two"}`))
		require.NoError(t, err)
		require.NotNil(t, second.Permit)
		require.Equal(t, int64(2), second.Attempt.FenceEpoch)
		replayedAfterFenceAdvance, err := kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, first.Attempt.AttemptID, first.Attempt.FenceEpoch,
			readBackEvidence(submission.ExecutionSucceeded, "readback-a", now))
		require.NoError(t, err)
		require.Equal(t, resolved, replayedAfterFenceAdvance)

		_, err = kernel.Complete(ctx, permitClaim("org-a", first.Permit), providerEvidence(submission.ExecutionSucceeded, "late-overwrite", now))
		require.Error(t, err)
		current, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, second.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionClaimed, current.Status)
	})

	t.Run("manual cancellation requires an outer authorization decision and no-side-effect evidence", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 35, 0, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return now }))
		require.NoError(t, err)
		acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-a", "listing-a", `{"title":"one"}`))
		require.NoError(t, err)
		_, err = kernel.MarkUnknown(ctx, permitClaim("org-a", acquired.Permit), submission.UnknownExecutionCancelled)
		require.NoError(t, err)
		now = now.Add(time.Minute)
		manual := submission.ExecutionEvidence{
			Kind: submission.EvidenceManualResolution, Outcome: submission.ExecutionCancelled,
			Reference: "incident-42", Fingerprint: executionTestDigest("not-sent"),
			Reason: "operator verified that the provider never received the request", ObservedAt: now,
		}
		_, err = kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID, acquired.Attempt.FenceEpoch, manual)
		require.ErrorIs(t, err, submission.ErrExecutionEvidenceRequired)

		forged := manual
		forged.AuthorizedBy = "caller-claimed-operator"
		_, err = kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID, acquired.Attempt.FenceEpoch, forged)
		require.ErrorIs(t, err, submission.ErrExecutionInvalid)

		authorizedKernel, err := submission.NewExecutionKernel(repository,
			submission.WithExecutionClock(func() time.Time { return now }),
			submission.WithManualResolutionAuthorizer(manualAuthorizerFunc(func(_ context.Context, scope submission.ExecutionScope, attemptID string) (string, error) {
				require.Equal(t, "org-a", scope.OrganizationID)
				require.Equal(t, acquired.Attempt.AttemptID, attemptID)
				return "operator-a", nil
			})),
		)
		require.NoError(t, err)
		cancelled, err := authorizedKernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID, acquired.Attempt.FenceEpoch, manual)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionCancelled, cancelled.Status)
		require.Equal(t, "operator-a", cancelled.Evidence.AuthorizedBy)
	})

	t.Run("organization scope is present on every read and mutation", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 40, 0, 0, time.UTC)
		kernel := executionKernel(t, db, func() time.Time { return now })
		acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-a", "listing-a", `{"title":"one"}`))
		require.NoError(t, err)

		_, err = kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-b"}, acquired.Attempt.AttemptID)
		require.ErrorIs(t, err, submission.ErrExecutionNotFound)
		wrongClaim := permitClaim("org-b", acquired.Permit)
		_, err = kernel.MarkUnknown(ctx, wrongClaim, submission.UnknownResponseLost)
		require.ErrorIs(t, err, submission.ErrExecutionNotFound)

		persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionClaimed, persisted.Status)
	})

	t.Run("precommit failure and cancelled context create no half state", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 50, 0, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		repository.fault = func(stage string) error {
			if stage == "before_commit" {
				return errors.New("injected commit failure")
			}
			return nil
		}
		kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return now }))
		require.NoError(t, err)
		command := executionCommand("org-a", "intent-a", "listing-a", `{"title":"one"}`)
		failed, err := kernel.Acquire(ctx, command)
		require.ErrorIs(t, err, submission.ErrExecutionUnavailable)
		require.Nil(t, failed.Permit)

		repository.fault = nil
		var count int64
		require.NoError(t, db.Table(AttemptTable).Count(&count).Error)
		require.Zero(t, count)
		require.NoError(t, db.Table(TargetFenceTable).Count(&count).Error)
		require.Zero(t, count)

		cancelled, cancelCall := context.WithCancel(ctx)
		cancelCall()
		failed, err = kernel.Acquire(cancelled, command)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, failed.Permit)
		require.NoError(t, db.Table(AttemptTable).Count(&count).Error)
		require.Zero(t, count)
	})

	t.Run("commit acknowledgement loss exposes no permit and replay cannot resend", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 55, 0, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		repository.fault = func(stage string) error {
			if stage == "after_commit" {
				return errors.New("commit acknowledgement lost")
			}
			return nil
		}
		kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return now }))
		require.NoError(t, err)
		command := executionCommand("org-a", "intent-ack-loss", "listing-a", `{"title":"one"}`)

		uncertain, err := kernel.Acquire(ctx, command)
		require.ErrorIs(t, err, submission.ErrExecutionOutcomeUnknown)
		require.Nil(t, uncertain.Permit)

		repository.fault = nil
		replay, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)
		require.True(t, replay.Replayed)
		require.Nil(t, replay.Permit)
		require.Equal(t, submission.ExecutionClaimed, replay.Attempt.Status)
	})

	t.Run("evidence reason character bound is enforced before persistence", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		kernel, err := submission.NewExecutionKernel(repository,
			submission.WithExecutionClock(func() time.Time { return now }),
			submission.WithManualResolutionAuthorizer(manualAuthorizerFunc(func(context.Context, submission.ExecutionScope, string) (string, error) {
				return "operator-a", nil
			})),
		)
		require.NoError(t, err)
		tooLong := strings.Repeat("界", 513)
		atBoundary := strings.Repeat("界", 512)

		provider, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-reason-provider", "listing-reason-provider", `{"title":"one"}`))
		require.NoError(t, err)
		providerRejected := providerEvidence(submission.ExecutionFailedDefinitive, "provider-rejected", now)
		providerRejected.Reason = tooLong
		_, err = kernel.Complete(ctx, permitClaim("org-a", provider.Permit), providerRejected)
		require.ErrorIs(t, err, submission.ErrExecutionEvidenceRequired)
		persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, provider.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionClaimed, persisted.Status)
		replay, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-reason-provider", "listing-reason-provider", `{"title":"one"}`))
		require.NoError(t, err)
		require.True(t, replay.Replayed)
		require.Nil(t, replay.Permit)
		_, err = kernel.Acquire(ctx, executionCommand("org-a", "intent-reason-blocked", "listing-reason-provider", `{"title":"two"}`))
		require.ErrorIs(t, err, submission.ErrExecutionTargetClaimed)
		providerRejected.Reason = atBoundary
		completed, err := kernel.Complete(ctx, permitClaim("org-a", provider.Permit), providerRejected)
		require.NoError(t, err)
		require.Equal(t, atBoundary, completed.Evidence.Reason)
		completedReplay, err := kernel.Complete(ctx, permitClaim("org-a", provider.Permit), providerRejected)
		require.NoError(t, err)
		require.Equal(t, completed, completedReplay)

		readbackCommand := executionCommand("org-a", "intent-reason-readback", "listing-reason-readback", `{"title":"one"}`)
		readback, err := kernel.Acquire(ctx, readbackCommand)
		require.NoError(t, err)
		_, err = kernel.MarkUnknown(ctx, permitClaim("org-a", readback.Permit), submission.UnknownResponseLost)
		require.NoError(t, err)
		readbackRejected := readBackEvidence(submission.ExecutionFailedDefinitive, "readback-rejected", now)
		readbackRejected.Reason = tooLong
		_, err = kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, readback.Attempt.AttemptID, readback.Attempt.FenceEpoch, readbackRejected)
		require.ErrorIs(t, err, submission.ErrExecutionEvidenceRequired)
		persisted, err = kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, readback.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionOutcomeUnknown, persisted.Status)
		readbackReplay, err := kernel.Acquire(ctx, readbackCommand)
		require.NoError(t, err)
		require.True(t, readbackReplay.Replayed)
		require.Nil(t, readbackReplay.Permit)
		_, err = kernel.Acquire(ctx, executionCommand("org-a", "intent-reason-readback-blocked", "listing-reason-readback", `{"title":"two"}`))
		require.ErrorIs(t, err, submission.ErrExecutionTargetClaimed)
		readbackRejected.Reason = atBoundary
		resolved, err := kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, readback.Attempt.AttemptID, readback.Attempt.FenceEpoch, readbackRejected)
		require.NoError(t, err)
		require.Equal(t, atBoundary, resolved.Evidence.Reason)
		resolvedReplay, err := kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, readback.Attempt.AttemptID, readback.Attempt.FenceEpoch, readbackRejected)
		require.NoError(t, err)
		require.Equal(t, resolved, resolvedReplay)

		manualCommand := executionCommand("org-a", "intent-reason-manual", "listing-reason-manual", `{"title":"one"}`)
		manual, err := kernel.Acquire(ctx, manualCommand)
		require.NoError(t, err)
		_, err = kernel.MarkUnknown(ctx, permitClaim("org-a", manual.Permit), submission.UnknownExecutionCancelled)
		require.NoError(t, err)
		manualCancelled := submission.ExecutionEvidence{
			Kind: submission.EvidenceManualResolution, Outcome: submission.ExecutionCancelled,
			Reference: "incident-reason", Fingerprint: executionTestDigest("manual-reason"),
			Reason: tooLong, ObservedAt: now,
		}
		_, err = kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, manual.Attempt.AttemptID, manual.Attempt.FenceEpoch, manualCancelled)
		require.ErrorIs(t, err, submission.ErrExecutionEvidenceRequired)
		persisted, err = kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, manual.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionOutcomeUnknown, persisted.Status)
		manualReplay, err := kernel.Acquire(ctx, manualCommand)
		require.NoError(t, err)
		require.True(t, manualReplay.Replayed)
		require.Nil(t, manualReplay.Permit)
		_, err = kernel.Acquire(ctx, executionCommand("org-a", "intent-reason-manual-blocked", "listing-reason-manual", `{"title":"two"}`))
		require.ErrorIs(t, err, submission.ErrExecutionTargetClaimed)
		manualCancelled.Reason = atBoundary
		cancelled, err := kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, manual.Attempt.AttemptID, manual.Attempt.FenceEpoch, manualCancelled)
		require.NoError(t, err)
		require.Equal(t, atBoundary, cancelled.Evidence.Reason)
		cancelledReplay, err := kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, manual.Attempt.AttemptID, manual.Attempt.FenceEpoch, manualCancelled)
		require.NoError(t, err)
		require.Equal(t, cancelled, cancelledReplay)
	})
}

func TestNewRepositoryRejectsPostgresSchemaDrift(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)

	mutations := []struct {
		name       string
		statements []string
	}{
		{
			name:       "wrong column type with unchanged name",
			statements: []string{"ALTER TABLE public." + AttemptTable + " ALTER COLUMN evidence_reason TYPE TEXT"},
		},
		{
			name:       "wrong nullability with unchanged name",
			statements: []string{"ALTER TABLE public." + AttemptTable + " ALTER COLUMN evidence_reason SET NOT NULL"},
		},
		{
			name:       "wrong target column length with unchanged name",
			statements: []string{"ALTER TABLE public." + TargetFenceTable + " ALTER COLUMN subject_id TYPE VARCHAR(127)"},
		},
		{
			name:       "wrong target nullability with unchanged name",
			statements: []string{"ALTER TABLE public." + TargetFenceTable + " ALTER COLUMN updated_at DROP NOT NULL"},
		},
		{
			name: "missing organization-qualified primary key",
			statements: []string{
				"ALTER TABLE public." + TargetFenceTable + " DROP CONSTRAINT listing_submission_target_fences_attempt_fkey",
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_pkey",
			},
		},
		{
			name: "wrong same-name intent uniqueness",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_intent_unique",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_intent_unique UNIQUE (organization_id, action, intent_key)",
			},
		},
		{
			name: "wrong same-name provider-key uniqueness",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_provider_key_unique",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_provider_key_unique UNIQUE (organization_id, action, provider_execution_key)",
			},
		},
		{
			name: "wrong same-name target primary key without organization",
			statements: []string{
				"ALTER TABLE public." + TargetFenceTable + " DROP CONSTRAINT listing_submission_target_fences_pkey",
				"ALTER TABLE public." + TargetFenceTable + " ADD CONSTRAINT listing_submission_target_fences_pkey PRIMARY KEY (platform, store_id, subject_id)",
			},
		},
		{
			name: "wrong same-name target foreign key",
			statements: []string{
				"ALTER TABLE public." + TargetFenceTable + " DROP CONSTRAINT listing_submission_target_fences_attempt_fkey",
				"ALTER TABLE public." + TargetFenceTable + " ADD CONSTRAINT listing_submission_target_fences_attempt_fkey FOREIGN KEY (organization_id, current_attempt_id) REFERENCES public." + AttemptTable + " (organization_id, attempt_id) ON DELETE CASCADE",
			},
		},
		{
			name: "same-name state constraint retains keywords but is relaxed",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_state_shape_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_state_shape_check CHECK ((status = 'claimed' AND unknown_reason IS NULL AND evidence_kind IS NULL AND finished_at IS NULL) OR (status = 'outcome_unknown' AND unknown_reason IN ('response_lost', 'lease_expired', 'execution_cancelled') AND evidence_kind IS NULL AND finished_at IS NULL) OR (status IN ('succeeded', 'failed_definitive', 'cancelled') AND unknown_reason IS NULL AND evidence_kind IS NOT NULL AND evidence_outcome = status AND evidence_reference IS NOT NULL AND evidence_fingerprint IS NOT NULL AND evidence_observed_at IS NOT NULL AND finished_at IS NOT NULL) OR true)",
			},
		},
		{
			name: "same-name status constraint changes literal case",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_status_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_status_check CHECK (status IN ('CLAIMED', 'outcome_unknown', 'succeeded', 'failed_definitive', 'cancelled'))",
			},
		},
		{
			name: "same-name digest constraint changes regex literal case",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_digest_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_digest_check CHECK (payload_fingerprint ~ '^[0-9A-F]{64}$' AND provider_execution_key ~ '^subk1_v1_[0-9A-F]{64}$' AND claim_token_hash ~ '^[0-9A-F]{64}$' AND (evidence_fingerprint IS NULL OR evidence_fingerprint ~ '^[0-9A-F]{64}$'))",
			},
		},
		{
			name: "same-name constraint is not validated",
			statements: []string{
				"ALTER TABLE public." + TargetFenceTable + " DROP CONSTRAINT listing_submission_target_fences_epoch_check",
				"ALTER TABLE public." + TargetFenceTable + " ADD CONSTRAINT listing_submission_target_fences_epoch_check CHECK (epoch > 0) NOT VALID",
			},
		},
	}

	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			reinstallExecutionSchema(t, db)
			for _, statement := range tc.statements {
				require.NoError(t, db.Exec(statement).Error)
			}
			repository, err := NewRepository(db)
			require.Nil(t, repository)
			require.ErrorIs(t, err, submission.ErrExecutionUnavailable)
		})
	}

	t.Run("correct schema is admitted", func(t *testing.T) {
		reinstallExecutionSchema(t, db)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		require.NotNil(t, repository)
	})
}

func executionKernel(t *testing.T, db *gorm.DB, clock func() time.Time) *submission.ExecutionKernel {
	t.Helper()
	repository, err := NewRepository(db)
	require.NoError(t, err)
	kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(clock))
	require.NoError(t, err)
	return kernel
}

func executionCommand(org, intent, subject, payload string) submission.AcquireExecutionCommand {
	return submission.AcquireExecutionCommand{
		Scope: submission.ExecutionScope{OrganizationID: org}, IntentKey: intent,
		Target: submission.ExecutionTarget{Platform: "shein", StoreID: "store-a", SubjectID: subject},
		Action: "save_draft", Payload: []byte(payload), ClaimOwnerID: "worker-a", Lease: 5 * time.Minute,
	}
}

func permitClaim(org string, permit *submission.SendPermit) submission.ExecutionClaim {
	return submission.ExecutionClaim{Scope: submission.ExecutionScope{OrganizationID: org}, AttemptID: permit.AttemptID, FenceEpoch: permit.FenceEpoch, OwnerID: permit.ClaimOwnerID, Token: permit.ClaimToken}
}

func providerEvidence(outcome submission.ExecutionStatus, reference string, now time.Time) submission.ExecutionEvidence {
	evidence := submission.ExecutionEvidence{Kind: submission.EvidenceProviderResponse, Outcome: outcome, Reference: reference, Fingerprint: executionTestDigest(reference), ObservedAt: now}
	if outcome == submission.ExecutionFailedDefinitive {
		evidence.Reason = "provider rejected the request before applying a side effect"
	}
	return evidence
}

func readBackEvidence(outcome submission.ExecutionStatus, reference string, now time.Time) submission.ExecutionEvidence {
	evidence := providerEvidence(outcome, reference, now)
	evidence.Kind = submission.EvidenceProviderReadBack
	return evidence
}

func executionTestDigest(value string) string {
	return "4778c951699235578c528c0b73bb72b252259125f504120d1285c79ff944ea90" // deterministic valid digest; reference still prevents aliasing
}

type manualAuthorizerFunc func(context.Context, submission.ExecutionScope, string) (string, error)

func (f manualAuthorizerFunc) AuthorizeManualResolution(ctx context.Context, scope submission.ExecutionScope, attemptID string) (string, error) {
	return f(ctx, scope, attemptID)
}

func resetExecutionTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("TRUNCATE TABLE "+TargetFenceTable+", "+AttemptTable+" CASCADE").Error)
}

func reinstallExecutionSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS public."+TargetFenceTable+", public."+AttemptTable+" CASCADE").Error)
	require.NoError(t, InstallSchema(db))
}

func openExecutionPostgres(t *testing.T, ctx context.Context) (*gorm.DB, interface{ Close() error }) {
	t.Helper()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("submission_kernel"),
		tcpostgres.WithUsername("submission_kernel"),
		tcpostgres.WithPassword("submission_kernel"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, sqlDB
}

func init() {
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		_ = os.Setenv("DOCKER_HOST", "npipe:////./pipe/dockerDesktopLinuxEngine")
	}
}
