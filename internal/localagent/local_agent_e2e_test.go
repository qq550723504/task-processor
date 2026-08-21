package localagent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/product/sourcing"
)

func TestLocalAgentProtocolCompletesWithoutListingKitTask(t *testing.T) {
	service := NewService(fixedClockForE2E())
	actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
	created, err := service.Create(actor, offerURL)
	require.NoError(t, err)
	runner := Runner{
		Jobs:    serviceJobs{service: service, actor: actor},
		Crawler: e2eCrawler{},
	}
	outcome, err := runner.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, OutcomeSucceeded, outcome.State)
	require.Equal(t, created.ID, outcome.JobID)
	claim, err := service.Claim(actor)
	require.NoError(t, err)
	require.Nil(t, claim)
}

type serviceJobs struct {
	service *Service
	actor   Actor
}

func (j serviceJobs) Claim(context.Context) (*Claim, error) { return j.service.Claim(j.actor) }
func (j serviceJobs) SubmitSuccess(_ context.Context, id, token string, product *sourcing.Alibaba1688ProductSnapshot) (Job, error) {
	return j.service.SubmitSuccess(j.actor, id, token, product)
}
func (j serviceJobs) SubmitFailure(_ context.Context, id, token string, failure Failure) (Job, error) {
	return j.service.SubmitFailure(j.actor, id, token, failure)
}

type e2eCrawler struct{}

func (e2eCrawler) Process(context.Context, string) (*model.Product1688, error) {
	return &model.Product1688{ID: "1052008074197", Title: "shirt", URL: offerURL}, nil
}

func fixedClockForE2E() func() time.Time {
	return func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) }
}
