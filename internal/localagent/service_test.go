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

	done, err := service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, validSnapshot())
	require.NoError(t, err)
	require.Equal(t, JobSucceeded, done.State)
	require.Equal(t, "crawler:1688:1052008074197", done.Envelope.Identity.SourceKey())
	require.NotNil(t, done.EnvelopeSummary)
	require.Equal(t, "crawler:1688:1052008074197", done.EnvelopeSummary.SourceKey)
	require.Equal(t, "1052008074197", done.EnvelopeSummary.ProductID)
	service.mu.Lock()
	require.Nil(t, service.jobs[job.ID].job.Envelope)
	service.mu.Unlock()
}

func TestServiceBoundsRetainedEnvelopeSummary(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)

	snapshot := validSnapshot()
	snapshot.Title = strings.Repeat("title", 1024)
	snapshot.Supplier.Name = strings.Repeat("supplier", 1024)
	snapshot.Supplier.CompanyName = snapshot.Supplier.Name
	done, err := service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, snapshot)
	require.NoError(t, err)
	require.LessOrEqual(t, len(done.EnvelopeSummary.Title), maxEnvelopeSummaryTitleBytes)
	require.LessOrEqual(t, len(done.EnvelopeSummary.SupplierName), maxEnvelopeSummarySupplierBytes)

	service.mu.Lock()
	retained := service.jobs[job.ID].job.EnvelopeSummary
	service.mu.Unlock()
	require.NotNil(t, retained)
	require.LessOrEqual(t, len(retained.Title), maxEnvelopeSummaryTitleBytes)
	require.LessOrEqual(t, len(retained.SupplierName), maxEnvelopeSummarySupplierBytes)
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

func TestServiceRejectsEmptySnapshotAsNonRetryable(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)

	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, nil)
	require.ErrorIs(t, err, ErrSnapshotInvalid)
	require.ErrorIs(t, err, ErrInvalidClaim)
}

func TestServiceRejectsSnapshotMissingCrawlerRequiredFacts(t *testing.T) {
	service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	claim, err := service.Claim(actor)
	require.NoError(t, err)

	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, &sourcing.Alibaba1688ProductSnapshot{
		ID: "1052008074197", Title: "shirt", URL: offerURL,
	})
	require.ErrorIs(t, err, ErrSnapshotInvalid)
}

func TestServiceRejectsMalformedOptionalAssetURLs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sourcing.Alibaba1688ProductSnapshot)
	}{
		{name: "gallery image", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Images = []string{"not-a-url"}
		}},
		{name: "hostless optional image", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Variants = []sourcing.Alibaba1688VariantSnapshot{{Image: "https:///.jpg"}}
		}},
		{name: "video URL", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Videos = []sourcing.Alibaba1688VideoSnapshot{{VideoURL: "not-a-url"}}
		}},
		{name: "video cover", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Videos = []sourcing.Alibaba1688VideoSnapshot{{CoverURL: "not-a-url"}}
		}},
		{name: "detail image", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.ProductDetails = []sourcing.Alibaba1688ProductDetailSnapshot{{Images: []string{"not-a-url"}}}
		}},
		{name: "variant image", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Variants = []sourcing.Alibaba1688VariantSnapshot{{Image: "not-a-url"}}
		}},
		{name: "package image", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.PackInfo = &sourcing.Alibaba1688PackInfoSnapshot{PackageImages: []string{"not-a-url"}}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
			actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
			job, err := service.Create(actor, offerURL)
			require.NoError(t, err)
			claim, err := service.Claim(actor)
			require.NoError(t, err)

			snapshot := validSnapshot()
			tt.mutate(snapshot)
			_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, snapshot)
			require.ErrorIs(t, err, ErrSnapshotInvalid)
		})
	}
}

func TestServiceRejectsImpossibleOptionalSupplierNumbers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sourcing.Alibaba1688ProductSnapshot)
	}{
		{name: "negative years in business", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Supplier.YearsInBusiness = -1
		}},
		{name: "supplier rating above maximum", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Supplier.Rating = 5.1
		}},
		{name: "response rate above maximum", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Supplier.ResponseRate = 100.1
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
			actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
			job, err := service.Create(actor, offerURL)
			require.NoError(t, err)
			claim, err := service.Claim(actor)
			require.NoError(t, err)

			snapshot := validSnapshot()
			tt.mutate(snapshot)
			_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, snapshot)
			require.ErrorIs(t, err, ErrSnapshotInvalid)
		})
	}
}

func TestServiceRejectsImpossibleProductNumbers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sourcing.Alibaba1688ProductSnapshot)
	}{
		{name: "negative sales volume", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.SalesVolume = -1
		}},
		{name: "negative review count", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.ReviewCount = -1
		}},
		{name: "product rating above maximum", mutate: func(snapshot *sourcing.Alibaba1688ProductSnapshot) {
			snapshot.Rating = 99
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(fixedClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)))
			actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
			job, err := service.Create(actor, offerURL)
			require.NoError(t, err)
			claim, err := service.Claim(actor)
			require.NoError(t, err)

			snapshot := validSnapshot()
			tt.mutate(snapshot)
			_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, snapshot)
			require.ErrorIs(t, err, ErrSnapshotInvalid)
		})
	}
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
	require.ErrorIs(t, err, ErrSnapshotInvalid)
	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, &sourcing.Alibaba1688ProductSnapshot{ID: "1052008074197", Title: "shirt", URL: "https://detail.1688.com/offer/999.html"})
	require.ErrorIs(t, err, ErrInvalidURL)
	require.ErrorIs(t, err, ErrSnapshotInvalid)
	_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, &sourcing.Alibaba1688ProductSnapshot{ID: "999", Title: "shirt", URL: offerURL})
	require.ErrorIs(t, err, ErrInvalidClaim)
	require.ErrorIs(t, err, ErrSnapshotInvalid)
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
	_, err = service.Create(Actor{TenantID: "tenant-a", UserID: "user-a"}, "https://detail.1688.com:/offer/1052008074197.html")
	require.ErrorIs(t, err, ErrInvalidURL)
	_, err = service.Create(Actor{TenantID: "tenant-a", UserID: "user-a"}, "https://detail.1688.com/offer/%31%30%35%32%30%30%38%30%37%34%31%39%37.html")
	require.ErrorIs(t, err, ErrInvalidURL)
	_, err = service.Create(Actor{TenantID: "tenant-a", UserID: "user-a"}, offerURL+"?")
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

func TestServiceClaimExtendsJobExpiryForFullLease(t *testing.T) {
	clock := newMutableClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	service := NewService(clock.Now)
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	job, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	clock.Advance(jobTTL - time.Minute)
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	require.Equal(t, clock.Now().Add(jobTTL), claim.Job.ExpiresAt)
	require.Greater(t, claim.Job.ExpiresAt, job.ExpiresAt)
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

func TestServiceEvictsJobAfterThreeExpiredClaims(t *testing.T) {
	clock := newMutableClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	service := NewService(clock.Now)
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	_, err := service.Create(actor, offerURL)
	require.NoError(t, err)

	for attempt := 0; attempt < 3; attempt++ {
		claim, err := service.Claim(actor)
		require.NoError(t, err)
		require.NotNil(t, claim)
		clock.Advance(claimTTL + time.Nanosecond)
	}
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	require.Nil(t, claim)
}

func fixedClock(value time.Time) func() time.Time {
	return func() time.Time { return value }
}

type mutableClock struct{ value time.Time }

func newMutableClock(value time.Time) *mutableClock { return &mutableClock{value: value} }
func (c *mutableClock) Now() time.Time              { return c.value }
func (c *mutableClock) Advance(delta time.Duration) { c.value = c.value.Add(delta) }

func validSnapshot() *sourcing.Alibaba1688ProductSnapshot {
	return &sourcing.Alibaba1688ProductSnapshot{
		ID:               "1052008074197",
		Title:            "shirt",
		URL:              offerURL,
		MainImage:        "https://img.1688.com/product.jpg",
		MinPrice:         12.5,
		MinOrderQuantity: 1,
		Supplier:         sourcing.Alibaba1688SupplierSnapshot{Name: "Acme"},
	}
}
