package localagent

import (
	"context"
	"errors"

	alibaba1688 "task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/product/sourcing"
)

type Jobs interface {
	Claim(context.Context) (*Claim, error)
	SubmitSuccess(context.Context, string, string, *sourcing.Alibaba1688ProductSnapshot) (Job, error)
	SubmitFailure(context.Context, string, string, Failure) (Job, error)
}

type targetedJobs interface {
	ClaimJob(context.Context, string) (*Claim, error)
}

type Crawler interface {
	Process(context.Context, string) (*model.Product1688, error)
}

type Runner struct {
	Jobs    Jobs
	Crawler Crawler
	JobID   string
}

type OutcomeState string

const (
	OutcomeIdle      OutcomeState = "idle"
	OutcomeSucceeded OutcomeState = "succeeded"
	OutcomeFailed    OutcomeState = "failed"
)

type Outcome struct {
	State OutcomeState
	JobID string
}

func (r Runner) RunOnce(ctx context.Context) (Outcome, error) {
	if r.Jobs == nil || r.Crawler == nil {
		return Outcome{}, errors.New("local-agent runner is not configured")
	}
	var claim *Claim
	var err error
	if r.JobID != "" {
		if targeted, ok := r.Jobs.(targetedJobs); ok {
			claim, err = targeted.ClaimJob(ctx, r.JobID)
		} else {
			return Outcome{}, errors.New("local-agent jobs client does not support targeted claims")
		}
	} else {
		claim, err = r.Jobs.Claim(ctx)
	}
	if err != nil {
		return Outcome{}, err
	}
	if claim == nil {
		return Outcome{State: OutcomeIdle}, nil
	}
	product, err := r.Crawler.Process(ctx, claim.Job.URL)
	if err != nil {
		_, submitErr := r.Jobs.SubmitFailure(ctx, claim.Job.ID, claim.ExecutionToken, classifyFailure(err))
		if submitErr != nil {
			return Outcome{}, submitErr
		}
		return Outcome{State: OutcomeFailed, JobID: claim.Job.ID}, nil
	}
	_, err = r.Jobs.SubmitSuccess(ctx, claim.Job.ID, claim.ExecutionToken, a1688.SnapshotFromLegacyProduct(product))
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{State: OutcomeSucceeded, JobID: claim.Job.ID}, nil
}

func classifyFailure(err error) Failure {
	var accessErr *alibaba1688.PublicAccessError
	if errors.As(err, &accessErr) {
		switch accessErr.Kind {
		case alibaba1688.PublicAccessFailureBrowser:
			return Failure{Kind: FailureBrowser, Message: "1688 browser could not be started"}
		case alibaba1688.PublicAccessFailureChallenge:
			return Failure{Kind: FailureChallenge, Message: "1688 challenge detected"}
		case alibaba1688.PublicAccessFailureMissingFields:
			return Failure{Kind: FailureExtraction, Message: "1688 product fields could not be extracted"}
		case alibaba1688.PublicAccessFailureInvalidURL:
			return Failure{Kind: FailureNavigation, Message: "1688 offer URL could not be opened"}
		case alibaba1688.PublicAccessFailureTransport:
			return Failure{Kind: FailureNavigation, Message: "1688 page navigation failed"}
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Failure{Kind: FailureNavigation, Message: "1688 page navigation was canceled"}
	}
	return Failure{Kind: FailureUnknown, Message: "1688 local crawl failed"}
}
