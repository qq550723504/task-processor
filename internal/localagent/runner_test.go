package localagent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	alibaba1688 "task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/product/sourcing"
)

func TestRunnerSubmitsSanitizedChallenge(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{err: alibaba1688.NewPublicAccessError(alibaba1688.PublicAccessFailureChallenge, errors.New("cookie=secret"))}
	_, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, FailureChallenge, api.submittedFailure.Kind)
	require.NotContains(t, api.submittedFailure.Message, "secret")
}

func TestRunnerSubmitsSnapshotAndReportsSuccess(t *testing.T) {
	api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
	crawler := &fakeCrawler{product: &model.Product1688{ID: "1052008074197", URL: offerURL}}
	outcome, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeSucceeded, outcome.State)
	require.Equal(t, "1052008074197", api.submittedProduct.ID)
}

type fakeJobsAPI struct {
	claim            *Claim
	submittedFailure Failure
	submittedProduct *sourcing.Alibaba1688ProductSnapshot
}

func (f *fakeJobsAPI) Claim(context.Context) (*Claim, error) { return f.claim, nil }
func (f *fakeJobsAPI) SubmitSuccess(_ context.Context, _ string, _ string, product *sourcing.Alibaba1688ProductSnapshot) (Job, error) {
	f.submittedProduct = product
	return Job{State: JobSucceeded}, nil
}
func (f *fakeJobsAPI) SubmitFailure(_ context.Context, _ string, _ string, failure Failure) (Job, error) {
	f.submittedFailure = failure
	return Job{State: JobFailed}, nil
}

type fakeCrawler struct {
	product *model.Product1688
	err     error
}

func (f *fakeCrawler) Process(context.Context, string) (*model.Product1688, error) {
	return f.product, f.err
}
