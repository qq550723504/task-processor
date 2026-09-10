package submissionpersistence

import (
	"context"
	"errors"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/listing/submission"
)

func TestRepositoryPostgresMutationLockTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)
	require.NoError(t, InstallSchema(db))
	for _, name := range []string{"complete expired attempt lock", "complete expired fence lock", "complete after renewal", "mark unknown after renewal", "resolve unknown", "expire"} {
		t.Run(name, func(t *testing.T) {
			resetExecutionTables(t, db)
			base := time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)
			serviceNow := base
			var transactionNanos atomic.Int64
			transactionNanos.Store(base.UnixNano())
			repository, err := NewRepository(db)
			require.NoError(t, err)
			repository.now = func() time.Time { return time.Unix(0, transactionNanos.Load()).UTC() }
			kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return serviceNow }))
			require.NoError(t, err)
			command := executionCommand("org-a", "lock-time", "lock-time", `{"title":"one"}`)
			command.Lease = time.Second
			acquired, err := kernel.Acquire(ctx, command)
			require.NoError(t, err)
			claim := permitClaim("org-a", acquired.Permit)
			scope := submission.ExecutionScope{OrganizationID: "org-a"}
			serviceNow = base.Add(500 * time.Millisecond)
			transactionNanos.Store(serviceNow.UnixNano())
			_, err = kernel.Renew(ctx, claim, 2*time.Second)
			require.NoError(t, err)
			if name == "resolve unknown" {
				_, err = kernel.MarkUnknown(ctx, claim, submission.UnknownResponseLost)
				require.NoError(t, err)
			}
			// The caller captured time before the competing renewal. The actual
			// mutation must use time after its PostgreSQL lock wait instead.
			serviceNow = base.Add(250 * time.Millisecond)
			lockedNow := base.Add(time.Second)
			wantStatus := submission.ExecutionSucceeded
			var wantErr error
			if strings.Contains(name, "expired") || name == "expire" {
				lockedNow = base.Add(2500 * time.Millisecond)
				wantStatus = submission.ExecutionOutcomeUnknown
				if name != "expire" {
					wantErr = submission.ErrExecutionClaimRejected
				}
			} else if name == "mark unknown after renewal" {
				wantStatus = submission.ExecutionOutcomeUnknown
			}
			holder := db.WithContext(ctx).Begin()
			require.NoError(t, holder.Error)
			t.Cleanup(func() { _ = holder.Rollback().Error })
			var holderPID int
			require.NoError(t, holder.Raw("SELECT pg_backend_pid()").Scan(&holderPID).Error)
			lockTable := attemptTable
			if name == "complete expired fence lock" {
				lockTable = targetTable
			}
			require.NoError(t, holder.Exec("SELECT 1 FROM "+lockTable+" FOR UPDATE").Error)
			completed := make(chan error, 1)
			go func() {
				var mutationErr error
				switch name {
				case "mark unknown after renewal":
					_, mutationErr = kernel.MarkUnknown(ctx, claim, submission.UnknownResponseLost)
				case "resolve unknown":
					_, mutationErr = kernel.ResolveUnknown(ctx, scope, claim.AttemptID, claim.FenceEpoch, readBackEvidence(submission.ExecutionSucceeded, "readback-lock-time", base))
				case "expire":
					_, mutationErr = kernel.Expire(ctx, scope, claim.AttemptID)
				default:
					_, mutationErr = kernel.Complete(ctx, claim, providerEvidence(submission.ExecutionSucceeded, "response-lock-time", base))
				}
				completed <- mutationErr
			}()
			require.Eventually(t, func() bool {
				var blocked bool
				return db.Raw("SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_stat_activity WHERE ? = ANY(pg_catalog.pg_blocking_pids(pid)))", holderPID).Scan(&blocked).Error == nil && blocked
			}, 5*time.Second, 10*time.Millisecond, "mutation must actually wait for the held PostgreSQL lock")
			transactionNanos.Store(lockedNow.UnixNano())
			require.NoError(t, holder.Commit().Error)
			select {
			case err = <-completed:
			case <-time.After(5 * time.Second):
				t.Fatal("mutation did not finish after releasing its PostgreSQL lock")
			}
			require.ErrorIs(t, err, wantErr)
			persisted, err := kernel.Get(ctx, scope, claim.AttemptID)
			require.NoError(t, err)
			require.Equal(t, wantStatus, persisted.Status)
			require.Equal(t, lockedNow, persisted.UpdatedAt)
			if wantStatus == submission.ExecutionSucceeded {
				require.Equal(t, &lockedNow, persisted.FinishedAt)
			} else {
				require.Nil(t, persisted.FinishedAt)
				require.Nil(t, persisted.Evidence)
			}
			var fence targetFenceRow
			require.NoError(t, db.Table(targetTable).Take(&fence).Error)
			require.Equal(t, string(wantStatus), fence.CurrentStatus)
			require.Equal(t, lockedNow, fence.UpdatedAt.UTC())
		})
	}
}

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

	t.Run("lease deadlines match committed PostgreSQL precision", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 27, 0, 456789500, time.FixedZone("test", 8*60*60))
		kernel := executionKernel(t, db, func() time.Time { return now })
		command := executionCommand("org-a", "intent-lease-precision", "listing-lease-precision", `{"title":"one"}`)
		command.Lease = time.Minute + 789*time.Nanosecond

		acquired, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)
		expectedInitial := now.UTC().Truncate(time.Microsecond).Add(command.Lease).Truncate(time.Microsecond)
		require.Equal(t, expectedInitial, acquired.Attempt.LeaseExpiresAt)
		require.Equal(t, expectedInitial, acquired.Permit.LeaseExpiresAt)
		persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, expectedInitial, persisted.LeaseExpiresAt)

		now = now.Add(30*time.Second + 321*time.Nanosecond)
		renewLease := 2*time.Minute + 999*time.Nanosecond
		renewed, err := kernel.Renew(ctx, permitClaim("org-a", acquired.Permit), renewLease)
		require.NoError(t, err)
		expectedRenewed := now.UTC().Truncate(time.Microsecond).Add(renewLease).Truncate(time.Microsecond)
		require.Equal(t, expectedRenewed, renewed.LeaseExpiresAt)
		persisted, err = kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, expectedRenewed, persisted.LeaseExpiresAt)

		now = expectedRenewed
		_, err = kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), providerEvidence(submission.ExecutionSucceeded, "at-expiry", now))
		require.ErrorIs(t, err, submission.ErrExecutionClaimRejected)
		persisted, err = kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionOutcomeUnknown, persisted.Status)
		require.Equal(t, submission.UnknownLeaseExpired, persisted.UnknownReason)
	})

	t.Run("lease expiry during persistence never returns live authority", func(t *testing.T) {
		for _, elapsed := range []time.Duration{time.Second, time.Second + time.Microsecond} {
			name := "at expiry"
			if elapsed > time.Second {
				name = "after expiry"
			}
			t.Run("acquire "+name, func(t *testing.T) {
				resetExecutionTables(t, db)
				base := time.Date(2026, 9, 10, 2, 29, 0, 0, time.UTC)
				now := base
				repository, err := NewRepository(db)
				require.NoError(t, err)
				repository.fault = func(stage string) error {
					if stage == "before_commit" {
						now = base.Add(elapsed)
					}
					return nil
				}
				const attemptID = "01890f5e-7b3d-7cc0-98a1-123456789abc"
				kernel, err := submission.NewExecutionKernel(repository,
					submission.WithExecutionClock(func() time.Time { return now }),
					submission.WithExecutionIDGenerator(func() (string, error) { return attemptID, nil }),
					submission.WithExecutionClaimTokenGenerator(func() (string, error) { return "lease-expiry-token", nil }),
				)
				require.NoError(t, err)
				command := executionCommand("org-a", "lease-expiry-intent", "lease-expiry-target", `{"title":"one"}`)
				command.Lease = time.Second

				acquisition, err := kernel.Acquire(ctx, command)
				require.ErrorIs(t, err, submission.ErrExecutionClaimRejected)
				require.Nil(t, acquisition.Permit)
				repository.fault = nil

				persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, attemptID)
				require.NoError(t, err)
				require.Equal(t, submission.ExecutionClaimed, persisted.Status)
				require.Equal(t, base.Add(time.Second), persisted.LeaseExpiresAt)
				replay, err := kernel.Acquire(ctx, command)
				require.NoError(t, err)
				require.True(t, replay.Replayed)
				require.Nil(t, replay.Permit)
				_, err = kernel.Acquire(ctx, executionCommand("org-a", "lease-expiry-blocked", "lease-expiry-target", `{"title":"two"}`))
				require.ErrorIs(t, err, submission.ErrExecutionTargetClaimed)
			})
		}

		t.Run("renew rechecks the old lease after locking", func(t *testing.T) {
			resetExecutionTables(t, db)
			base := time.Date(2026, 9, 10, 2, 29, 10, 0, time.UTC)
			serviceNow, transactionNow := base, base
			repository, err := NewRepository(db)
			require.NoError(t, err)
			repository.now = func() time.Time { return transactionNow }
			kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return serviceNow }))
			require.NoError(t, err)
			command := executionCommand("org-a", "renew-lock-expiry", "renew-lock-expiry", `{"title":"one"}`)
			command.Lease = time.Second
			acquired, err := kernel.Acquire(ctx, command)
			require.NoError(t, err)

			serviceNow = base.Add(500 * time.Millisecond)
			transactionNow = acquired.Attempt.LeaseExpiresAt
			renewed, err := kernel.Renew(ctx, permitClaim("org-a", acquired.Permit), time.Second)
			require.ErrorIs(t, err, submission.ErrExecutionClaimRejected)
			require.Equal(t, submission.ExecutionAttempt{}, renewed)
			persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
			require.NoError(t, err)
			require.Equal(t, submission.ExecutionOutcomeUnknown, persisted.Status)
			require.Equal(t, submission.UnknownLeaseExpired, persisted.UnknownReason)
		})

		t.Run("renew refuses a new lease that expires before return", func(t *testing.T) {
			resetExecutionTables(t, db)
			base := time.Date(2026, 9, 10, 2, 29, 20, 0, time.UTC)
			serviceNow, transactionNow := base, base
			repository, err := NewRepository(db)
			require.NoError(t, err)
			repository.now = func() time.Time { return transactionNow }
			kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return serviceNow }))
			require.NoError(t, err)
			command := executionCommand("org-a", "renew-return-expiry", "renew-return-expiry", `{"title":"one"}`)
			command.Lease = time.Second
			acquired, err := kernel.Acquire(ctx, command)
			require.NoError(t, err)

			serviceNow = base.Add(500 * time.Millisecond)
			transactionNow = serviceNow
			repository.fault = func(stage string) error {
				if stage == "before_commit" {
					serviceNow = base.Add(2 * time.Second)
				}
				return nil
			}
			renewed, err := kernel.Renew(ctx, permitClaim("org-a", acquired.Permit), time.Second)
			require.ErrorIs(t, err, submission.ErrExecutionClaimRejected)
			require.Equal(t, submission.ExecutionAttempt{}, renewed)
			repository.fault = nil
			persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
			require.NoError(t, err)
			require.Equal(t, submission.ExecutionClaimed, persisted.Status)
			require.Equal(t, base.Add(1500*time.Millisecond), persisted.LeaseExpiresAt)
			transactionNow = serviceNow
			_, err = kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), providerEvidence(submission.ExecutionSucceeded, "renew-expired", serviceNow))
			require.ErrorIs(t, err, submission.ErrExecutionClaimRejected)
		})
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
		repository.now = func() time.Time { return now }
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

	t.Run("write uow forces synchronous commit locally", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 52, 0, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return now }))
		require.NoError(t, err)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		t.Cleanup(func() {
			if cleanupErr := db.Exec("SET synchronous_commit = on").Error; cleanupErr != nil {
				t.Errorf("restore synchronous_commit: %v", cleanupErr)
			}
			if cleanupErr := db.Exec("DROP TRIGGER IF EXISTS require_submission_synchronous_commit ON public." + AttemptTable).Error; cleanupErr != nil {
				t.Errorf("drop synchronous commit trigger: %v", cleanupErr)
			}
			if cleanupErr := db.Exec("DROP FUNCTION IF EXISTS public.require_submission_synchronous_commit()").Error; cleanupErr != nil {
				t.Errorf("drop synchronous commit function: %v", cleanupErr)
			}
			sqlDB.SetMaxOpenConns(8)
			sqlDB.SetMaxIdleConns(2)
		})
		require.NoError(t, db.Exec(`
CREATE FUNCTION public.require_submission_synchronous_commit() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF current_setting('synchronous_commit') <> 'on' THEN
        RAISE EXCEPTION 'submission write requires synchronous_commit=on';
    END IF;
    RETURN NEW;
END
$$`).Error)
		require.NoError(t, db.Exec("CREATE TRIGGER require_submission_synchronous_commit BEFORE INSERT ON public."+AttemptTable+" FOR EACH ROW EXECUTE FUNCTION public.require_submission_synchronous_commit()").Error)
		require.NoError(t, db.Exec("SET synchronous_commit = off").Error)

		acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-sync-commit", "listing-sync-commit", `{"title":"one"}`))
		require.NoError(t, err)
		require.NotNil(t, acquired.Permit)

		var setting string
		require.NoError(t, db.Raw("SHOW synchronous_commit").Scan(&setting).Error)
		require.Equal(t, "off", setting, "SET LOCAL must not leak into the pooled session")
	})

	t.Run("synchronous commit setup failure creates no permit", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 54, 0, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		repository.fault = func(stage string) error {
			if stage == "synchronous_commit" {
				return errors.New("injected synchronous commit setup failure")
			}
			return nil
		}
		kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return now }))
		require.NoError(t, err)

		failed, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-sync-failure", "listing-sync-failure", `{"title":"one"}`))
		require.ErrorIs(t, err, submission.ErrExecutionUnavailable)
		require.Nil(t, failed.Permit)

		var count int64
		require.NoError(t, db.Table(AttemptTable).Count(&count).Error)
		require.Zero(t, count)
		require.NoError(t, db.Table(TargetFenceTable).Count(&count).Error)
		require.Zero(t, count)
	})

	t.Run("database rejects null required state fields", func(t *testing.T) {
		tests := []struct {
			name   string
			intent string
			mutate func(*executionAttemptRow)
		}{
			{
				name:   "outcome unknown without reason",
				intent: "intent-null-unknown-reason",
				mutate: func(row *executionAttemptRow) {
					row.Status = string(submission.ExecutionOutcomeUnknown)
					row.UnknownReason = nil
				},
			},
			{
				name:   "terminal evidence without outcome",
				intent: "intent-null-evidence-outcome",
				mutate: func(row *executionAttemptRow) {
					kind := string(submission.EvidenceProviderResponse)
					reference, fingerprint := "provider-reference", executionTestDigest("provider-reference")
					observedAt, finishedAt := row.UpdatedAt, row.UpdatedAt
					row.Status = string(submission.ExecutionSucceeded)
					row.EvidenceKind = &kind
					row.EvidenceOutcome = nil
					row.EvidenceReference = &reference
					row.EvidenceFingerprint = &fingerprint
					row.EvidenceObservedAt = &observedAt
					row.FinishedAt = &finishedAt
				},
			},
			{
				name:   "claimed state with stray evidence field",
				intent: "intent-claimed-stray-evidence",
				mutate: func(row *executionAttemptRow) {
					reason := "must not be hidden by read-back mapping"
					row.EvidenceReason = &reason
				},
			},
		}
		for index, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				resetExecutionTables(t, db)
				now := time.Date(2026, 9, 10, 2, 55+index, 0, 0, time.UTC)
				kernel := executionKernel(t, db, func() time.Time { return now })
				acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-source", "listing-source", `{"title":"one"}`))
				require.NoError(t, err)

				row := attemptRowFrom(acquired.Attempt, submission.ExecutionClaimTokenHash(acquired.Permit.ClaimToken))
				row.AttemptID = []string{"01890f5e-7b3d-7cc0-98a1-123456789abc", "01890f5e-7b3d-7cc0-98a1-123456789abd", "01890f5e-7b3d-7cc0-98a1-123456789abe"}[index]
				row.IntentKey = tc.intent
				row.SubjectID = "listing-" + tc.intent
				row.ProviderExecutionKey = executionTestProviderKey(t, row.OrganizationID, row.IntentKey)
				row.FenceEpoch++
				tc.mutate(&row)

				err = db.Table(attemptTable).Create(&row).Error
				require.Error(t, err, "invalid state row must be rejected by PostgreSQL, not only by read-back validation")
				var postgresErr *pgconn.PgError
				require.ErrorAs(t, err, &postgresErr)
				require.Equal(t, "23514", postgresErr.Code)
				require.Equal(t, "listing_submission_execution_attempts_state_shape_check", postgresErr.ConstraintName)
				var count int64
				require.NoError(t, db.Table(attemptTable).Where("organization_id = ? AND intent_key = ?", "org-a", tc.intent).Count(&count).Error)
				require.Zero(t, count)
			})
		}
	})

	t.Run("database identity values match persisted domain validation", func(t *testing.T) {
		const (
			attemptIdentityConstraint = "listing_submission_execution_attempts_identity_check"
			attemptIDConstraint       = "listing_submission_execution_attempts_id_v7_check"
			targetIdentityConstraint  = "listing_submission_target_fences_identity_check"
		)
		attemptCases := []struct {
			name       string
			constraint string
			mutate     func(*executionAttemptRow)
		}{
			{name: "attempt organization internal space", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.OrganizationID = "bad org" }},
			{name: "attempt intent internal space", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.IntentKey = "bad key" }},
			{name: "attempt platform internal space", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.Platform = "bad platform" }},
			{name: "attempt platform uppercase", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.Platform = "SHEIN" }},
			{name: "attempt store internal space", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.StoreID = "bad store" }},
			{name: "attempt subject internal space", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.SubjectID = "bad subject" }},
			{name: "attempt action internal space", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.Action = "bad action" }},
			{name: "attempt action uppercase", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.Action = "SAVE_DRAFT" }},
			{name: "attempt claim owner invalid prefix", constraint: attemptIdentityConstraint, mutate: func(row *executionAttemptRow) { row.ClaimOwnerID = "-worker" }},
			{name: "attempt UUIDv7 non-RFC4122 variant", constraint: attemptIDConstraint, mutate: func(row *executionAttemptRow) { row.AttemptID = "01890f5e-7b3d-7cc0-78a1-123456789abc" }},
			{name: "non derived provider key", constraint: "listing_submission_execution_attempts_provider_key_check", mutate: func(row *executionAttemptRow) { row.ProviderExecutionKey = "subk1_v1_" + strings.Repeat("e", 64) }},
			{name: "provider key derived for another organization", constraint: "listing_submission_execution_attempts_provider_key_check", mutate: func(row *executionAttemptRow) {
				row.ProviderExecutionKey = executionTestProviderKey(t, "org-b", row.IntentKey)
			}},
			{name: "provider key derived for another intent", constraint: "listing_submission_execution_attempts_provider_key_check", mutate: func(row *executionAttemptRow) {
				row.ProviderExecutionKey = executionTestProviderKey(t, row.OrganizationID, "another-intent")
			}},
		}
		for _, tc := range attemptCases {
			t.Run(tc.name, func(t *testing.T) {
				resetExecutionTables(t, db)
				now := time.Date(2026, 9, 10, 2, 58, 0, 0, time.UTC)
				kernel := executionKernel(t, db, func() time.Time { return now })
				acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "identity-source", "identity-source", `{"title":"one"}`))
				require.NoError(t, err)
				row := attemptRowFrom(acquired.Attempt, submission.ExecutionClaimTokenHash(acquired.Permit.ClaimToken))
				row.AttemptID = "01890f5e-7b3d-7cc0-98a1-123456789abc"
				row.IntentKey = "identity-candidate"
				row.SubjectID = "identity-candidate"
				row.ProviderExecutionKey = executionTestProviderKey(t, row.OrganizationID, row.IntentKey)
				row.FenceEpoch++
				tc.mutate(&row)

				err = db.Table(attemptTable).Create(&row).Error
				require.Error(t, err, "PostgreSQL must reject identity values rejected by the domain")
				var postgresErr *pgconn.PgError
				require.ErrorAs(t, err, &postgresErr)
				require.Equal(t, "23514", postgresErr.Code)
				require.Equal(t, tc.constraint, postgresErr.ConstraintName)
			})
		}

		targetCases := []struct {
			name   string
			mutate func(*targetFenceRow)
		}{
			{name: "target organization internal space", mutate: func(row *targetFenceRow) { row.OrganizationID = "bad org" }},
			{name: "target platform internal space", mutate: func(row *targetFenceRow) { row.Platform = "bad platform" }},
			{name: "target platform uppercase", mutate: func(row *targetFenceRow) { row.Platform = "SHEIN" }},
			{name: "target store internal space", mutate: func(row *targetFenceRow) { row.StoreID = "bad store" }},
			{name: "target subject internal space", mutate: func(row *targetFenceRow) { row.SubjectID = "bad subject" }},
		}
		for _, tc := range targetCases {
			t.Run(tc.name, func(t *testing.T) {
				resetExecutionTables(t, db)
				now := time.Date(2026, 9, 10, 2, 58, 0, 0, time.UTC)
				kernel := executionKernel(t, db, func() time.Time { return now })
				acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "target-identity-source", "target-identity-source", `{"title":"one"}`))
				require.NoError(t, err)
				row := targetFenceRow{
					OrganizationID: acquired.Attempt.OrganizationID, Platform: "shein", StoreID: "store-other", SubjectID: "subject-other",
					Epoch: acquired.Attempt.FenceEpoch, CurrentAttemptID: acquired.Attempt.AttemptID,
					CurrentStatus: string(acquired.Attempt.Status), UpdatedAt: acquired.Attempt.UpdatedAt,
				}
				tc.mutate(&row)

				err = db.Table(targetTable).Create(&row).Error
				require.Error(t, err, "PostgreSQL must reject target identities rejected by the domain")
				var postgresErr *pgconn.PgError
				require.ErrorAs(t, err, &postgresErr)
				require.Equal(t, "23514", postgresErr.Code)
				require.Equal(t, targetIdentityConstraint, postgresErr.ConstraintName)
			})
		}

		t.Run("ASCII boundary identifiers remain accepted", func(t *testing.T) {
			resetExecutionTables(t, db)
			now := time.Date(2026, 9, 10, 2, 58, 0, 0, time.UTC)
			boundary := strings.Repeat("a", 128)
			reservation, err := submission.NewExecutionReservation(submission.AcquireExecutionCommand{
				Scope: submission.ExecutionScope{OrganizationID: boundary}, IntentKey: boundary,
				Target: submission.ExecutionTarget{Platform: boundary, StoreID: boundary, SubjectID: boundary},
				Action: boundary, Payload: []byte(`{"title":"one"}`), ClaimOwnerID: boundary, Lease: time.Minute,
			}, "01890f5e-7b3d-7cc0-98a1-123456789abc", "boundary-claim-token", now)
			require.NoError(t, err)
			reservation.Attempt.FenceEpoch = 1
			attempt := attemptRowFrom(reservation.Attempt, reservation.ClaimTokenHash)
			require.NoError(t, db.Table(attemptTable).Create(&attempt).Error)
			fence := targetFenceRow{
				OrganizationID: reservation.Attempt.OrganizationID, Platform: reservation.Attempt.Target.Platform,
				StoreID: reservation.Attempt.Target.StoreID, SubjectID: reservation.Attempt.Target.SubjectID,
				Epoch: 1, CurrentAttemptID: reservation.Attempt.AttemptID,
				CurrentStatus: string(reservation.Attempt.Status), UpdatedAt: reservation.Attempt.UpdatedAt,
			}
			require.NoError(t, db.Table(targetTable).Create(&fence).Error)
		})
	})

	t.Run("domain derived provider key vectors are accepted", func(t *testing.T) {
		keys := make(map[string]bool)
		for _, pair := range [][2]string{{"a", "b"}, {"ab", "c"}, {"a", "bc"}, {"b", "b"}, {"Org.A_:-", "Intent.B_:-"}, {strings.Repeat("A", 128), strings.Repeat("b", 128)}} {
			t.Run(pair[0]+"/"+pair[1], func(t *testing.T) {
				resetExecutionTables(t, db)
				now := time.Date(2026, 9, 10, 5, 0, 0, 0, time.UTC)
				kernel := executionKernel(t, db, func() time.Time { return now })
				acquired, err := kernel.Acquire(ctx, executionCommand(pair[0], pair[1], "key-vector", `{"title":"one"}`))
				require.NoError(t, err)
				require.NotNil(t, acquired.Permit)
				require.False(t, keys[acquired.Attempt.ProviderExecutionKey], "length-prefixed organization/intent tuples must not alias")
				keys[acquired.Attempt.ProviderExecutionKey] = true
				persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: pair[0]}, acquired.Attempt.AttemptID)
				require.NoError(t, err)
				require.Equal(t, acquired.Attempt, persisted)
			})
		}
	})

	t.Run("database evidence values match persisted domain validation", func(t *testing.T) {
		const (
			definitiveReasonConstraint = "listing_submission_execution_attempts_definitive_reason_check"
			evidenceConstraint         = "listing_submission_execution_attempts_evidence_check"
			evidenceValueConstraint    = "listing_submission_execution_attempts_evidence_value_check"
		)
		require.Empty(t, strings.TrimSpace(executionEvidenceTrimSpaceCharacters))
		tests := []struct {
			name       string
			constraint string
			mutate     func(*executionAttemptRow)
		}{
			{
				name: "provider response definitive failure with empty reason", constraint: definitiveReasonConstraint,
				mutate: func(row *executionAttemptRow) {
					empty, failed := "", string(submission.ExecutionFailedDefinitive)
					row.Status, row.EvidenceOutcome, row.EvidenceReason = failed, &failed, &empty
				},
			},
			{
				name: "provider readback definitive failure with Unicode whitespace reason", constraint: definitiveReasonConstraint,
				mutate: func(row *executionAttemptRow) {
					kind, failed, whitespace := string(submission.EvidenceProviderReadBack), string(submission.ExecutionFailedDefinitive), executionEvidenceTrimSpaceCharacters
					row.Status, row.EvidenceKind, row.EvidenceOutcome, row.EvidenceReason = failed, &kind, &failed, &whitespace
				},
			},
			{
				name: "manual success with whitespace reason", constraint: definitiveReasonConstraint,
				mutate: func(row *executionAttemptRow) {
					kind, authorizedBy, whitespace := string(submission.EvidenceManualResolution), "operator-a", "\u1680\u2007\u202f"
					row.EvidenceKind, row.EvidenceAuthorizedBy, row.EvidenceReason = &kind, &authorizedBy, &whitespace
				},
			},
			{
				name: "manual cancellation with empty reason", constraint: definitiveReasonConstraint,
				mutate: func(row *executionAttemptRow) {
					kind, cancelled, authorizedBy, empty := string(submission.EvidenceManualResolution), string(submission.ExecutionCancelled), "operator-a", ""
					row.Status, row.EvidenceKind, row.EvidenceOutcome = cancelled, &kind, &cancelled
					row.EvidenceAuthorizedBy, row.EvidenceReason = &authorizedBy, &empty
				},
			},
			{
				name: "blank evidence reference", constraint: evidenceValueConstraint,
				mutate: func(row *executionAttemptRow) { empty := ""; row.EvidenceReference = &empty },
			},
			{
				name: "invalid evidence reference", constraint: evidenceValueConstraint,
				mutate: func(row *executionAttemptRow) { invalid := "bad reference"; row.EvidenceReference = &invalid },
			},
			{
				name: "blank manual authorizer", constraint: evidenceConstraint,
				mutate: func(row *executionAttemptRow) {
					kind, authorizedBy, reason := string(submission.EvidenceManualResolution), "", "verified"
					row.EvidenceKind, row.EvidenceAuthorizedBy, row.EvidenceReason = &kind, &authorizedBy, &reason
				},
			},
			{
				name: "invalid manual authorizer", constraint: evidenceConstraint,
				mutate: func(row *executionAttemptRow) {
					kind, authorizedBy, reason := string(submission.EvidenceManualResolution), "-operator", "verified"
					row.EvidenceKind, row.EvidenceAuthorizedBy, row.EvidenceReason = &kind, &authorizedBy, &reason
				},
			},
			{
				name: "evidence observed after finalization", constraint: evidenceValueConstraint,
				mutate: func(row *executionAttemptRow) {
					observedAt := row.FinishedAt.Add(time.Microsecond)
					row.EvidenceObservedAt = &observedAt
				},
			},
			{
				name: "evidence observed before attempt creation", constraint: evidenceValueConstraint,
				mutate: func(row *executionAttemptRow) {
					observedAt := row.CreatedAt.Add(-time.Microsecond)
					row.EvidenceObservedAt = &observedAt
				},
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				resetExecutionTables(t, db)
				now := time.Date(2026, 9, 10, 2, 59, 0, 0, time.UTC)
				kernel := executionKernel(t, db, func() time.Time { return now })
				acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-source", "listing-source", `{"title":"one"}`))
				require.NoError(t, err)

				row := terminalExecutionRow(t, acquired, "01890f5e-7b3d-7cc0-98a1-123456789abf", "intent-invalid-evidence", "listing-invalid-evidence")
				tc.mutate(&row)
				err = db.Table(attemptTable).Create(&row).Error
				require.Error(t, err, "invalid evidence row must be rejected by PostgreSQL, not only by read-back validation")
				var postgresErr *pgconn.PgError
				require.ErrorAs(t, err, &postgresErr)
				require.Equal(t, "23514", postgresErr.Code)
				require.Equal(t, tc.constraint, postgresErr.ConstraintName)
			})
		}

		positiveReasons := []struct {
			name   string
			mutate func(*executionAttemptRow)
			reason string
		}{
			{
				name: "provider success keeps optional whitespace reason", reason: executionEvidenceTrimSpaceCharacters,
				mutate: func(row *executionAttemptRow) {
					reason := executionEvidenceTrimSpaceCharacters
					row.EvidenceReason = &reason
				},
			},
			{
				name: "manual success keeps surrounding Unicode whitespace", reason: "\u00a0verified\u3000",
				mutate: func(row *executionAttemptRow) {
					kind, authorizedBy, reason := string(submission.EvidenceManualResolution), "operator-a", "\u00a0verified\u3000"
					row.EvidenceKind, row.EvidenceAuthorizedBy, row.EvidenceReason = &kind, &authorizedBy, &reason
				},
			},
		}
		for _, tc := range positiveReasons {
			t.Run(tc.name, func(t *testing.T) {
				resetExecutionTables(t, db)
				now := time.Date(2026, 9, 10, 2, 59, 0, 0, time.UTC)
				kernel := executionKernel(t, db, func() time.Time { return now })
				acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "intent-source", "listing-source", `{"title":"one"}`))
				require.NoError(t, err)
				row := terminalExecutionRow(t, acquired, acquired.Attempt.AttemptID, acquired.Attempt.IntentKey, acquired.Attempt.Target.SubjectID)
				tc.mutate(&row)
				require.NoError(t, db.Table(attemptTable).
					Where("organization_id = ? AND attempt_id = ?", acquired.Attempt.OrganizationID, acquired.Attempt.AttemptID).
					Updates(row.mutableValues()).Error)
				repository, err := NewRepository(db)
				require.NoError(t, err)
				persisted, err := repository.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, row.AttemptID)
				require.NoError(t, err)
				require.Equal(t, tc.reason, persisted.Evidence.Reason)
			})
		}
	})

	t.Run("database time values match readable transition facts", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 5, 0, 0, 0, time.UTC)
		kernel := executionKernel(t, db, func() time.Time { return now })
		acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "time-values", "time-values", `{"title":"one"}`))
		require.NoError(t, err)
		_, err = kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), providerEvidence(submission.ExecutionSucceeded, "time-values", now))
		require.NoError(t, err)
		for _, column := range []string{"created_at", "updated_at", "lease_expires_at", "evidence_observed_at", "finished_at"} {
			for _, value := range []string{"'infinity'::timestamptz", "'-infinity'::timestamptz", "make_timestamptz(1,1,1,0,0,0,'UTC')"} {
				t.Run(column+"/"+value, func(t *testing.T) {
					tx := db.Begin()
					defer func() { _ = tx.Rollback().Error }()
					err := tx.Exec("UPDATE " + attemptTable + " SET " + column + " = " + value).Error
					require.Error(t, err)
					var pgErr *pgconn.PgError
					require.ErrorAs(t, err, &pgErr)
					require.Equal(t, "23514", pgErr.Code)
				})
			}
		}
		for _, value := range []string{"'infinity'::timestamptz", "'-infinity'::timestamptz", "make_timestamptz(1,1,1,0,0,0,'UTC')"} {
			t.Run("fence/"+value, func(t *testing.T) {
				tx := db.Begin()
				defer func() { _ = tx.Rollback().Error }()
				err := tx.Exec("UPDATE " + targetTable + " SET updated_at = " + value).Error
				require.Error(t, err)
				var pgErr *pgconn.PgError
				require.ErrorAs(t, err, &pgErr)
				require.Equal(t, "23514", pgErr.Code)
			})
		}
		for _, kind := range []submission.EvidenceKind{submission.EvidenceProviderResponse, submission.EvidenceProviderReadBack, submission.EvidenceManualResolution} {
			t.Run(string(kind), func(t *testing.T) {
				tx := db.Begin()
				defer func() { _ = tx.Rollback().Error }()
				if kind == submission.EvidenceManualResolution {
					require.NoError(t, tx.Exec("UPDATE "+attemptTable+" SET evidence_kind = ?, evidence_reason = 'verified', evidence_authorized_by = 'operator-a'", kind).Error)
				} else {
					require.NoError(t, tx.Exec("UPDATE "+attemptTable+" SET evidence_kind = ?", kind).Error)
				}
				err := tx.Exec("UPDATE " + attemptTable + " SET updated_at = lease_expires_at, finished_at = lease_expires_at").Error
				if kind == submission.EvidenceProviderResponse {
					require.Error(t, err)
				} else {
					require.NoError(t, err, "qualified resolution may happen after the lease")
				}
			})
		}
		for _, offset := range []string{"'-1 microsecond'", "'1 microsecond'"} {
			t.Run("terminal update differs "+offset, func(t *testing.T) {
				tx := db.Begin()
				defer func() { _ = tx.Rollback().Error }()
				err := tx.Exec("UPDATE " + attemptTable + " SET updated_at = finished_at + interval " + offset).Error
				require.Error(t, err)
			})
		}
	})

	t.Run("provider authorizer metadata is rejected before PostgreSQL mutation", func(t *testing.T) {
		tests := []struct {
			name    string
			kind    submission.EvidenceKind
			outcome submission.ExecutionStatus
		}{
			{name: "response success", kind: submission.EvidenceProviderResponse, outcome: submission.ExecutionSucceeded},
			{name: "response definitive failure", kind: submission.EvidenceProviderResponse, outcome: submission.ExecutionFailedDefinitive},
			{name: "readback success", kind: submission.EvidenceProviderReadBack, outcome: submission.ExecutionSucceeded},
			{name: "readback definitive failure", kind: submission.EvidenceProviderReadBack, outcome: submission.ExecutionFailedDefinitive},
		}
		authorizers := []struct {
			name  string
			value string
		}{
			{name: "nonempty", value: "caller-claimed-operator"},
			{name: "whitespace", value: " \t\n"},
		}
		for index, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				for authorizerIndex, authorizer := range authorizers {
					t.Run(authorizer.name, func(t *testing.T) {
						resetExecutionTables(t, db)
						caseIndex := index*len(authorizers) + authorizerIndex
						now := time.Date(2026, 9, 10, 2, 59, caseIndex, 0, time.UTC)
						kernel := executionKernel(t, db, func() time.Time { return now })
						command := executionCommand("org-a", "provider-authorizer-"+string(rune('a'+caseIndex)), "provider-authorizer-target", `{"title":"one"}`)
						acquired, err := kernel.Acquire(ctx, command)
						require.NoError(t, err)
						if tc.kind == submission.EvidenceProviderReadBack {
							_, err = kernel.MarkUnknown(ctx, permitClaim("org-a", acquired.Permit), submission.UnknownResponseLost)
							require.NoError(t, err)
						}
						evidence := providerEvidence(tc.outcome, "provider-authorizer", now)
						evidence.Kind = tc.kind
						evidence.AuthorizedBy = authorizer.value
						if tc.kind == submission.EvidenceProviderResponse {
							_, err = kernel.Complete(ctx, permitClaim("org-a", acquired.Permit), evidence)
						} else {
							_, err = kernel.ResolveUnknown(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID, acquired.Attempt.FenceEpoch, evidence)
						}
						require.ErrorIs(t, err, submission.ErrExecutionEvidenceRequired)

						persisted, err := kernel.Get(ctx, submission.ExecutionScope{OrganizationID: "org-a"}, acquired.Attempt.AttemptID)
						require.NoError(t, err)
						if tc.kind == submission.EvidenceProviderResponse {
							require.Equal(t, submission.ExecutionClaimed, persisted.Status)
						} else {
							require.Equal(t, submission.ExecutionOutcomeUnknown, persisted.Status)
						}
						require.Nil(t, persisted.Evidence)
						replay, err := kernel.Acquire(ctx, command)
						require.NoError(t, err)
						require.True(t, replay.Replayed)
						require.Nil(t, replay.Permit)
						_, err = kernel.Acquire(ctx, executionCommand("org-a", "blocked-"+string(rune('a'+caseIndex)), "provider-authorizer-target", `{"title":"two"}`))
						require.ErrorIs(t, err, submission.ErrExecutionTargetClaimed)
					})
				}
			})
		}
	})

	t.Run("write interception after admission rolls back without a permit", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 59, 30, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		require.NoError(t, db.Exec(`
CREATE FUNCTION public.skip_submission_fence_insert() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RETURN NULL;
END
$$`).Error)
		require.NoError(t, db.Exec("CREATE TRIGGER skip_submission_fence_insert BEFORE INSERT ON public."+TargetFenceTable+" FOR EACH ROW EXECUTE FUNCTION public.skip_submission_fence_insert()").Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS skip_submission_fence_insert ON public." + TargetFenceTable).Error
			_ = db.Exec("DROP FUNCTION IF EXISTS public.skip_submission_fence_insert()").Error
		})
		kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return now }))
		require.NoError(t, err)

		for _, intent := range []string{"trigger-skipped-fence-a", "trigger-skipped-fence-b"} {
			acquired, err := kernel.Acquire(ctx, executionCommand("org-a", intent, "trigger-skipped-fence", `{"title":"one"}`))
			require.ErrorIs(t, err, submission.ErrExecutionUnavailable)
			require.Nil(t, acquired.Permit)
		}
		var count int64
		require.NoError(t, db.Table(attemptTable).Count(&count).Error)
		require.Zero(t, count)
		require.NoError(t, db.Table(targetTable).Count(&count).Error)
		require.Zero(t, count)
	})

	t.Run("attempt insert interception after admission leaves no half state", func(t *testing.T) {
		resetExecutionTables(t, db)
		now := time.Date(2026, 9, 10, 2, 59, 31, 0, time.UTC)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		require.NoError(t, db.Exec(`
CREATE FUNCTION public.skip_submission_attempt_insert() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RETURN NULL;
END
$$`).Error)
		require.NoError(t, db.Exec("CREATE TRIGGER skip_submission_attempt_insert BEFORE INSERT ON public."+AttemptTable+" FOR EACH ROW EXECUTE FUNCTION public.skip_submission_attempt_insert()").Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS skip_submission_attempt_insert ON public." + AttemptTable).Error
			_ = db.Exec("DROP FUNCTION IF EXISTS public.skip_submission_attempt_insert()").Error
		})
		kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(func() time.Time { return now }))
		require.NoError(t, err)

		acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "trigger-skipped-attempt", "trigger-skipped-attempt", `{"title":"one"}`))
		require.ErrorIs(t, err, submission.ErrExecutionUnavailable)
		require.Nil(t, acquired.Permit)
		var count int64
		require.NoError(t, db.Table(attemptTable).Count(&count).Error)
		require.Zero(t, count)
		require.NoError(t, db.Table(targetTable).Count(&count).Error)
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
		repository.now = func() time.Time { return now }
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
			name:       "missing attempt finite time constraint",
			statements: []string{"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_finite_time_check"},
		},
		{
			name:       "missing terminal time constraint",
			statements: []string{"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_terminal_time_check"},
		},
		{
			name:       "missing target finite time constraint",
			statements: []string{"ALTER TABLE public." + TargetFenceTable + " DROP CONSTRAINT listing_submission_target_fences_time_check"},
		},
		{
			name: "provider key constraint permits non derived keys",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_provider_key_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_provider_key_check CHECK (provider_execution_key ~ '^subk1_v1_[0-9a-f]{64}$')",
			},
		},
		{
			name: "evidence constraint permits observation after finalization",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_evidence_value_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_evidence_value_check CHECK ((evidence_kind IS NULL OR (evidence_reference ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$' AND evidence_observed_at >= created_at)) IS TRUE)",
			},
		},
		{
			name:       "attempt standalone unique index",
			statements: []string{"CREATE UNIQUE INDEX unexpected_attempt_unique ON public." + AttemptTable + " (intent_key)"},
		},
		{
			name:       "target standalone partial unique index",
			statements: []string{"CREATE UNIQUE INDEX unexpected_target_unique ON public." + TargetFenceTable + " (current_status) WHERE current_status = 'claimed'"},
		},
		{
			name: "attempt table has an unexpected user trigger",
			statements: []string{
				"CREATE OR REPLACE FUNCTION public.unexpected_submission_trigger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$",
				"CREATE TRIGGER unexpected_submission_attempt_trigger BEFORE INSERT ON public." + AttemptTable + " FOR EACH ROW EXECUTE FUNCTION public.unexpected_submission_trigger()",
			},
		},
		{
			name: "target table has an unexpected user trigger",
			statements: []string{
				"CREATE OR REPLACE FUNCTION public.unexpected_submission_trigger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$",
				"CREATE TRIGGER unexpected_submission_target_trigger BEFORE INSERT ON public." + TargetFenceTable + " FOR EACH ROW EXECUTE FUNCTION public.unexpected_submission_trigger()",
			},
		},
		{
			name: "attempt identity constraint keeps only the old trim and length checks",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_identity_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_identity_check CHECK (octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id) AND octet_length(intent_key) BETWEEN 1 AND 128 AND intent_key = btrim(intent_key) AND octet_length(platform) BETWEEN 1 AND 128 AND platform = btrim(platform) AND octet_length(store_id) BETWEEN 1 AND 128 AND store_id = btrim(store_id) AND octet_length(subject_id) BETWEEN 1 AND 128 AND subject_id = btrim(subject_id) AND octet_length(action) BETWEEN 1 AND 128 AND action = btrim(action) AND octet_length(claim_owner_id) BETWEEN 1 AND 128 AND claim_owner_id = btrim(claim_owner_id))",
			},
		},
		{
			name: "target identity constraint keeps only the old trim and length checks",
			statements: []string{
				"ALTER TABLE public." + TargetFenceTable + " DROP CONSTRAINT listing_submission_target_fences_identity_check",
				"ALTER TABLE public." + TargetFenceTable + " ADD CONSTRAINT listing_submission_target_fences_identity_check CHECK (octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id) AND octet_length(platform) BETWEEN 1 AND 128 AND platform = btrim(platform) AND octet_length(store_id) BETWEEN 1 AND 128 AND store_id = btrim(store_id) AND octet_length(subject_id) BETWEEN 1 AND 128 AND subject_id = btrim(subject_id))",
			},
		},
		{
			name: "attempt id constraint checks version but not RFC4122 variant",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_id_v7_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_id_v7_check CHECK (substring(attempt_id::text FROM 15 FOR 1) = '7')",
			},
		},
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
			name:       "target epoch becomes an identity column",
			statements: []string{"ALTER TABLE public." + TargetFenceTable + " ALTER COLUMN epoch ADD GENERATED ALWAYS AS IDENTITY"},
		},
		{
			name:       "target fence table is unlogged",
			statements: []string{"ALTER TABLE public." + TargetFenceTable + " SET UNLOGGED"},
		},
		{
			name: "both durable tables are unlogged",
			statements: []string{
				"ALTER TABLE public." + TargetFenceTable + " SET UNLOGGED",
				"ALTER TABLE public." + AttemptTable + " SET UNLOGGED",
			},
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
			name: "same-name state constraint accepts null required fields",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_state_shape_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_state_shape_check CHECK ((status = 'claimed' AND unknown_reason IS NULL AND evidence_kind IS NULL AND finished_at IS NULL) OR (status = 'outcome_unknown' AND unknown_reason IN ('response_lost', 'lease_expired', 'execution_cancelled') AND evidence_kind IS NULL AND finished_at IS NULL) OR (status IN ('succeeded', 'failed_definitive', 'cancelled') AND unknown_reason IS NULL AND evidence_kind IS NOT NULL AND evidence_outcome = status AND evidence_reference IS NOT NULL AND evidence_fingerprint IS NOT NULL AND evidence_observed_at IS NOT NULL AND finished_at IS NOT NULL))",
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
			name: "same-name evidence constraint accepts invalid manual authorizer",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_evidence_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_evidence_check CHECK (evidence_kind IS NULL OR (evidence_kind = 'provider_response' AND evidence_outcome IN ('succeeded', 'failed_definitive') AND evidence_authorized_by IS NULL) OR (evidence_kind = 'provider_readback' AND evidence_outcome IN ('succeeded', 'failed_definitive') AND evidence_authorized_by IS NULL) OR (evidence_kind = 'manual_resolution' AND evidence_outcome IN ('succeeded', 'failed_definitive', 'cancelled') AND evidence_authorized_by IS NOT NULL AND evidence_reason IS NOT NULL))",
			},
		},
		{
			name: "missing evidence value constraint",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_evidence_value_check",
			},
		},
		{
			name: "same-name definitive reason constraint accepts blank reason",
			statements: []string{
				"ALTER TABLE public." + AttemptTable + " DROP CONSTRAINT listing_submission_execution_attempts_definitive_reason_check",
				"ALTER TABLE public." + AttemptTable + " ADD CONSTRAINT listing_submission_execution_attempts_definitive_reason_check CHECK (evidence_outcome IS NULL OR evidence_outcome NOT IN ('failed_definitive', 'cancelled') OR evidence_reason IS NOT NULL)",
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

	t.Run("schema time contracts are independent of session timezone", func(t *testing.T) {
		reinstallExecutionSchema(t, db)
		for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York"} {
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("SELECT set_config('TimeZone', ?, true)", zone).Error; err != nil {
					return err
				}
				if err := VerifySchema(ctx, tx); err != nil {
					return err
				}
				var actual string
				if err := tx.Raw("SHOW TimeZone").Scan(&actual).Error; err != nil {
					return err
				}
				require.Equal(t, zone, actual, "verification must not change the caller timezone")
				return nil
			}))
		}
	})

	t.Run("non unique operational indexes are admitted", func(t *testing.T) {
		reinstallExecutionSchema(t, db)
		require.NoError(t, db.Exec("CREATE INDEX operational_attempt_status ON public."+AttemptTable+" (status)").Error)
		require.NoError(t, db.Exec("CREATE INDEX operational_target_time ON public."+TargetFenceTable+" (updated_at)").Error)
		repository, err := NewRepository(db)
		require.NoError(t, err)
		require.NotNil(t, repository)
		kernel, err := submission.NewExecutionKernel(repository)
		require.NoError(t, err)
		attemptIDs := make(map[string]bool)
		for _, organizationID := range []string{"org-a", "org-b"} {
			command := executionCommand(organizationID, "same-intent", "same-target", `{"title":"one"}`)
			acquired, err := kernel.Acquire(ctx, command)
			require.NoError(t, err)
			require.NotNil(t, acquired.Permit)
			require.Equal(t, organizationID, acquired.Attempt.OrganizationID)
			attemptIDs[acquired.Attempt.AttemptID] = true
			replay, err := kernel.Acquire(ctx, command)
			require.NoError(t, err)
			require.True(t, replay.Replayed)
			require.Nil(t, replay.Permit)
			require.Equal(t, acquired.Attempt, replay.Attempt)
			var fence targetFenceRow
			require.NoError(t, db.Table(targetTable).Where("organization_id = ?", organizationID).Take(&fence).Error)
			require.Equal(t, acquired.Attempt.AttemptID, fence.CurrentAttemptID)
			require.Equal(t, acquired.Permit.FenceEpoch, fence.Epoch)
			require.Equal(t, string(submission.ExecutionClaimed), fence.CurrentStatus)
		}
		require.Len(t, attemptIDs, 2)
	})
}

func TestRepositoryPostgresExecutionInheritance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)
	t.Run("descendant duplicate bypasses an active fence", func(t *testing.T) {
		reinstallExecutionSchema(t, db)
		now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
		// Pre-DDL instance is solely a diagnostic of why admission must reject
		// inheritance. Runtime DDL monitoring is not promised by the repository.
		kernel := executionKernel(t, db, func() time.Time { return now })
		first, err := kernel.Acquire(ctx, executionCommand("org-a", "first", "target", `{"title":"one"}`))
		require.NoError(t, err)
		_, err = kernel.Complete(ctx, permitClaim("org-a", first.Permit), providerEvidence(submission.ExecutionSucceeded, "first-result", now))
		require.NoError(t, err)
		var stale targetFenceRow
		require.NoError(t, db.Table(targetTable).Take(&stale).Error)
		second, err := kernel.Acquire(ctx, executionCommand("org-a", "second", "target", `{"title":"one"}`))
		require.NoError(t, err)
		require.NotNil(t, second.Permit)
		var active targetFenceRow
		require.NoError(t, db.Table(targetTable).Take(&active).Error)
		probe, err := NewRepository(db)
		require.NoError(t, err)
		require.NoError(t, db.Exec("CREATE TABLE public.execution_fence_child () INHERITS (public."+TargetFenceTable+")").Error)
		require.NoError(t, db.Table("public.execution_fence_child").Create(&active).Error)
		require.NoError(t, db.Exec("UPDATE ONLY public."+TargetFenceTable+" SET epoch = ?, current_attempt_id = ?, current_status = ?, updated_at = ?", stale.Epoch, stale.CurrentAttemptID, stale.CurrentStatus, stale.UpdatedAt).Error)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			selected, found, selectErr := probe.findTarget(ctx, tx, "org-a", first.Attempt.Target, true)
			require.NoError(t, selectErr)
			require.True(t, found)
			require.Equal(t, stale, selected, "the actual production Take selects the stale parent fence")
			var source string
			require.NoError(t, tx.Table(targetTable).Select("tableoid::regclass::text").Where("current_attempt_id = ?", selected.CurrentAttemptID).Scan(&source).Error)
			require.Equal(t, TargetFenceTable, source)
			return nil
		}))
		third, err := kernel.Acquire(ctx, executionCommand("org-a", "third", "target", `{"title":"one"}`))
		require.NoError(t, err)
		require.NotNil(t, third.Permit)
		var claimed int64
		require.NoError(t, db.Table(targetTable).Where("current_status = ?", "claimed").Count(&claimed).Error)
		require.EqualValues(t, 2, claimed, "parent and descendant now contain competing active fences")
		require.Equal(t, second.Permit.FenceEpoch, third.Permit.FenceEpoch)
		var secondStatus string
		require.NoError(t, db.Table(attemptTable).Select("status").Where("attempt_id = ?", second.Attempt.AttemptID).Scan(&secondStatus).Error)
		require.Equal(t, "claimed", secondStatus)
		require.True(t, second.Permit.LeaseExpiresAt.After(now))
		repository, err := NewRepository(db)
		require.ErrorContains(t, err, "inheritance")
		require.Nil(t, repository)
	})
	for _, table := range []string{AttemptTable, TargetFenceTable} {
		t.Run(table, func(t *testing.T) {
			reinstallExecutionSchema(t, db)
			child := table + "_child"
			require.NoError(t, db.Exec("CREATE TABLE public."+child+" () INHERITS (public."+table+")").Error)
			repository, err := NewRepository(db)
			require.ErrorContains(t, err, "inheritance")
			require.Nil(t, repository)
			require.ErrorContains(t, VerifySchema(ctx, db), "inheritance")
			require.NoError(t, db.Exec("DROP TABLE public."+child).Error)
			repository, err = NewRepository(db)
			require.NoError(t, err, "a stale relhassubclass flag must not reject an inheritance-free table")
			require.NotNil(t, repository)
			parent := table + "_parent"
			require.NoError(t, db.Exec("CREATE TABLE public."+parent+" ()").Error)
			require.NoError(t, db.Exec("ALTER TABLE public."+table+" INHERIT public."+parent).Error)
			repository, err = NewRepository(db)
			require.ErrorContains(t, err, "inheritance")
			require.Nil(t, repository)
			require.NoError(t, db.Exec("ALTER TABLE public."+table+" NO INHERIT public."+parent).Error)
			require.NoError(t, VerifySchema(ctx, db))
		})
	}
}

func TestRepositoryPostgresExecutionDatabaseAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)
	t.Run("ordinary role loses visibility and writes without RLS policy", func(t *testing.T) {
		reinstallExecutionSchema(t, db)
		kernel := executionKernel(t, db, time.Now)
		acquired, err := kernel.Acquire(ctx, executionCommand("org-a", "rls-intent", "rls-target", `{"title":"one"}`))
		require.NoError(t, err)
		require.NoError(t, db.Exec("CREATE ROLE execution_rls_probe NOLOGIN NOSUPERUSER NOBYPASSRLS").Error)
		require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO execution_rls_probe").Error)
		require.NoError(t, db.Exec("GRANT SELECT, INSERT, UPDATE ON public."+AttemptTable+", public."+TargetFenceTable+" TO execution_rls_probe").Error)
		require.NoError(t, db.Exec("ALTER TABLE public."+AttemptTable+" ENABLE ROW LEVEL SECURITY").Error)
		require.NoError(t, db.Exec("ALTER TABLE public."+AttemptTable+" FORCE ROW LEVEL SECURITY").Error)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			require.NoError(t, tx.Exec("SET LOCAL ROLE execution_rls_probe").Error)
			var bypass bool
			require.NoError(t, tx.Raw("SELECT rolsuper OR rolbypassrls FROM pg_catalog.pg_roles WHERE rolname = current_user").Scan(&bypass).Error)
			require.False(t, bypass)
			var count int64
			require.NoError(t, tx.Table(attemptTable).Count(&count).Error)
			require.Zero(t, count, "the committed attempt is hidden by RLS")
			writeErr := tx.Transaction(func(writeTx *gorm.DB) error {
				row := attemptRowFrom(acquired.Attempt, submission.ExecutionClaimTokenHash(acquired.Permit.ClaimToken))
				return writeTx.Table(attemptTable).Create(&row).Error
			})
			var pgErr *pgconn.PgError
			require.ErrorAs(t, writeErr, &pgErr)
			require.Equal(t, "42501", pgErr.Code, "RLS rejects insertion before uniqueness is considered")
			repository, admissionErr := NewRepository(tx)
			require.ErrorContains(t, admissionErr, "row security")
			require.Nil(t, repository)
			return nil
		}))
	})
	for _, table := range []string{AttemptTable, TargetFenceTable} {
		for _, mode := range []string{"ENABLE", "FORCE"} {
			t.Run(table+"/"+mode+" RLS", func(t *testing.T) {
				reinstallExecutionSchema(t, db)
				require.NoError(t, db.Exec("ALTER TABLE public."+table+" "+mode+" ROW LEVEL SECURITY").Error)
				repository, err := NewRepository(db)
				require.ErrorContains(t, err, "row security")
				require.Nil(t, repository)
				require.ErrorContains(t, VerifySchema(ctx, db), "row security")
			})
		}
	}
	t.Run("SQL_ASCII cannot uphold UTF8 evidence", func(t *testing.T) {
		require.NoError(t, db.Exec("CREATE DATABASE execution_ascii TEMPLATE template0 ENCODING 'SQL_ASCII' LC_COLLATE 'C' LC_CTYPE 'C'").Error)
		dsn, err := url.Parse(db.Dialector.(*postgres.Dialector).DSN)
		require.NoError(t, err)
		dsn.Path = "/execution_ascii"
		asciiDB, err := gorm.Open(postgres.Open(dsn.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		sqlDB, err := asciiDB.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		// Install raw contract DDL to test admission of an already-existing schema.
		for _, statement := range schemaStatements {
			require.NoError(t, asciiDB.Exec(statement).Error)
		}
		var invalid string
		require.NoError(t, asciiDB.Raw("SELECT convert_from(decode('ff', 'hex'), 'SQL_ASCII')").Scan(&invalid).Error)
		require.False(t, utf8.ValidString(invalid), "SQL_ASCII really permits invalid UTF8 text")
		repository, err := NewRepository(asciiDB)
		require.ErrorContains(t, err, "UTF8")
		require.Nil(t, repository)
		require.ErrorContains(t, VerifySchema(ctx, asciiDB), "UTF8")
	})
}

func TestRepositoryPostgresExecutionRewriteRules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)

	t.Run("redirected insert can acknowledge a missing fence", func(t *testing.T) {
		reinstallExecutionSchema(t, db)
		// Deliberately retain a pre-DDL repository to demonstrate why admission
		// must reject this fixture. Continuous DDL monitoring is not its contract.
		kernel := executionKernel(t, db, time.Now)
		require.NoError(t, db.Exec("CREATE TABLE public.execution_rule_shadow (LIKE public."+TargetFenceTable+")").Error)
		require.NoError(t, db.Exec("CREATE RULE redirect_fence AS ON INSERT TO public."+TargetFenceTable+" DO INSTEAD INSERT INTO public.execution_rule_shadow VALUES (NEW.*)").Error)
		for _, intent := range []string{"first-intent", "second-intent"} {
			acquired, err := kernel.Acquire(ctx, executionCommand("org-a", intent, "same-target", `{"title":"one"}`))
			require.NoError(t, err)
			require.NotNil(t, acquired.Permit, "the rewritten INSERT reports one affected row")
		}
		var canonicalCount, shadowCount int64
		require.NoError(t, db.Table(targetTable).Count(&canonicalCount).Error)
		require.NoError(t, db.Table("public.execution_rule_shadow").Count(&shadowCount).Error)
		require.Zero(t, canonicalCount)
		require.EqualValues(t, 2, shadowCount)
		repository, err := NewRepository(db)
		require.ErrorContains(t, err, "rewrite rules")
		require.Nil(t, repository)
	})
	for _, table := range []string{AttemptTable, TargetFenceTable} {
		for _, event := range []string{"INSERT", "UPDATE", "DELETE"} {
			t.Run(table+"/"+event, func(t *testing.T) {
				reinstallExecutionSchema(t, db)
				require.NoError(t, db.Exec("CREATE RULE suppress_write AS ON "+event+" TO public."+table+" DO INSTEAD NOTHING").Error)
				repository, err := NewRepository(db)
				require.ErrorContains(t, err, "rewrite rules")
				require.Nil(t, repository)
				require.ErrorContains(t, VerifySchema(ctx, db), "rewrite rules")
			})
		}
	}
}

func TestRepositoryPostgresExecutionCollations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)
	require.NoError(t, db.Exec("CREATE COLLATION public.execution_case_insensitive (provider = icu, locale = 'und-u-ks-level2', deterministic = false)").Error)
	require.NoError(t, db.Exec("CREATE COLLATION public.execution_deterministic (provider = icu, locale = 'und-u-ks-level2', deterministic = true)").Error)
	var equal bool
	require.NoError(t, db.Raw("SELECT 'CLAIMED' = 'claimed' COLLATE public.execution_case_insensitive").Scan(&equal).Error)
	require.True(t, equal, "the drift fixture must actually relax byte equality")
	installWithCollation := func(t *testing.T, table, column, typeSQL, collation string) {
		t.Helper()
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS public."+TargetFenceTable+", public."+AttemptTable+" CASCADE").Error)
		for _, statement := range schemaStatements {
			if strings.HasPrefix(statement, "CREATE TABLE public."+table+" (") {
				definition := "\n    " + column + " " + typeSQL
				require.Equal(t, 1, strings.Count(statement, definition))
				statement = strings.Replace(statement, definition, definition+" COLLATE "+collation, 1)
			}
			require.NoError(t, db.Exec(statement).Error)
		}
	}

	for _, column := range []struct{ table, name, typeSQL string }{
		{AttemptTable, "status", "VARCHAR(32)"},
		{TargetFenceTable, "current_status", "VARCHAR(32)"},
		{AttemptTable, "intent_key", "VARCHAR(128)"},
		{AttemptTable, "evidence_reason", "VARCHAR(512)"},
		{AttemptTable, "claim_token_hash", "CHAR(64)"},
		{TargetFenceTable, "subject_id", "VARCHAR(128)"},
	} {
		t.Run(column.table+"/"+column.name, func(t *testing.T) {
			installWithCollation(t, column.table, column.name, column.typeSQL, "public.execution_case_insensitive")
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("SELECT set_config('search_path', 'pg_catalog', true)").Error; err != nil {
					return err
				}
				return verifyConstraints(ctx, tx, column.table, expectedConstraints[column.table])
			}), "constraint text alone cannot detect this drift")
			repository, err := NewRepository(db)
			require.ErrorContains(t, err, "deterministic collation")
			require.Nil(t, repository)
			require.ErrorContains(t, VerifySchema(ctx, db), "deterministic collation")
		})
	}
	for _, collation := range []string{"pg_catalog.\"C\"", "public.execution_deterministic"} {
		t.Run("admit/"+collation, func(t *testing.T) {
			installWithCollation(t, AttemptTable, "status", "VARCHAR(32)", collation)
			repository, err := NewRepository(db)
			require.NoError(t, err)
			require.NotNil(t, repository)
			require.NoError(t, db.Raw("SELECT 'CLAIMED' = 'claimed' COLLATE "+collation).Scan(&equal).Error)
			require.False(t, equal)
		})
	}
}

func TestVerifyColumnsRejectsGeneratedAttributes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, _ := openExecutionPostgres(t, ctx)
	const table = "listing_submission_generated_column_probe"
	require.NoError(t, db.Exec("CREATE TABLE public."+table+" (source BIGINT NOT NULL, computed BIGINT GENERATED ALWAYS AS (source + 1) STORED)").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS public." + table).Error })

	err := verifyColumns(ctx, db, table, []columnContract{
		{name: "source", typeSQL: "bigint", notNull: true},
		{name: "computed", typeSQL: "bigint"},
	})
	require.Error(t, err)
}

func executionKernel(t *testing.T, db *gorm.DB, clock func() time.Time) *submission.ExecutionKernel {
	t.Helper()
	repository, err := NewRepository(db)
	require.NoError(t, err)
	repository.now = clock
	kernel, err := submission.NewExecutionKernel(repository, submission.WithExecutionClock(clock))
	require.NoError(t, err)
	return kernel
}

func executionTestProviderKey(t *testing.T, organizationID, intentKey string) string {
	t.Helper()
	reservation, err := submission.NewExecutionReservation(executionCommand(organizationID, intentKey, "key-fixture", `{"title":"one"}`),
		"01890f5e-7b3d-7cc0-98a1-123456789abc", "fixture-token", time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return reservation.Attempt.ProviderExecutionKey
}

func terminalExecutionRow(t *testing.T, source submission.ExecutionAcquisition, attemptID, intentKey, subjectID string) executionAttemptRow {
	t.Helper()
	row := attemptRowFrom(source.Attempt, submission.ExecutionClaimTokenHash(source.Permit.ClaimToken))
	kind, outcome := string(submission.EvidenceProviderResponse), string(submission.ExecutionSucceeded)
	reference, fingerprint := "provider-reference", executionTestDigest("provider-reference")
	observedAt, finishedAt := row.UpdatedAt, row.UpdatedAt
	row.AttemptID, row.IntentKey, row.SubjectID = attemptID, intentKey, subjectID
	if attemptID != source.Attempt.AttemptID {
		row.ProviderExecutionKey = executionTestProviderKey(t, row.OrganizationID, row.IntentKey)
		row.FenceEpoch++
	}
	row.Status = outcome
	row.UnknownReason = nil
	row.EvidenceKind, row.EvidenceOutcome = &kind, &outcome
	row.EvidenceReference, row.EvidenceFingerprint = &reference, &fingerprint
	row.EvidenceReason, row.EvidenceAuthorizedBy = nil, nil
	row.EvidenceObservedAt, row.FinishedAt = &observedAt, &finishedAt
	return row
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
