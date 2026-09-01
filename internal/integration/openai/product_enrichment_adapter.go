package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"task-processor/internal/product/enrichment"
)

const (
	productEnrichmentPromptMaxBytes          = 64 * 1024
	productEnrichmentPromptMaxRawStringBytes = 64 * 1024
	productEnrichmentPromptMaxNodes          = 4096
	productEnrichmentPromptMaxDepth          = 128
	productEnrichmentOutputMaxBytes          = 64 * 1024
)

var errProductEnrichmentByteLimitExceeded = errors.New("product enrichment byte limit exceeded")

type productEnrichmentPromptBudgetKind string

const (
	productEnrichmentPromptBudgetWithin         productEnrichmentPromptBudgetKind = "within_budget"
	productEnrichmentPromptBudgetRawStringBytes productEnrichmentPromptBudgetKind = "raw_string_bytes"
	productEnrichmentPromptBudgetNodes          productEnrichmentPromptBudgetKind = "nodes"
	productEnrichmentPromptBudgetDepth          productEnrichmentPromptBudgetKind = "depth"
	productEnrichmentPromptBudgetCycle          productEnrichmentPromptBudgetKind = "cycle"
	productEnrichmentPromptBudgetUnsupported    productEnrichmentPromptBudgetKind = "unsupported_kind"
)

type productEnrichmentPromptBudget struct {
	Kind           productEnrichmentPromptBudgetKind
	RawStringBytes int
	Nodes          int
}

type productEnrichmentPromptBudgetError struct {
	Kind productEnrichmentPromptBudgetKind
}

func (e productEnrichmentPromptBudgetError) Error() string {
	return "product enrichment prompt budget exceeded: " + string(e.Kind)
}

// TextInvocationRequest is the bounded provider-neutral request owned by this
// Integration package. Prompt and provider output must each remain within the
// advertised byte limits.
type TextInvocationRequest struct {
	Prompt         string
	MaxOutputBytes int
}

// TextInvoker is the narrow Integration-owned capability required by product
// enrichment. OpenAI clients may implement it without exposing provider request
// or response types to the product domain. The caller/App owns the business
// deadline. Implementations must honor the context and MaxOutputBytes, and own
// any shorter provider transport timeout without replacing or extending the
// caller's deadline.
type TextInvoker interface {
	Generate(context.Context, TextInvocationRequest) (string, error)
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
	// Run before requested-field canonicalization because that step allocates a
	// map proportional to the policy slice supplied by the caller.
	if budget := inspectProductEnrichmentPromptBudget(request); budget.Kind != productEnrichmentPromptBudgetWithin {
		return enrichment.Candidate{}, enrichment.ErrInputInvalid
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

	if err := ctx.Err(); err != nil {
		return enrichment.Candidate{}, canonicalContextError(err)
	}
	response, err := a.invoker.Generate(ctx, TextInvocationRequest{
		Prompt:         prompt,
		MaxOutputBytes: productEnrichmentOutputMaxBytes,
	})
	if contextErr := ctx.Err(); contextErr != nil {
		return enrichment.Candidate{}, canonicalContextError(contextErr)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return enrichment.Candidate{}, canonicalContextError(err)
		}
		return enrichment.Candidate{}, enrichment.ErrExternalCapabilityUnavailable
	}
	if len(response) > productEnrichmentOutputMaxBytes {
		return enrichment.Candidate{}, enrichment.ErrOutputValidation
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
	return buildProductEnrichmentPromptWithMarshal(request, evidenceID, requestedFields, json.Marshal)
}

func buildProductEnrichmentPromptWithMarshal(
	request enrichment.GenerationRequest,
	evidenceID string,
	requestedFields []string,
	marshal func(any) ([]byte, error),
) (string, error) {
	budget := inspectProductEnrichmentPromptBudget(request)
	if budget.Kind != productEnrichmentPromptBudgetWithin {
		return "", productEnrichmentPromptBudgetError{Kind: budget.Kind}
	}
	if marshal == nil {
		return "", enrichment.ErrInputInvalid
	}

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
	encoded, err := marshal(payload)
	if err != nil {
		return "", err
	}
	if len(encoded) > productEnrichmentPromptMaxBytes {
		return "", errProductEnrichmentByteLimitExceeded
	}
	return string(encoded), nil
}

type productEnrichmentPromptBudgetReference struct {
	kind    reflect.Kind
	typeOf  reflect.Type
	pointer uintptr
}

type productEnrichmentPromptBudgetWalker struct {
	budget productEnrichmentPromptBudget
	active map[productEnrichmentPromptBudgetReference]struct{}
}

var productEnrichmentPromptTimeType = reflect.TypeOf(time.Time{})

// inspectProductEnrichmentPromptBudget walks request structure without copying
// slices, strings, or map keys. Its only variable allocation is active-path
// cycle state, bounded by productEnrichmentPromptMaxDepth.
func inspectProductEnrichmentPromptBudget(value any) productEnrichmentPromptBudget {
	walker := &productEnrichmentPromptBudgetWalker{
		budget: productEnrichmentPromptBudget{Kind: productEnrichmentPromptBudgetWithin},
		active: make(map[productEnrichmentPromptBudgetReference]struct{}),
	}
	walker.budget.Kind = walker.visit(reflect.ValueOf(value), 0)
	return walker.budget
}

func (w *productEnrichmentPromptBudgetWalker) visit(value reflect.Value, depth int) productEnrichmentPromptBudgetKind {
	if !value.IsValid() {
		return productEnrichmentPromptBudgetWithin
	}
	if depth > productEnrichmentPromptMaxDepth {
		return productEnrichmentPromptBudgetDepth
	}
	w.budget.Nodes++
	if w.budget.Nodes > productEnrichmentPromptMaxNodes {
		return productEnrichmentPromptBudgetNodes
	}

	switch value.Kind() {
	case reflect.String:
		length := len(value.String())
		if length > productEnrichmentPromptMaxRawStringBytes-w.budget.RawStringBytes {
			w.budget.RawStringBytes = productEnrichmentPromptMaxRawStringBytes + 1
			return productEnrichmentPromptBudgetRawStringBytes
		}
		w.budget.RawStringBytes += length
		return productEnrichmentPromptBudgetWithin
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128:
		return productEnrichmentPromptBudgetWithin
	case reflect.Interface:
		if value.IsNil() {
			return productEnrichmentPromptBudgetWithin
		}
		return w.visit(value.Elem(), depth+1)
	case reflect.Pointer:
		if value.IsNil() {
			return productEnrichmentPromptBudgetWithin
		}
		return w.visitReference(value, func() productEnrichmentPromptBudgetKind {
			return w.visit(value.Elem(), depth+1)
		})
	case reflect.Struct:
		// time.Time is a fixed-shape JSON scalar. Traversing its unexported
		// time.Location implementation graph would measure runtime internals,
		// not provider-neutral request data.
		if value.Type() == productEnrichmentPromptTimeType {
			return productEnrichmentPromptBudgetWithin
		}
		for index := 0; index < value.NumField(); index++ {
			if kind := w.visit(value.Field(index), depth+1); kind != productEnrichmentPromptBudgetWithin {
				return kind
			}
		}
		return productEnrichmentPromptBudgetWithin
	case reflect.Slice:
		if value.IsNil() {
			return productEnrichmentPromptBudgetWithin
		}
		return w.visitReference(value, func() productEnrichmentPromptBudgetKind {
			for index := 0; index < value.Len(); index++ {
				if kind := w.visit(value.Index(index), depth+1); kind != productEnrichmentPromptBudgetWithin {
					return kind
				}
			}
			return productEnrichmentPromptBudgetWithin
		})
	case reflect.Array:
		for index := 0; index < value.Len(); index++ {
			if kind := w.visit(value.Index(index), depth+1); kind != productEnrichmentPromptBudgetWithin {
				return kind
			}
		}
		return productEnrichmentPromptBudgetWithin
	case reflect.Map:
		if value.IsNil() {
			return productEnrichmentPromptBudgetWithin
		}
		return w.visitReference(value, func() productEnrichmentPromptBudgetKind {
			iterator := value.MapRange()
			for iterator.Next() {
				if kind := w.visit(iterator.Key(), depth+1); kind != productEnrichmentPromptBudgetWithin {
					return kind
				}
				if kind := w.visit(iterator.Value(), depth+1); kind != productEnrichmentPromptBudgetWithin {
					return kind
				}
			}
			return productEnrichmentPromptBudgetWithin
		})
	default:
		return productEnrichmentPromptBudgetUnsupported
	}
}

func (w *productEnrichmentPromptBudgetWalker) visitReference(
	value reflect.Value,
	visitChildren func() productEnrichmentPromptBudgetKind,
) productEnrichmentPromptBudgetKind {
	reference := productEnrichmentPromptBudgetReference{
		kind:    value.Kind(),
		typeOf:  value.Type(),
		pointer: value.Pointer(),
	}
	if _, cycle := w.active[reference]; cycle {
		return productEnrichmentPromptBudgetCycle
	}
	w.active[reference] = struct{}{}
	kind := visitChildren()
	delete(w.active, reference)
	return kind
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
