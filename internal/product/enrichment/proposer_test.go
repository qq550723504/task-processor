package enrichment

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func TestNewProposerRejectsNilGenerator(t *testing.T) {
	t.Parallel()

	proposer, err := NewProposer(Dependencies{})
	if proposer != nil {
		t.Fatalf("NewProposer() proposer = %#v, want nil", proposer)
	}
	if !errors.Is(err, ErrExternalCapabilityUnavailable) {
		t.Fatalf("NewProposer() error = %v, want ErrExternalCapabilityUnavailable", err)
	}
}

func TestNewProposerRejectsTypedNilGenerator(t *testing.T) {
	t.Parallel()

	var generator *nilCandidateGenerator
	proposer, err := NewProposer(Dependencies{Generator: generator})
	if proposer != nil {
		t.Fatalf("NewProposer() proposer = %#v, want nil", proposer)
	}
	if !errors.Is(err, ErrExternalCapabilityUnavailable) {
		t.Fatalf("NewProposer() error = %v, want ErrExternalCapabilityUnavailable", err)
	}
}

func TestProposeBuildsOneCanonicalEvidenceBackedProposal(t *testing.T) {
	t.Parallel()

	generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		return Candidate{Changes: []FieldChange{{
			Field:       "description",
			Value:       "Steel bottle",
			EvidenceIDs: []string{"raw-1"},
		}}}, nil
	})
	proposer, err := NewProposer(Dependencies{Generator: generator})
	if err != nil {
		t.Fatalf("NewProposer() error = %v", err)
	}

	capturedAt := time.Date(2026, time.August, 31, 9, 8, 7, 0, time.UTC)
	request := validRequest()
	request.Source.RawReference = sourcing.RawSourceReference{
		ReferenceType: "crawler_snapshot",
		ReferenceID:   "raw-1",
		SnapshotID:    "snapshot-7",
		Checksum:      "sha256:source",
		URL:           "https://source.example/products/B001",
		CapturedAt:    capturedAt,
		Metadata:      map[string]string{"etag": "v7"},
	}

	proposal, err := proposer.Propose(context.Background(), request)
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	wantEvidence := []Evidence{{
		ReferenceType: "crawler_snapshot",
		ID:            "raw-1",
		ReferenceID:   "raw-1",
		SnapshotID:    "snapshot-7",
		Checksum:      "sha256:source",
		URL:           "https://source.example/products/B001",
		CapturedAt:    capturedAt,
		Metadata:      map[string]string{"etag": "v7"},
	}}
	if !reflect.DeepEqual(proposal.Evidence, wantEvidence) {
		t.Fatalf("Propose() evidence = %#v, want %#v", proposal.Evidence, wantEvidence)
	}
	if got := proposal.Changes; !reflect.DeepEqual(got, []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}}) {
		t.Fatalf("Propose() changes = %#v", got)
	}
	if proposal.Quality.Overall != 100 || proposal.Quality.EvidenceCoverage != 100 || proposal.Quality.RequiredFieldCoverage != 100 {
		t.Fatalf("Propose() quality = %#v, want all scores 100", proposal.Quality)
	}
	if !proposal.Validation.Valid || proposal.Validation.EvaluatedChanges != 1 {
		t.Fatalf("Propose() validation = %#v, want valid one-change proposal", proposal.Validation)
	}
}

func TestProposeReturnsOnlyStableGenerationErrors(t *testing.T) {
	t.Parallel()

	providerFailure := errors.New("provider sdk failure")
	tests := []struct {
		name      string
		generator candidateGeneratorFunc
		want      error
		not       error
	}{
		{
			name: "unknown generator failure is hidden",
			generator: func(context.Context, GenerationRequest) (Candidate, error) {
				return Candidate{}, providerFailure
			},
			want: ErrExternalCapabilityUnavailable,
			not:  providerFailure,
		},
		{
			name: "wrapped cancellation is canonical",
			generator: func(context.Context, GenerationRequest) (Candidate, error) {
				return Candidate{}, fmt.Errorf("provider stopped: %w", context.Canceled)
			},
			want: context.Canceled,
		},
		{
			name: "wrapped deadline is canonical",
			generator: func(context.Context, GenerationRequest) (Candidate, error) {
				return Candidate{}, fmt.Errorf("provider stopped: %w", context.DeadlineExceeded)
			},
			want: context.DeadlineExceeded,
		},
		{
			name: "domain error remains canonical",
			generator: func(context.Context, GenerationRequest) (Candidate, error) {
				return Candidate{}, fmt.Errorf("adapter parse: %w", ErrOutputValidation)
			},
			want: ErrOutputValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposer, err := NewProposer(Dependencies{Generator: tt.generator})
			if err != nil {
				t.Fatalf("NewProposer() error = %v", err)
			}
			_, err = proposer.Propose(context.Background(), validRequest())
			if err != tt.want {
				t.Fatalf("Propose() error = %v, want canonical %v", err, tt.want)
			}
			if tt.not != nil && errors.Is(err, tt.not) {
				t.Fatalf("Propose() error exposes generator failure %v", tt.not)
			}
		})
	}
}

func TestProposeHonorsCancellationThatOccursDuringGeneration(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		cancel()
		return Candidate{Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}}}, nil
	})
	proposer, err := NewProposer(Dependencies{Generator: generator})
	if err != nil {
		t.Fatalf("NewProposer() error = %v", err)
	}

	proposal, err := proposer.Propose(ctx, validRequest())
	if err != context.Canceled {
		t.Fatalf("Propose() error = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(proposal, Proposal{}) {
		t.Fatalf("Propose() proposal = %#v, want zero proposal after cancellation", proposal)
	}
}

func TestProposeDoesNotGenerateWhenContextCancelsDuringInputPreparation(t *testing.T) {
	t.Parallel()

	generatorCalls := 0
	proposer, err := NewProposer(Dependencies{Generator: candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		generatorCalls++
		return Candidate{Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}}}, nil
	})})
	if err != nil {
		t.Fatalf("NewProposer() error = %v", err)
	}

	proposal, err := proposer.Propose(&cancelAfterFirstErrContext{}, validRequest())
	if err != context.Canceled {
		t.Fatalf("Propose() error = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(proposal, Proposal{}) {
		t.Fatalf("Propose() proposal = %#v, want zero proposal", proposal)
	}
	if generatorCalls != 0 {
		t.Fatalf("CandidateGenerator calls = %d, want 0", generatorCalls)
	}
}

func TestProposeUsesOneCanonicalRawEvidenceIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		reference   sourcing.RawSourceReference
		referenceID string
		wantErr     error
	}{
		{
			name: "reference id wins over snapshot and checksum",
			reference: sourcing.RawSourceReference{
				ReferenceID: "reference-1",
				SnapshotID:  "snapshot-1",
				Checksum:    "sha256:one",
			},
			referenceID: "reference-1",
		},
		{
			name:        "snapshot id is the first fallback",
			reference:   sourcing.RawSourceReference{SnapshotID: "snapshot-1", Checksum: "sha256:one"},
			referenceID: "snapshot-1",
		},
		{
			name:        "checksum is the final fallback",
			reference:   sourcing.RawSourceReference{Checksum: "sha256:one"},
			referenceID: "sha256:one",
		},
		{
			name:        "source identity is not raw evidence",
			reference:   sourcing.RawSourceReference{},
			referenceID: "B001",
			wantErr:     ErrEvidenceInsufficient,
		},
		{
			name:        "unknown raw evidence id is rejected",
			reference:   sourcing.RawSourceReference{ReferenceID: "raw-1"},
			referenceID: "unknown",
			wantErr:     ErrEvidenceInsufficient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validRequest()
			request.Source.RawReference = tt.reference
			generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
				return Candidate{Changes: []FieldChange{{
					Field:       "description",
					Value:       "Steel bottle",
					EvidenceIDs: []string{tt.referenceID, tt.referenceID},
				}}}, nil
			})
			proposer, err := NewProposer(Dependencies{Generator: generator})
			if err != nil {
				t.Fatalf("NewProposer() error = %v", err)
			}

			proposal, err := proposer.Propose(context.Background(), request)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("Propose() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Propose() error = %v", err)
			}
			if len(proposal.Evidence) != 1 || proposal.Evidence[0].ID != tt.referenceID {
				t.Fatalf("Propose() evidence = %#v, want one evidence with ID %q", proposal.Evidence, tt.referenceID)
			}
			if got := proposal.Changes[0].EvidenceIDs; !reflect.DeepEqual(got, []string{tt.referenceID}) {
				t.Fatalf("Propose() change evidence IDs = %v, want one canonical ID", got)
			}
		})
	}
}

func TestProposeUsesFixedValidationAndScoringOrder(t *testing.T) {
	t.Parallel()

	t.Run("input validation precedes generation", func(t *testing.T) {
		request := validRequest()
		request.Policy.Version = ""
		proposer, err := NewProposer(Dependencies{Generator: candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
			panic("generator must not run for invalid input")
		})})
		if err != nil {
			t.Fatalf("NewProposer() error = %v", err)
		}
		_, err = proposer.Propose(context.Background(), request)
		if err != ErrInputInvalid {
			t.Fatalf("Propose() error = %v, want ErrInputInvalid", err)
		}
	})

	t.Run("evidence validation precedes output validation", func(t *testing.T) {
		proposer, err := NewProposer(Dependencies{Generator: candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
			return Candidate{Changes: []FieldChange{{Field: "", Value: "", EvidenceIDs: []string{"unknown"}}}}, nil
		})})
		if err != nil {
			t.Fatalf("NewProposer() error = %v", err)
		}
		_, err = proposer.Propose(context.Background(), validRequest())
		if err != ErrEvidenceInsufficient {
			t.Fatalf("Propose() error = %v, want ErrEvidenceInsufficient", err)
		}
	})

	t.Run("scoring precedes output validation", func(t *testing.T) {
		proposer, err := NewProposer(Dependencies{Generator: candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
			return Candidate{Changes: []FieldChange{{Field: "", Value: "", EvidenceIDs: []string{"raw-1"}}}}, nil
		})})
		if err != nil {
			t.Fatalf("NewProposer() error = %v", err)
		}
		proposal, err := proposer.Propose(context.Background(), validRequest())
		if err != ErrOutputValidation {
			t.Fatalf("Propose() error = %v, want ErrOutputValidation", err)
		}
		if proposal.Quality.EvidenceCoverage != 100 {
			t.Fatalf("Propose() quality = %#v, want scoring result retained before validation error", proposal.Quality)
		}
	})
}

func TestProposeRejectsMalformedWarningsAndRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		candidate Candidate
	}{
		{
			name: "warning without code",
			candidate: Candidate{
				Changes:  []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}},
				Warnings: []Warning{{Code: "  ", Message: "source is limited"}},
			},
		},
		{
			name: "rejection without code",
			candidate: Candidate{
				Changes:    []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}},
				Rejections: []Rejection{{Code: "", Message: "cannot support claim"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposer, err := NewProposer(Dependencies{Generator: candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
				return tt.candidate, nil
			})})
			if err != nil {
				t.Fatalf("NewProposer() error = %v", err)
			}
			_, err = proposer.Propose(context.Background(), validRequest())
			if err != ErrOutputValidation {
				t.Fatalf("Propose() error = %v, want ErrOutputValidation", err)
			}
		})
	}
}

func TestProposeValidatesContextAndInputBeforeGeneration(t *testing.T) {
	t.Parallel()

	validCandidate := Candidate{Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}}}
	generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		return validCandidate, nil
	})
	proposer, err := NewProposer(Dependencies{Generator: generator})
	if err != nil {
		t.Fatalf("NewProposer() error = %v", err)
	}

	t.Run("nil context", func(t *testing.T) {
		_, err := proposer.Propose(nil, validRequest()) //nolint:staticcheck
		if err != ErrInputInvalid {
			t.Fatalf("Propose() error = %v, want ErrInputInvalid", err)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := proposer.Propose(ctx, validRequest())
		if err != context.Canceled {
			t.Fatalf("Propose() error = %v, want context.Canceled", err)
		}
	})

	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{
			name: "source identity is incomplete",
			mutate: func(request *Request) {
				request.Source.Identity.SourcePlatform = ""
			},
		},
		{
			name: "policy version is not canonical",
			mutate: func(request *Request) {
				request.Policy.Version = " v1 "
			},
		},
		{
			name: "policy field is duplicated",
			mutate: func(request *Request) {
				request.Policy.AllowedFields = []string{"description", "description"}
			},
		},
		{
			name: "required field is not allowed",
			mutate: func(request *Request) {
				request.Policy.RequiredFields = []string{"title"}
			},
		},
		{
			name: "minimum score is NaN",
			mutate: func(request *Request) {
				request.Policy.MinimumQualityScore = math.NaN()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validRequest()
			tt.mutate(&request)
			_, err := proposer.Propose(context.Background(), request)
			if err != ErrInputInvalid {
				t.Fatalf("Propose() error = %v, want ErrInputInvalid", err)
			}
		})
	}
}

func TestProposeReturnsStableWarningsRejectionsAndQuality(t *testing.T) {
	t.Parallel()

	request := validRequest()
	request.Policy.AllowedFields = []string{"description", "title"}
	request.Policy.RequiredFields = []string{"description", "title"}
	request.Policy.MinimumQualityScore = 90
	generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		return Candidate{
			Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}},
			Warnings: []Warning{
				{Code: " ZETA ", Field: " title ", Message: " Last warning "},
				{Code: "ALPHA", Field: " description ", Message: " First warning "},
			},
			Rejections: []Rejection{
				{Code: " ZETA ", Field: " title ", Message: " Last rejection "},
				{Code: "ALPHA", Field: " description ", Message: " First rejection "},
			},
		}, nil
	})
	proposer, err := NewProposer(Dependencies{Generator: generator})
	if err != nil {
		t.Fatalf("NewProposer() error = %v", err)
	}

	proposal, err := proposer.Propose(context.Background(), request)
	if err != ErrPolicyRejected {
		t.Fatalf("Propose() error = %v, want ErrPolicyRejected", err)
	}
	if proposal.Quality.EvidenceCoverage != 100 || proposal.Quality.RequiredFieldCoverage != 50 || proposal.Quality.Overall != 75 {
		t.Fatalf("Propose() quality = %#v, want evidence=100 required=50 overall=75", proposal.Quality)
	}
	if proposal.Validation.Valid {
		t.Fatalf("Propose() validation = %#v, want rejected", proposal.Validation)
	}
	wantWarnings := []Warning{
		{Code: "alpha", Field: "description", Message: "First warning"},
		{Code: "zeta", Field: "title", Message: "Last warning"},
	}
	if !reflect.DeepEqual(proposal.Warnings, wantWarnings) {
		t.Fatalf("Propose() warnings = %#v, want %#v", proposal.Warnings, wantWarnings)
	}
	wantRejections := []Rejection{
		{Code: "alpha", Field: "description", Message: "First rejection"},
		{Code: "quality_below_minimum", Message: "proposal quality is below policy minimum"},
		{Code: "required_field_missing", Field: "title", Message: "required field change is missing"},
		{Code: "zeta", Field: "title", Message: "Last rejection"},
	}
	if !reflect.DeepEqual(proposal.Rejections, wantRejections) {
		t.Fatalf("Propose() rejections = %#v, want %#v", proposal.Rejections, wantRejections)
	}
}

func TestProposeDeduplicatesCanonicalGeneratorDiagnostics(t *testing.T) {
	t.Parallel()

	metadata := map[string]string{"source": "raw-1"}
	generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		return Candidate{
			Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}},
			Warnings: []Warning{
				{Code: " SOURCE_LIMITED ", Field: " description ", Message: " Only one source ", Metadata: metadata},
				{Code: "source_limited", Field: "description", Message: "Only one source", Metadata: map[string]string{"source": "raw-1"}},
			},
			Rejections: []Rejection{
				{Code: " CLAIM_UNSUPPORTED ", Field: " title ", Message: " Claim lacks evidence ", Metadata: metadata},
				{Code: "claim_unsupported", Field: "title", Message: "Claim lacks evidence", Metadata: map[string]string{"source": "raw-1"}},
			},
		}, nil
	})
	proposer, err := NewProposer(Dependencies{Generator: generator})
	if err != nil {
		t.Fatalf("NewProposer() error = %v", err)
	}

	proposal, err := proposer.Propose(context.Background(), validRequest())
	if err != ErrPolicyRejected {
		t.Fatalf("Propose() error = %v, want ErrPolicyRejected", err)
	}
	if len(proposal.Warnings) != 1 {
		t.Fatalf("Propose() warnings = %#v, want one canonical warning", proposal.Warnings)
	}
	if len(proposal.Rejections) != 1 {
		t.Fatalf("Propose() rejections = %#v, want one canonical rejection", proposal.Rejections)
	}
}

func TestProposeOrdersDistinctDiagnosticMetadataDeterministically(t *testing.T) {
	t.Parallel()

	metadataOne := map[string]string{"a": "b", "c": "d"}
	metadataTwo := map[string]string{"a": "b\x00c\x00d"}
	propose := func(reverse bool) Proposal {
		t.Helper()

		warnings := []Warning{
			{Code: "same", Field: "description", Message: "same warning", Metadata: metadataOne},
			{Code: "same", Field: "description", Message: "same warning", Metadata: metadataTwo},
		}
		rejections := []Rejection{
			{Code: "same", Field: "title", Message: "same rejection", Metadata: metadataOne},
			{Code: "same", Field: "title", Message: "same rejection", Metadata: metadataTwo},
		}
		if reverse {
			warnings[0], warnings[1] = warnings[1], warnings[0]
			rejections[0], rejections[1] = rejections[1], rejections[0]
		}
		proposer, err := NewProposer(Dependencies{Generator: candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
			return Candidate{
				Changes:    []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}},
				Warnings:   warnings,
				Rejections: rejections,
			}, nil
		})})
		if err != nil {
			t.Fatalf("NewProposer() error = %v", err)
		}
		proposal, err := proposer.Propose(context.Background(), validRequest())
		if err != ErrPolicyRejected {
			t.Fatalf("Propose() error = %v, want ErrPolicyRejected", err)
		}
		return proposal
	}

	forward := propose(false)
	reversed := propose(true)
	if !reflect.DeepEqual(forward, reversed) {
		t.Fatalf("diagnostic order depends on generator order\nforward: %#v\nreverse: %#v", forward, reversed)
	}
	if len(forward.Warnings) != 2 || len(forward.Rejections) != 2 {
		t.Fatalf("distinct metadata was collapsed: warnings=%#v rejections=%#v", forward.Warnings, forward.Rejections)
	}
}

func TestProposeDeduplicatesGeneratorAndPolicyRejection(t *testing.T) {
	t.Parallel()

	request := validRequest()
	request.Policy.AllowedFields = []string{"description", "title"}
	request.Policy.RequiredFields = []string{"description", "title"}
	request.Policy.MinimumQualityScore = 0
	generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		return Candidate{
			Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}},
			Rejections: []Rejection{{
				Code:    " REQUIRED_FIELD_MISSING ",
				Field:   " title ",
				Message: " required field change is missing ",
			}},
		}, nil
	})
	proposer, err := NewProposer(Dependencies{Generator: generator})
	if err != nil {
		t.Fatalf("NewProposer() error = %v", err)
	}

	proposal, err := proposer.Propose(context.Background(), request)
	if err != ErrPolicyRejected {
		t.Fatalf("Propose() error = %v, want ErrPolicyRejected", err)
	}
	want := []Rejection{{
		Code:    "required_field_missing",
		Field:   "title",
		Message: "required field change is missing",
	}}
	if !reflect.DeepEqual(proposal.Rejections, want) {
		t.Fatalf("Propose() rejections = %#v, want %#v", proposal.Rejections, want)
	}
}

func validRequest() Request {
	return Request{
		Snapshot: catalog.ProductSnapshot{Title: "Bottle"},
		Source: sourcing.SourceEnvelope{
			Identity: sourcing.SourceIdentity{
				SourceType:     sourcing.SourceTypeCrawler,
				SourcePlatform: "amazon",
				SourceID:       "B001",
			},
			RawReference: sourcing.RawSourceReference{ReferenceID: "raw-1"},
		},
		Policy: PolicySnapshot{
			Version:             "v1",
			AllowedFields:       []string{"description"},
			RequiredFields:      []string{"description"},
			MinimumQualityScore: 80,
		},
	}
}

type candidateGeneratorFunc func(context.Context, GenerationRequest) (Candidate, error)

func (f candidateGeneratorFunc) Generate(ctx context.Context, request GenerationRequest) (Candidate, error) {
	return f(ctx, request)
}

type nilCandidateGenerator struct{}

func (*nilCandidateGenerator) Generate(context.Context, GenerationRequest) (Candidate, error) {
	panic("typed-nil generator must be rejected before use")
}

type cancelAfterFirstErrContext struct {
	errCalls int
}

func (*cancelAfterFirstErrContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*cancelAfterFirstErrContext) Done() <-chan struct{}       { return nil }
func (c *cancelAfterFirstErrContext) Err() error {
	c.errCalls++
	if c.errCalls == 1 {
		return nil
	}
	return context.Canceled
}
func (*cancelAfterFirstErrContext) Value(any) any { return nil }
