package localagent

import (
	"context"
	"errors"
	"time"

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

type crawlerPreparer interface {
	Prepare(context.Context) error
}

type Crawler interface {
	Process(context.Context, string) (*model.Product1688, error)
}

type Runner struct {
	Jobs             Jobs
	Crawler          Crawler
	JobID            string
	CrawlerPrepared  bool
	PreparationError error
}

type OutcomeState string

const (
	OutcomeIdle      OutcomeState = "idle"
	OutcomeSucceeded OutcomeState = "succeeded"
	OutcomeFailed    OutcomeState = "failed"
)

const failureSubmitTimeout = 10 * time.Second

type Outcome struct {
	State           OutcomeState
	JobID           string
	EnvelopeSummary *EnvelopeSummary
}

func (r Runner) RunOnce(ctx context.Context) (Outcome, error) {
	if r.Jobs == nil || r.Crawler == nil {
		return Outcome{}, errors.New("local-agent runner is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if r.PreparationError != nil {
		return r.reportPreparationFailure(ctx, r.PreparationError)
	}
	if !r.CrawlerPrepared {
		if preparer, ok := r.Crawler.(crawlerPreparer); ok {
			if err := preparer.Prepare(ctx); err != nil {
				return r.reportPreparationFailure(ctx, err)
			}
		}
	}
	claim, err := r.claim(ctx)
	if err != nil {
		return Outcome{}, err
	}
	if claim == nil {
		return Outcome{State: OutcomeIdle}, nil
	}
	product, err := r.Crawler.Process(ctx, claim.Job.URL)
	if err != nil {
		_, submitErr := r.submitFailure(ctx, claim.Job.ID, claim.ExecutionToken, classifyFailure(err))
		if submitErr != nil {
			return Outcome{}, submitErr
		}
		return Outcome{State: OutcomeFailed, JobID: claim.Job.ID}, nil
	}
	if product == nil {
		_, submitErr := r.submitFailure(ctx, claim.Job.ID, claim.ExecutionToken, Failure{Kind: FailureExtraction, Message: "1688 crawler returned no product"})
		if submitErr != nil {
			return Outcome{}, submitErr
		}
		return Outcome{State: OutcomeFailed, JobID: claim.Job.ID}, nil
	}
	var submitted Job
	submitted, err = r.Jobs.SubmitSuccess(ctx, claim.Job.ID, claim.ExecutionToken, a1688.SnapshotFromLegacyProduct(product))
	if err != nil {
		if errors.Is(err, ErrSnapshotTooLarge) || errors.Is(err, ErrSnapshotInvalid) {
			message := "1688 product snapshot failed server validation"
			if errors.Is(err, ErrSnapshotTooLarge) {
				message = "1688 product snapshot exceeds submission size limit"
			}
			_, submitErr := r.submitFailure(ctx, claim.Job.ID, claim.ExecutionToken, Failure{Kind: FailureExtraction, Message: message})
			if submitErr != nil {
				return Outcome{}, submitErr
			}
			return Outcome{State: OutcomeFailed, JobID: claim.Job.ID}, nil
		}
		return Outcome{}, err
	}
	return Outcome{State: OutcomeSucceeded, JobID: claim.Job.ID, EnvelopeSummary: submitted.EnvelopeSummary}, nil
}

func (r Runner) reportPreparationFailure(ctx context.Context, preparationErr error) (Outcome, error) {
	claim, claimErr := r.claim(ctx)
	if claimErr != nil {
		return Outcome{}, preparationErr
	}
	if claim == nil {
		return Outcome{}, preparationErr
	}
	_, submitErr := r.submitFailure(ctx, claim.Job.ID, claim.ExecutionToken, Failure{Kind: FailureBrowser, Message: "1688 browser could not be started"})
	if submitErr != nil {
		return Outcome{}, submitErr
	}
	return Outcome{State: OutcomeFailed, JobID: claim.Job.ID}, nil
}

func (r Runner) claim(ctx context.Context) (*Claim, error) {
	if r.JobID != "" {
		if targeted, ok := r.Jobs.(targetedJobs); ok {
			return targeted.ClaimJob(ctx, r.JobID)
		}
		return nil, errors.New("local-agent jobs client does not support targeted claims")
	}
	return r.Jobs.Claim(ctx)
}

func (r Runner) submitFailure(ctx context.Context, jobID, executionToken string, failure Failure) (Job, error) {
	failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureSubmitTimeout)
	defer cancel()
	return r.Jobs.SubmitFailure(failureCtx, jobID, executionToken, failure)
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
		case alibaba1688.PublicAccessFailureValidation:
			return Failure{Kind: FailureExtraction, Message: "1688 product validation failed"}
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
