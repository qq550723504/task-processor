package enrichment

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestValidateCandidateRechecksRepairedFieldsEvidenceAndPolicy(t *testing.T) {
	request := validRequest()
	request.Policy.AllowedFields = []string{"title"}
	request.Policy.RequiredFields = []string{"title"}
	candidate := Candidate{Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}}}
	first, err := ValidateCandidate(context.Background(), request, candidate)
	if !errors.Is(err, ErrPolicyRejected) || first.Validation.Valid {
		t.Fatal("disallowed/missing field was not rejected", err)
	}
	candidate.Changes[0].Field = "title"
	second, err := ValidateCandidate(context.Background(), request, candidate)
	if err != nil || !second.Validation.Valid || second.Validation.EvaluatedChanges != 1 {
		t.Fatal("repaired candidate was not revalidated", err)
	}
	candidate.Changes[0].EvidenceIDs[0] = "another-source"
	third, err := ValidateCandidate(context.Background(), request, candidate)
	if !errors.Is(err, ErrEvidenceInsufficient) || third.Validation.Valid {
		t.Fatal("repair introduced unbound evidence", err)
	}
	candidate.Changes[0].EvidenceIDs[0] = "raw-1"
	request.Policy.AllowedFields = []string{"description"}
	request.Policy.RequiredFields = []string{"description"}
	fourth, err := ValidateCandidate(context.Background(), request, candidate)
	if !errors.Is(err, ErrPolicyRejected) || fourth.Validation.Valid {
		t.Fatal("previous validation bypassed current policy", err)
	}
}

func TestValidateCandidatePreservesCallerInputsAndMatchesProposer(t *testing.T) {
	request := validRequest()
	candidate := Candidate{Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1", "raw-1"}}}}
	before := cloneCandidate(candidate)
	p, err := NewProposer(Dependencies{Generator: candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) { return candidate, nil })})
	if err != nil {
		t.Fatal(err)
	}
	generated, err := p.Propose(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateCandidate(context.Background(), request, candidate)
	if err != nil || !reflect.DeepEqual(validated, generated) || !reflect.DeepEqual(before, candidate) {
		t.Fatal("pure validation diverged or mutated caller-owned candidate", err)
	}
	validated.Changes[0].EvidenceIDs[0] = "changed"
	if !reflect.DeepEqual(before, candidate) {
		t.Fatal("result aliases original candidate")
	}
}

func TestValidateCandidateRejectsInvalidOrCancelledInput(t *testing.T) {
	if _, err := ValidateCandidate(nil, validRequest(), Candidate{}); !errors.Is(err, ErrInputInvalid) {
		t.Fatal("nil context accepted", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ValidateCandidate(ctx, validRequest(), Candidate{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled validation ran", err)
	}
	if _, err := ValidateCandidate(context.Background(), Request{}, Candidate{}); !errors.Is(err, ErrInputInvalid) {
		t.Fatal("invalid request accepted", err)
	}
}
