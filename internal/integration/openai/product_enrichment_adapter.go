package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"

	"task-processor/internal/product/enrichment"
)

// TextInvoker is the narrow Integration-owned capability required by product
// enrichment. OpenAI clients may implement it without exposing provider request
// or response types to the product domain.
type TextInvoker interface {
	Generate(context.Context, string) (string, error)
}

// ProductEnrichmentAdapter translates the provider-neutral enrichment request
// into one deterministic text invocation and validates its strict JSON output.
type ProductEnrichmentAdapter struct {
	invoker TextInvoker
}

func NewProductEnrichmentAdapter(invoker TextInvoker) *ProductEnrichmentAdapter {
	return &ProductEnrichmentAdapter{invoker: invoker}
}

func (a *ProductEnrichmentAdapter) Generate(ctx context.Context, request enrichment.GenerationRequest) (enrichment.Candidate, error) {
	if ctx == nil {
		return enrichment.Candidate{}, enrichment.ErrInputInvalid
	}
	if err := ctx.Err(); err != nil {
		return enrichment.Candidate{}, canonicalContextError(err)
	}
	if a == nil || isNilTextInvoker(a.invoker) {
		return enrichment.Candidate{}, enrichment.ErrExternalCapabilityUnavailable
	}

	evidenceID, err := enrichment.CanonicalEvidenceID(request.Source)
	if err != nil {
		return enrichment.Candidate{}, err
	}
	requestedFields, err := canonicalRequestedFields(request.Policy)
	if err != nil {
		return enrichment.Candidate{}, err
	}
	prompt, err := buildProductEnrichmentPrompt(request, evidenceID, requestedFields)
	if err != nil {
		return enrichment.Candidate{}, enrichment.ErrInputInvalid
	}

	response, err := a.invoker.Generate(ctx, prompt)
	if contextErr := ctx.Err(); contextErr != nil {
		return enrichment.Candidate{}, canonicalContextError(contextErr)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return enrichment.Candidate{}, canonicalContextError(err)
		}
		return enrichment.Candidate{}, enrichment.ErrExternalCapabilityUnavailable
	}

	candidate, err := parseProductEnrichmentResponse(response, requestedFields, evidenceID)
	if err != nil {
		return enrichment.Candidate{}, err
	}
	return candidate, nil
}

func canonicalContextError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return err
}

func isNilTextInvoker(invoker TextInvoker) bool {
	if invoker == nil {
		return true
	}
	value := reflect.ValueOf(invoker)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func canonicalRequestedFields(policy enrichment.PolicySnapshot) ([]string, error) {
	fields := policy.AllowedFields
	if len(fields) == 0 {
		fields = policy.RequiredFields
	}
	if len(fields) == 0 {
		fields = []string{"brand", "description", "title"}
	}

	seen := make(map[string]struct{}, len(fields))
	canonical := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" || field != strings.TrimSpace(field) {
			return nil, enrichment.ErrInputInvalid
		}
		if _, duplicate := seen[field]; duplicate {
			return nil, enrichment.ErrInputInvalid
		}
		seen[field] = struct{}{}
		canonical = append(canonical, field)
	}
	sort.Strings(canonical)
	return canonical, nil
}

func buildProductEnrichmentPrompt(request enrichment.GenerationRequest, evidenceID string, requestedFields []string) (string, error) {
	policy := request.Policy
	policy.AllowedFields = append([]string(nil), policy.AllowedFields...)
	policy.RequiredFields = append([]string(nil), policy.RequiredFields...)
	sort.Strings(policy.AllowedFields)
	sort.Strings(policy.RequiredFields)

	domainRequest := request
	domainRequest.Policy = policy
	payload := struct {
		Instruction     string                       `json:"instruction"`
		RequestedFields []string                     `json:"requested_fields"`
		EvidenceID      string                       `json:"evidence_id"`
		Request         enrichment.GenerationRequest `json:"request"`
	}{
		Instruction:     "Return exactly one JSON object. Use only requested field paths as keys and non-empty strings as values. Return no markdown, metadata, scores, or explanations.",
		RequestedFields: append([]string(nil), requestedFields...),
		EvidenceID:      evidenceID,
		Request:         domainRequest,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func parseProductEnrichmentResponse(raw string, requestedFields []string, evidenceID string) (enrichment.Candidate, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	first, err := decoder.Token()
	if err != nil {
		return enrichment.Candidate{}, enrichment.ErrOutputValidation
	}
	opening, ok := first.(json.Delim)
	if !ok || opening != '{' {
		return enrichment.Candidate{}, enrichment.ErrOutputValidation
	}

	allowed := make(map[string]struct{}, len(requestedFields))
	for _, field := range requestedFields {
		allowed[field] = struct{}{}
	}
	values := make(map[string]string, len(requestedFields))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return enrichment.Candidate{}, enrichment.ErrOutputValidation
		}
		field, ok := token.(string)
		if !ok {
			return enrichment.Candidate{}, enrichment.ErrOutputValidation
		}
		if _, supported := allowed[field]; !supported {
			return enrichment.Candidate{}, enrichment.ErrOutputValidation
		}
		if _, duplicate := values[field]; duplicate {
			return enrichment.Candidate{}, enrichment.ErrOutputValidation
		}
		var value string
		if err := decoder.Decode(&value); err != nil || value == "" || value != strings.TrimSpace(value) {
			return enrichment.Candidate{}, enrichment.ErrOutputValidation
		}
		values[field] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(values) == 0 {
		return enrichment.Candidate{}, enrichment.ErrOutputValidation
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return enrichment.Candidate{}, enrichment.ErrOutputValidation
	}

	fields := make([]string, 0, len(values))
	for field := range values {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	changes := make([]enrichment.FieldChange, 0, len(fields))
	for _, field := range fields {
		changes = append(changes, enrichment.FieldChange{
			Field:       field,
			Value:       values[field],
			EvidenceIDs: []string{evidenceID},
		})
	}
	return enrichment.Candidate{Changes: changes}, nil
}

var _ enrichment.CandidateGenerator = (*ProductEnrichmentAdapter)(nil)
