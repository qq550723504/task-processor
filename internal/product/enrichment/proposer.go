package enrichment

import (
	"context"
	"errors"
	"reflect"
)

type Proposer interface {
	Propose(context.Context, Request) (Proposal, error)
}

type Dependencies struct {
	Generator CandidateGenerator
}

type proposer struct {
	generator CandidateGenerator
}

func NewProposer(deps Dependencies) (Proposer, error) {
	if isNilCandidateGenerator(deps.Generator) {
		return nil, ErrExternalCapabilityUnavailable
	}
	return &proposer{generator: deps.Generator}, nil
}

func isNilCandidateGenerator(generator CandidateGenerator) bool {
	if generator == nil {
		return true
	}
	value := reflect.ValueOf(generator)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (p *proposer) Propose(ctx context.Context, request Request) (Proposal, error) {
	if ctx == nil {
		return Proposal{}, ErrInputInvalid
	}
	if err := ctx.Err(); err != nil {
		return Proposal{}, err
	}
	working := cloneRequest(request)
	if err := validateRequest(working); err != nil {
		return Proposal{}, err
	}

	generationInput := cloneRequest(working)
	if err := ctx.Err(); err != nil {
		return Proposal{}, err
	}
	candidate, err := p.generator.Generate(ctx, GenerationRequest{
		Snapshot: generationInput.Snapshot,
		Source:   generationInput.Source,
		Policy:   generationInput.Policy,
	})
	if contextErr := ctx.Err(); contextErr != nil {
		return Proposal{}, contextErr
	}
	if err != nil {
		return Proposal{}, stableGenerationError(err)
	}
	candidate = cloneCandidate(candidate)

	evidence, err := validateEvidence(working.Source, candidate.Changes)
	if err != nil {
		return Proposal{}, err
	}
	quality := scoreCandidate(candidate, working.Policy)
	proposal := Proposal{
		Changes:    cloneFieldChanges(candidate.Changes),
		Evidence:   cloneEvidence(evidence),
		Quality:    quality,
		Warnings:   stableWarnings(candidate.Warnings),
		Rejections: stableRejections(candidate.Rejections),
	}
	proposal.Validation = ValidationResult{
		Valid:            len(proposal.Rejections) == 0,
		EvaluatedChanges: len(proposal.Changes),
	}
	if err := validateProposal(&proposal, working.Policy); err != nil {
		return proposal, err
	}
	return proposal, nil
}

func stableGenerationError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	for _, stable := range []error{
		ErrInputInvalid,
		ErrEvidenceInsufficient,
		ErrCapabilityUnsupported,
		ErrPolicyRejected,
		ErrExternalCapabilityUnavailable,
		ErrOutputValidation,
	} {
		if errors.Is(err, stable) {
			return stable
		}
	}
	return ErrExternalCapabilityUnavailable
}
