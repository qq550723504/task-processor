package localagent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	alibaba1688 "task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/crawler/alibaba1688/model"
	sourcea1688 "task-processor/internal/integration/crawler/a1688"
)

func TestRunnerSubmitsSanitizedChallenge(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{err: alibaba1688.NewPublicAccessError(alibaba1688.PublicAccessFailureChallenge, errors.New("cookie=secret"))}
	_, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, FailureChallenge, api.submittedFailure.Kind)
	require.NotContains(t, api.submittedFailure.Message, "secret")
}

func TestRunnerClassifiesBrowserStartupFailure(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{err: alibaba1688.NewPublicAccessError(alibaba1688.PublicAccessFailureBrowser, errors.New("playwright install failed"))}
	_, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, FailureBrowser, api.submittedFailure.Kind)
}

func TestRunnerClassifiesProductValidationFailureAsExtraction(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{err: alibaba1688.NewPublicAccessError(alibaba1688.PublicAccessFailureValidation, errors.New("invalid image"))}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.Equal(t, FailureExtraction, api.submittedFailure.Kind)
}

func TestRunnerRecordsPreparationFailureAgainstPendingJob(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{prepareErr: errors.New("playwright install failed")}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.Equal(t, FailureBrowser, api.submittedFailure.Kind)
}

func TestRunnerReportsPreflightPreparationFailureWithoutRetryingPreparation(t *testing.T) {
	order := []string{}
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}, order: &order}
	crawler := &fakeCrawler{prepareErr: errors.New("playwright install failed"), order: &order}
	outcome, err := (Runner{Jobs: api, Crawler: crawler, PreparationError: errors.New("preflight failed")}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.Equal(t, FailureBrowser, api.submittedFailure.Kind)
	require.Equal(t, []string{"claim"}, order)
}

func TestRunnerSubmitsSnapshotAndReportsSuccess(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{product: &model.Product1688{ID: "1052008074197", URL: offerURL}}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeSucceeded, outcome.State)
	require.Equal(t, "1052008074197", api.submittedProduct.ID)
}

func TestRunnerReportsReconstructedEnvelopeSummary(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{product: &model.Product1688{ID: "1052008074197", URL: offerURL}}

	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.NotNil(t, outcome.EnvelopeSummary)
	require.Equal(t, "crawler:1688:1052008074197", outcome.EnvelopeSummary.SourceKey)
}

func TestRunnerTerminatesOversizedSnapshotAsExtractionFailure(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}, submitSuccessErr: ErrSnapshotTooLarge}
	crawler := &fakeCrawler{product: &model.Product1688{ID: "1052008074197", URL: offerURL}}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.Equal(t, FailureExtraction, api.submittedFailure.Kind)
}

func TestRunnerTerminatesInvalidSnapshotAsExtractionFailure(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}, submitSuccessErr: ErrSnapshotInvalid}
	crawler := &fakeCrawler{product: &model.Product1688{ID: "1052008074197", URL: offerURL}}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.Equal(t, FailureExtraction, api.submittedFailure.Kind)
}

func TestRunnerTerminatesEmptySnapshotAsExtractionFailure(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}, submitSuccessErr: ErrSnapshotInvalid}
	crawler := &fakeCrawler{}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.Equal(t, FailureExtraction, api.submittedFailure.Kind)
}

func TestRunnerTerminatesNilCrawlerResultAsExtractionFailure(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.Equal(t, FailureExtraction, api.submittedFailure.Kind)
}

func TestRunnerPreparesCrawlerBeforeClaim(t *testing.T) {
	order := []string{}
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}, order: &order}
	crawler := &fakeCrawler{product: &model.Product1688{ID: "1052008074197", URL: offerURL}, order: &order}

	_, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"prepare", "claim", "process", "success"}, order)
}

func TestRunnerSkipsPreparationWhenCrawlerWasPreparedBeforeJobCreation(t *testing.T) {
	order := []string{}
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}, order: &order}
	crawler := &fakeCrawler{product: &model.Product1688{ID: "1052008074197", URL: offerURL}, order: &order}

	_, err := (Runner{Jobs: api, Crawler: crawler, CrawlerPrepared: true}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"claim", "process", "success"}, order)
}

func TestRunnerSubmitsCancellationFailureWithLiveContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{err: context.Canceled, cancelOnProcess: cancel}

	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, OutcomeFailed, outcome.State)
	require.NoError(t, api.submittedFailureContextErr)
}

type fakeJobsAPI struct {
	claim                      *Claim
	submittedFailure           Failure
	submittedProduct           *sourcea1688.Alibaba1688ProductSnapshot
	submitSuccessErr           error
	order                      *[]string
	submittedFailureContextErr error
}

func (f *fakeJobsAPI) Claim(context.Context) (*Claim, error) {
	if f.order != nil {
		*f.order = append(*f.order, "claim")
	}
	return f.claim, nil
}
func (f *fakeJobsAPI) SubmitSuccess(_ context.Context, _ string, _ string, product *sourcea1688.Alibaba1688ProductSnapshot) (Job, error) {
	if f.order != nil {
		*f.order = append(*f.order, "success")
	}
	f.submittedProduct = product
	if f.submitSuccessErr != nil {
		return Job{}, f.submitSuccessErr
	}
	return Job{State: JobSucceeded, EnvelopeSummary: &EnvelopeSummary{SourceKey: "crawler:1688:1052008074197", ProductID: "1052008074197"}}, nil
}
func (f *fakeJobsAPI) SubmitFailure(ctx context.Context, _ string, _ string, failure Failure) (Job, error) {
	f.submittedFailureContextErr = ctx.Err()
	f.submittedFailure = failure
	return Job{State: JobFailed}, nil
}

type fakeCrawler struct {
	product         *model.Product1688
	err             error
	prepareErr      error
	order           *[]string
	cancelOnProcess context.CancelFunc
}

func (f *fakeCrawler) Prepare(context.Context) error {
	if f.order != nil {
		*f.order = append(*f.order, "prepare")
	}
	return f.prepareErr
}

func (f *fakeCrawler) Process(context.Context, string) (*model.Product1688, error) {
	if f.order != nil {
		*f.order = append(*f.order, "process")
	}
	if f.cancelOnProcess != nil {
		f.cancelOnProcess()
	}
	return f.product, f.err
}
