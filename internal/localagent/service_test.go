package localagent

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

const offerURL = "https://detail.1688.com/offer/1052008074197.html"

func TestServiceClaimIsTenantScopedAndSingleUse(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	owner := Actor{TenantID: "tenant-a", UserID: "user-a"}
	_, err := service.Create(owner, offerURL)
	require.NoError(t, err)

	wrongTenant, err := service.Claim(Actor{TenantID: "tenant-b", UserID: "user-b"})
	require.NoError(t, err)
	require.Nil(t, wrongTenant)

	claim, err := service.Claim(owner)
	require.NoError(t, err)
	require.NotEmpty(t, claim.ExecutionToken)
	again, err := service.Claim(owner)
	require.NoError(t, err)
	require.Nil(t, again)
}

func TestServiceClaimJobTargetsCreatedJob(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	first, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	second, err := service.Create(actor, "https://detail.1688.com/offer/1052008074198.html")
	require.NoError(t, err)
	claim, err := service.ClaimJob(actor, second.ID)
	require.NoError(t, err)
	require.Equal(t, second.ID, claim.Job.ID)
	claim, err = service.ClaimJob(actor, first.ID)
	require.NoError(t, err)
	require.Equal(t, first.ID, claim.Job.ID)
}

func TestServiceBuildsEnvelopeFromAcceptedSnapshot(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)

	done, err := service.SubmitSuccess(actor, job.ID, claim.ExecutionToken,
		&sourcing.Alibaba1688ProductSnapshot{ID: "1052008074197", Title: "shirt", URL: offerURL})
	require.NoError(t, err)
	require.Equal(t, JobSucceeded, done.State)
	require.Equal(t, "crawler:1688:1052008074197", done.Envelope.Identity.SourceKey())
}

func TestServiceRejectsExpiredClaim(t *testing.T) {
	clock := newMutableClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	service := NewService(clock.Now)
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	clock.Advance(claimTTL + time.Nanosecond)

	_, err = service.SubmitFailure(actor, job.ID, claim.ExecutionToken, Failure{Kind: FailureNavigation, Message: "timeout"})
	require.ErrorIs(t, err, ErrClaimExpired)
}

func TestServiceRejectsOversizedSnapshotAndKeepsClaimActive(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, &sourcing.Alibaba1688ProductSnapshot{
		ID: "1052008074197", URL: offerURL, Title: strings.Repeat("x", maxSnapshotBytes),
	})
	require.ErrorIs(t, err, ErrSnapshotTooLarge)
	second, err := service.SubmitFailure(actor, job.ID, claim.ExecutionToken, Failure{Kind: FailureExtraction, Message: "retryable"})
	require.NoError(t, err)
	require.Equal(t, JobFailed, second.State)
}

func TestServiceAcceptsOnlySnapshotForClaimedURL(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, &sourcing.Alibaba1688ProductSnapshot{ID: "1052008074197", Title: "shirt"})
	require.ErrorIs(t, err, ErrInvalidURL)
	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, &sourcing.Alibaba1688ProductSnapshot{ID: "1052008074197", Title: "shirt", URL: "https://detail.1688.com/offer/999.html"})
	require.ErrorIs(t, err, ErrInvalidURL)
	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, &sourcing.Alibaba1688ProductSnapshot{ID: "999", Title: "shirt", URL: offerURL})
	require.ErrorIs(t, err, ErrInvalidClaim)
}

func TestServiceRejectsOversizedFailureDiagnostic(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	_, err = service.SubmitFailure(actor, job.ID, claim.ExecutionToken, Failure{Kind: FailureNavigation, Message: strings.Repeat("x", maxDiagnosticBytes+1)})
	require.ErrorIs(t, err, ErrFailureInvalid)
}

func TestServiceRejectsNonPublicOfferURL(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	_, err := service.Create(Actor{TenantID: "tenant-a", UserID: "user-a"}, "https://www.1688.com/offer/1052008074197.html")
	require.ErrorIs(t, err, ErrInvalidURL)
	_, err = service.Create(Actor{TenantID: "tenant-a", UserID: "user-a"}, "https://detail.1688.com/offer/1052008074197")
	require.ErrorIs(t, err, ErrInvalidURL)
	_, err = service.Create(Actor{TenantID: "tenant-a", UserID: "user-a"}, "https://detail.1688.com/offer/1052008074197.html/extra")
	require.ErrorIs(t, err, ErrInvalidURL)
	_, err = service.Create(Actor{TenantID: "tenant-a", UserID: "user-a"}, "https://detail.1688.com:443/offer/1052008074197.html")
	require.ErrorIs(t, err, ErrInvalidURL)
}

func TestServiceCapacityIsIsolatedPerTenant(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	noisy := Actor{TenantID: "tenant-noisy", UserID: "user-a"}
	other := Actor{TenantID: "tenant-other", UserID: "user-b"}
	for i := 0; i < maxJobsPerTenant; i++ {
		_, err := service.Create(noisy, offerURL)
		require.NoError(t, err)
	}
	_, err := service.Create(noisy, offerURL)
	require.ErrorIs(t, err, ErrCapacity)
	_, err = service.Create(other, offerURL)
	require.NoError(t, err)
}

func TestClaimLeaseOutlivesManagedBrowserDownload(t *testing.T) {
	require.Greater(t, claimTTL, 10*time.Minute)
	require.Greater(t, jobTTL, claimTTL)
}

func TestServiceCleansExpiredAndRetainedTerminalJobs(t *testing.T) {
	clock := newMutableClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	service := NewService(clock.Now)
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	_, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	clock.Advance(jobTTL + time.Nanosecond)
	_, err = service.Create(actor, offerURL)
	require.NoError(t, err)
	service.mu.Lock()
	require.Len(t, service.jobs, 1)
	service.mu.Unlock()

	claim, err := service.Claim(actor)
	require.NoError(t, err)
	_, err = service.SubmitFailure(actor, claim.Job.ID, claim.ExecutionToken, Failure{Kind: FailureChallenge, Message: "challenge"})
	require.NoError(t, err)
	clock.Advance(terminalRetention + time.Nanosecond)
	_, err = service.Create(actor, offerURL)
	require.NoError(t, err)
	service.mu.Lock()
	require.Len(t, service.jobs, 1)
	service.mu.Unlock()
}

func TestServiceRejectsWrongTokenAndDuplicateTerminalResult(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	_, err = service.SubmitFailure(actor, job.ID, "wrong-token", Failure{Kind: FailureChallenge, Message: "captcha"})
	require.ErrorIs(t, err, ErrInvalidClaim)
	done, err := service.SubmitFailure(actor, job.ID, claim.ExecutionToken, Failure{Kind: FailureChallenge, Message: "captcha"})
	require.NoError(t, err)
	_, err = service.SubmitFailure(actor, job.ID, claim.ExecutionToken, Failure{Kind: FailureChallenge, Message: "again"})
	require.ErrorIs(t, err, ErrInvalidClaim)
	require.Equal(t, JobFailed, done.State)
}

func TestServiceRequeuesExpiredClaimWhileJobIsAlive(t *testing.T) {
	clock := newMutableClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	service := NewService(clock.Now)
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	_, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	first, err := service.Claim(actor)
	require.NoError(t, err)
	clock.Advance(claimTTL + time.Nanosecond)
	second, err := service.Claim(actor)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, first.ExecutionToken, second.ExecutionToken)
}

func fixedClock(value time.Time) func() time.Time {
	return func() time.Time { return value }
}

type mutableClock struct{ value time.Time }

func newMutableClock(value time.Time) *mutableClock { return &mutableClock{value: value} }
func (c *mutableClock) Now() time.Time              { return c.value }
func (c *mutableClock) Advance(delta time.Duration) { c.value = c.value.Add(delta) }
