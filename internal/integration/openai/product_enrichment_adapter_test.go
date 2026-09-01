package openai

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/sourcing"
)

func TestProductEnrichmentAdapterMapsStrictJSONToEvidenceBackedChanges(t *testing.T) {
	t.Parallel()

	invoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle","title":"Insulated Bottle"}`}
	adapter := NewProductEnrichmentAdapter(invoker)
	request := validEnrichmentGenerationRequest()
	request.Policy.AllowedFields = []string{"title", "description"}
	request.Policy.RequiredFields = []string{"description"}
	before := mustMarshalEnrichmentRequest(t, request)

	got, err := adapter.Generate(context.Background(), request)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	want := enrichment.Candidate{Changes: []enrichment.FieldChange{
		{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}},
		{Field: "title", Value: "Insulated Bottle", EvidenceIDs: []string{"raw-1"}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Generate() candidate = %#v, want %#v", got, want)
	}
	if after := mustMarshalEnrichmentRequest(t, request); !reflect.DeepEqual(after, before) {
		t.Fatalf("Generate() mutated request\nbefore: %s\nafter:  %s", before, after)
	}
	if invoker.output != `{"description":"Steel bottle","title":"Insulated Bottle"}` {
		t.Fatalf("Generate() mutated invoker response = %q", invoker.output)
	}
}

func TestProductEnrichmentAdapterUsesCanonicalRawEvidenceFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reference sourcing.RawSourceReference
		wantID    string
	}{
		{
			name:      "reference id wins",
			reference: sourcing.RawSourceReference{ReferenceID: "reference-1", SnapshotID: "snapshot-1", Checksum: "sha256:one"},
			wantID:    "reference-1",
		},
		{
			name:      "snapshot id fallback",
			reference: sourcing.RawSourceReference{SnapshotID: "snapshot-1", Checksum: "sha256:one"},
			wantID:    "snapshot-1",
		},
		{
			name:      "checksum fallback",
			reference: sourcing.RawSourceReference{Checksum: "sha256:one"},
			wantID:    "sha256:one",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validEnrichmentGenerationRequest()
			request.Source.RawReference = tt.reference
			adapter := NewProductEnrichmentAdapter(&enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`})

			got, err := adapter.Generate(context.Background(), request)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			if gotID := got.Changes[0].EvidenceIDs[0]; gotID != tt.wantID {
				t.Fatalf("Generate() evidence ID = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func TestProductEnrichmentAdapterMapsInvocationFailureWithoutLeakingCause(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("openai sdk request failed")
	adapter := NewProductEnrichmentAdapter(&enrichmentTextInvokerStub{err: providerErr})

	_, err := adapter.Generate(context.Background(), validEnrichmentGenerationRequest())
	if err != enrichment.ErrExternalCapabilityUnavailable {
		t.Fatalf("Generate() error = %v, want ErrExternalCapabilityUnavailable", err)
	}
	if errors.Is(err, providerErr) {
		t.Fatalf("Generate() error exposes invocation cause %v", providerErr)
	}
}

func TestProductEnrichmentAdapterRejectsTypedNilInvoker(t *testing.T) {
	t.Parallel()

	var invoker *enrichmentTextInvokerStub
	adapter := NewProductEnrichmentAdapter(invoker)

	_, err := adapter.Generate(context.Background(), validEnrichmentGenerationRequest())
	if err != enrichment.ErrExternalCapabilityUnavailable {
		t.Fatalf("Generate() error = %v, want ErrExternalCapabilityUnavailable", err)
	}
}

func TestProductEnrichmentAdapterPreservesContextAndCancellation(t *testing.T) {
	t.Parallel()

	type contextKey string
	const key contextKey = "trace"
	deadline := time.Now().Add(time.Hour).Round(0)
	ctx, cancelDeadline := context.WithDeadline(context.WithValue(context.Background(), key, "trace-7"), deadline)
	defer cancelDeadline()
	invoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}
	adapter := NewProductEnrichmentAdapter(invoker)

	if _, err := adapter.Generate(ctx, validEnrichmentGenerationRequest()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := invoker.contextValue(key); got != "trace-7" {
		t.Fatalf("invocation context value = %v, want trace-7", got)
	}
	if invoker.ctx != ctx {
		t.Fatal("adapter replaced the caller context")
	}
	if got, ok := invoker.ctx.Deadline(); !ok || !got.Equal(deadline) {
		t.Fatalf("invocation deadline = %v, %v; want %v, true", got, ok, deadline)
	}
	if invoker.request.MaxOutputBytes != productEnrichmentOutputMaxBytes {
		t.Fatalf("MaxOutputBytes = %d, want %d", invoker.request.MaxOutputBytes, productEnrichmentOutputMaxBytes)
	}
	if len(invoker.request.Prompt) > productEnrichmentPromptMaxBytes {
		t.Fatalf("prompt bytes = %d, limit = %d", len(invoker.request.Prompt), productEnrichmentPromptMaxBytes)
	}
	if strings.HasSuffix(invoker.request.Prompt, "\n") {
		t.Fatal("streaming JSON encoder newline leaked into provider prompt")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	beforeCalls := invoker.calls
	if _, err := adapter.Generate(canceled, validEnrichmentGenerationRequest()); err != context.Canceled {
		t.Fatalf("Generate(canceled) error = %v, want context.Canceled", err)
	}
	if invoker.calls != beforeCalls {
		t.Fatalf("invocation calls after pre-cancellation = %d, want %d", invoker.calls, beforeCalls)
	}

	cancelDuringCall, cancelDuringCallFunc := context.WithCancel(context.Background())
	cancelingInvoker := &enrichmentTextInvokerStub{
		output: `{"description":"Steel bottle"}`,
		onGenerate: func() {
			cancelDuringCallFunc()
		},
	}
	if _, err := NewProductEnrichmentAdapter(cancelingInvoker).Generate(cancelDuringCall, validEnrichmentGenerationRequest()); err != context.Canceled {
		t.Fatalf("Generate(canceled during call) error = %v, want context.Canceled", err)
	}
}

func TestProductEnrichmentAdapterDoesNotInvokeAfterPreparationCancellation(t *testing.T) {
	t.Parallel()

	ctx := &successiveEnrichmentErrContext{value: "trace-before-provider"}
	invoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}

	got, err := NewProductEnrichmentAdapter(invoker).Generate(ctx, validEnrichmentGenerationRequest())
	if err != context.Canceled {
		t.Fatalf("Generate() error = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(got, enrichment.Candidate{}) {
		t.Fatalf("Generate() candidate = %#v, want zero candidate", got)
	}
	if invoker.calls != 0 {
		t.Fatalf("invocation calls = %d, want 0 after preparation cancellation", invoker.calls)
	}
	if ctx.errCalls != 2 {
		t.Fatalf("context Err calls = %d, want initial and pre-invocation checks", ctx.errCalls)
	}
}

func TestProductEnrichmentPromptPreflightRejectsHugeStringWithoutMarshal(t *testing.T) {
	t.Parallel()

	request := validEnrichmentGenerationRequest()
	request.Snapshot.Description = strings.Repeat("x", productEnrichmentPromptMaxRawStringBytes+1)
	assertProductEnrichmentPromptPreflightRejectsWithoutMarshal(t, request, productEnrichmentPromptBudgetRawStringBytes)

	invoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}
	got, err := NewProductEnrichmentAdapter(invoker).Generate(context.Background(), request)
	if err != enrichment.ErrInputInvalid || !reflect.DeepEqual(got, enrichment.Candidate{}) || invoker.calls != 0 {
		t.Fatalf("Generate(huge string) = (%#v, %v, calls %d), want (zero, ErrInputInvalid, calls 0)", got, err, invoker.calls)
	}
}

func TestProductEnrichmentPromptPreflightRejectsExcessiveCollectionNodesWithoutMarshal(t *testing.T) {
	t.Parallel()

	request := validEnrichmentGenerationRequest()
	request.Snapshot.SellingPoints = make([]string, productEnrichmentPromptMaxNodes+1)
	assertProductEnrichmentPromptPreflightRejectsWithoutMarshal(t, request, productEnrichmentPromptBudgetNodes)

	invoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}
	got, err := NewProductEnrichmentAdapter(invoker).Generate(context.Background(), request)
	if err != enrichment.ErrInputInvalid || !reflect.DeepEqual(got, enrichment.Candidate{}) || invoker.calls != 0 {
		t.Fatalf("Generate(excess nodes) = (%#v, %v, calls %d), want (zero, ErrInputInvalid, calls 0)", got, err, invoker.calls)
	}
}

func TestProductEnrichmentPromptPreflightAcceptsSamePointerDifferentLengthSlice(t *testing.T) {
	t.Parallel()

	value := make([]any, 1)
	value[0] = value[:0]
	budget := inspectProductEnrichmentPromptBudget(value)
	if budget.Kind != productEnrichmentPromptBudgetWithin {
		t.Fatalf("same-pointer empty subslice budget = %#v, want within budget", budget)
	}
	if budget.Nodes != 3 || budget.RawStringBytes != 0 {
		t.Fatalf("same-pointer empty subslice budget = %#v, want 3 nodes and 0 raw bytes", budget)
	}
}

func TestProductEnrichmentPromptPreflightRejectsTrueSliceCycle(t *testing.T) {
	t.Parallel()

	value := make([]any, 1)
	value[0] = value
	budget := inspectProductEnrichmentPromptBudget(value)
	if budget.Kind != productEnrichmentPromptBudgetCycle {
		t.Fatalf("true slice cycle budget kind = %q, want %q", budget.Kind, productEnrichmentPromptBudgetCycle)
	}
}

func TestProductEnrichmentPromptPreflightRejectsPointerCycleFailClosed(t *testing.T) {
	t.Parallel()

	type cyclicValue struct{ Next *cyclicValue }
	value := &cyclicValue{}
	value.Next = value
	budget := inspectProductEnrichmentPromptBudget(value)
	if budget.Kind != productEnrichmentPromptBudgetCycle {
		t.Fatalf("cycle budget kind = %q, want %q", budget.Kind, productEnrichmentPromptBudgetCycle)
	}
}

func TestProductEnrichmentPromptPreflightAcceptsCompletedSharedReferences(t *testing.T) {
	t.Parallel()

	type sharedValue struct{ Text string }
	sharedPointer := &sharedValue{Text: "pointer"}
	sharedMap := map[string]string{"map-key": "map-value"}
	sharedSlice := []string{"slice-value"}
	value := struct {
		PointerA *sharedValue
		PointerB *sharedValue
		MapA     map[string]string
		MapB     map[string]string
		SliceA   []string
		SliceB   []string
	}{
		PointerA: sharedPointer,
		PointerB: sharedPointer,
		MapA:     sharedMap,
		MapB:     sharedMap,
		SliceA:   sharedSlice,
		SliceB:   sharedSlice,
	}

	budget := inspectProductEnrichmentPromptBudget(value)
	if budget.Kind != productEnrichmentPromptBudgetWithin {
		t.Fatalf("completed shared references budget = %#v, want within budget", budget)
	}
}

func TestProductEnrichmentPromptPreflightRejectsExcessiveDepthFailClosed(t *testing.T) {
	t.Parallel()

	type nestedValue struct{ Next *nestedValue }
	root := &nestedValue{}
	cursor := root
	for range productEnrichmentPromptMaxDepth + 1 {
		cursor.Next = &nestedValue{}
		cursor = cursor.Next
	}
	budget := inspectProductEnrichmentPromptBudget(root)
	if budget.Kind != productEnrichmentPromptBudgetDepth {
		t.Fatalf("depth budget kind = %q, want %q", budget.Kind, productEnrichmentPromptBudgetDepth)
	}
}

func TestProductEnrichmentAdapterEnforcesSerializedPromptByteLimit(t *testing.T) {
	t.Parallel()

	const (
		exactPromptBytes     = 65536
		oversizedPromptBytes = 65537
	)
	exactRequest, exactOracle := enrichmentRequestAndOracleForPromptBytes(t, exactPromptBytes)
	if len(exactOracle) != exactPromptBytes {
		t.Fatalf("exact oracle bytes = %d, want %d", len(exactOracle), exactPromptBytes)
	}
	if budget := inspectProductEnrichmentPromptBudget(exactRequest); budget.Kind != productEnrichmentPromptBudgetWithin {
		t.Fatalf("exact prompt preflight = %#v, want within budget", budget)
	}
	exactInvoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}
	if _, err := NewProductEnrichmentAdapter(exactInvoker).Generate(context.Background(), exactRequest); err != nil {
		t.Fatalf("Generate(exact prompt limit) error = %v", err)
	}
	if got := len(exactInvoker.request.Prompt); got != exactPromptBytes {
		t.Fatalf("exact prompt bytes = %d, want %d", got, exactPromptBytes)
	}
	if exactInvoker.request.Prompt != string(exactOracle) {
		t.Fatal("captured exact-limit prompt differs from independent wire oracle")
	}

	overRequest, overOracle := enrichmentRequestAndOracleForPromptBytes(t, oversizedPromptBytes)
	if len(overOracle) != oversizedPromptBytes {
		t.Fatalf("limit+1 oracle bytes = %d, want %d", len(overOracle), oversizedPromptBytes)
	}
	overInvoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}
	got, err := NewProductEnrichmentAdapter(overInvoker).Generate(context.Background(), overRequest)
	if err != enrichment.ErrInputInvalid {
		t.Fatalf("Generate() error = %v, want ErrInputInvalid", err)
	}
	if !reflect.DeepEqual(got, enrichment.Candidate{}) {
		t.Fatalf("Generate() candidate = %#v, want zero candidate", got)
	}
	if overInvoker.calls != 0 {
		t.Fatalf("invocation calls = %d, want 0 for oversized serialized prompt", overInvoker.calls)
	}
}

func TestProductEnrichmentSerializedPromptBoundaryFixtureIsIndependent(t *testing.T) {
	t.Parallel()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve product enrichment adapter test source")
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	if err != nil {
		t.Fatalf("parse product enrichment adapter test source: %v", err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "TestProductEnrichmentAdapterEnforcesSerializedPromptByteLimit" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if ok && identifier.Name == "productEnrichmentPromptMaxBytes" {
				t.Error("serialized prompt boundary fixture references production limit instead of test-local design literals")
			}
			return true
		})
		return
	}
	t.Fatal("serialized prompt boundary test function not found")
}

func TestProductEnrichmentPromptLimitMatchesDesignContract(t *testing.T) {
	t.Parallel()

	const designPromptLimitBytes = 65536
	if productEnrichmentPromptMaxBytes != designPromptLimitBytes {
		t.Fatalf("production prompt limit = %d, want design contract literal %d", productEnrichmentPromptMaxBytes, designPromptLimitBytes)
	}
}

func TestProductEnrichmentAdapterEnforcesProviderOutputByteLimit(t *testing.T) {
	t.Parallel()

	prefix := `{"description":"`
	suffix := `"}`
	exactValue := strings.Repeat("x", productEnrichmentOutputMaxBytes-len(prefix)-len(suffix))
	exactOutput := prefix + exactValue + suffix
	if len(exactOutput) != productEnrichmentOutputMaxBytes {
		t.Fatalf("exact output fixture bytes = %d, want %d", len(exactOutput), productEnrichmentOutputMaxBytes)
	}
	exact, err := NewProductEnrichmentAdapter(&enrichmentTextInvokerStub{output: exactOutput}).Generate(context.Background(), validEnrichmentGenerationRequest())
	if err != nil {
		t.Fatalf("Generate(exact limit) error = %v", err)
	}
	if got := exact.Changes[0].Value; got != exactValue {
		t.Fatalf("Generate(exact limit) value bytes = %d, want %d", len(got), len(exactValue))
	}

	oversizedOutput := prefix + exactValue + "x" + suffix
	got, err := NewProductEnrichmentAdapter(&enrichmentTextInvokerStub{output: oversizedOutput}).Generate(context.Background(), validEnrichmentGenerationRequest())
	if err != enrichment.ErrOutputValidation {
		t.Fatalf("Generate(limit+1) error = %v, want ErrOutputValidation", err)
	}
	if !reflect.DeepEqual(got, enrichment.Candidate{}) {
		t.Fatalf("Generate(limit+1) candidate = %#v, want zero candidate", got)
	}
}

func TestProductEnrichmentAdapterRejectsInvalidOrUnsupportedOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
	}{
		{name: "malformed JSON", output: `{"description":`},
		{name: "trailing JSON", output: `{"description":"Steel bottle"} {}`},
		{name: "non object", output: `["Steel bottle"]`},
		{name: "non string value", output: `{"description":{"text":"Steel bottle"}}`},
		{name: "unsupported field", output: `{"provider_score":"90"}`},
		{name: "empty object", output: `{}`},
		{name: "blank value", output: `{"description":" "}`},
		{name: "duplicate field", output: `{"description":"one","description":"two"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewProductEnrichmentAdapter(&enrichmentTextInvokerStub{output: tt.output})
			got, err := adapter.Generate(context.Background(), validEnrichmentGenerationRequest())
			if err != enrichment.ErrOutputValidation {
				t.Fatalf("Generate() error = %v, want ErrOutputValidation", err)
			}
			if !reflect.DeepEqual(got, enrichment.Candidate{}) {
				t.Fatalf("Generate() candidate = %#v, want zero candidate", got)
			}
		})
	}
}

func TestProductEnrichmentAdapterRejectsMissingRawEvidenceBeforeInvocation(t *testing.T) {
	t.Parallel()

	request := validEnrichmentGenerationRequest()
	request.Source.RawReference = sourcing.RawSourceReference{}
	invoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}

	_, err := NewProductEnrichmentAdapter(invoker).Generate(context.Background(), request)
	if err != enrichment.ErrEvidenceInsufficient {
		t.Fatalf("Generate() error = %v, want ErrEvidenceInsufficient", err)
	}
	if invoker.calls != 0 {
		t.Fatalf("invocation calls = %d, want 0 without canonical raw evidence", invoker.calls)
	}
}

func TestProductEnrichmentAdapterBuildsDeterministicRequestFromDomainFacts(t *testing.T) {
	t.Parallel()

	first := validEnrichmentGenerationRequest()
	first.Policy.AllowedFields = []string{"title", "description"}
	first.Source.RawReference.Metadata = map[string]string{"z": "last", "a": "first"}
	first.Source.ProductCandidate.Attributes = map[string]string{"material": "steel", "finish": "matte"}
	second := validEnrichmentGenerationRequest()
	second.Policy.AllowedFields = []string{"description", "title"}
	second.Source.RawReference.Metadata = map[string]string{"a": "first", "z": "last"}
	second.Source.ProductCandidate.Attributes = map[string]string{"finish": "matte", "material": "steel"}
	firstInvoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}
	secondInvoker := &enrichmentTextInvokerStub{output: `{"description":"Steel bottle"}`}

	if _, err := NewProductEnrichmentAdapter(firstInvoker).Generate(context.Background(), first); err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}
	if _, err := NewProductEnrichmentAdapter(secondInvoker).Generate(context.Background(), second); err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	if firstInvoker.request.Prompt != secondInvoker.request.Prompt {
		t.Fatalf("semantically equal requests produced different prompts\nfirst:  %s\nsecond: %s", firstInvoker.request.Prompt, secondInvoker.request.Prompt)
	}
}

func validEnrichmentGenerationRequest() enrichment.GenerationRequest {
	return enrichment.GenerationRequest{
		Snapshot: catalog.ProductSnapshot{
			Title:       "Bottle",
			Brand:       "Acme",
			Description: "Insulated bottle",
			Attributes:  []catalog.Attribute{{Name: "material", Value: "steel"}},
		},
		Source: sourcing.SourceEnvelope{
			Identity: sourcing.SourceIdentity{
				SourceType:     sourcing.SourceTypeCrawler,
				SourcePlatform: "1688",
				SourceID:       "B001",
			},
			RawReference: sourcing.RawSourceReference{ReferenceID: "raw-1"},
			ProductCandidate: sourcing.ProductCandidate{
				Title:      "Bottle",
				Attributes: map[string]string{"material": "steel"},
			},
		},
		Policy: enrichment.PolicySnapshot{
			Version:             "v1",
			AllowedFields:       []string{"description"},
			RequiredFields:      []string{"description"},
			MinimumQualityScore: 80,
		},
	}
}

func assertProductEnrichmentPromptPreflightRejectsWithoutMarshal(
	t *testing.T,
	request enrichment.GenerationRequest,
	wantKind productEnrichmentPromptBudgetKind,
) {
	t.Helper()

	budget := inspectProductEnrichmentPromptBudget(request)
	if budget.Kind != wantKind {
		t.Fatalf("preflight budget kind = %q, want %q (raw bytes %d, nodes %d)", budget.Kind, wantKind, budget.RawStringBytes, budget.Nodes)
	}
	requestedFields, err := canonicalRequestedFields(request.Policy)
	if err != nil {
		t.Fatalf("canonicalRequestedFields() error = %v", err)
	}
	evidenceID, err := enrichment.CanonicalEvidenceID(request.Source)
	if err != nil {
		t.Fatalf("CanonicalEvidenceID() error = %v", err)
	}
	marshalCalls := 0
	_, err = buildProductEnrichmentPromptWithMarshal(request, evidenceID, requestedFields, func(any) ([]byte, error) {
		marshalCalls++
		return nil, errors.New("marshal must not be called")
	})
	if err == nil {
		t.Fatal("buildProductEnrichmentPromptWithMarshal() error = nil, want preflight rejection")
	}
	if marshalCalls != 0 {
		t.Fatalf("marshal calls = %d, want 0 after preflight rejection", marshalCalls)
	}
}

type productEnrichmentPromptWireOracle struct {
	Instruction     string                       `json:"instruction"`
	RequestedFields []string                     `json:"requested_fields"`
	EvidenceID      string                       `json:"evidence_id"`
	Request         enrichment.GenerationRequest `json:"request"`
}

func enrichmentRequestAndOracleForPromptBytes(t *testing.T, targetBytes int) (enrichment.GenerationRequest, []byte) {
	t.Helper()

	request := validEnrichmentGenerationRequest()
	request.Snapshot.Description = "x"
	baseline := marshalProductEnrichmentPromptWireOracle(t, request)
	paddingBytes := targetBytes - len(baseline)
	if paddingBytes < 0 {
		t.Fatalf("target prompt bytes %d smaller than baseline %d", targetBytes, len(baseline))
	}
	request.Snapshot.Description = strings.Repeat("x", paddingBytes+1)
	return request, marshalProductEnrichmentPromptWireOracle(t, request)
}

func marshalProductEnrichmentPromptWireOracle(t *testing.T, request enrichment.GenerationRequest) []byte {
	t.Helper()

	request.Policy.AllowedFields = append([]string(nil), request.Policy.AllowedFields...)
	request.Policy.RequiredFields = append([]string(nil), request.Policy.RequiredFields...)
	sort.Strings(request.Policy.AllowedFields)
	sort.Strings(request.Policy.RequiredFields)
	wire := productEnrichmentPromptWireOracle{
		Instruction:     "Return exactly one JSON object. Use only requested field paths as keys and non-empty strings as values. Return no markdown, metadata, scores, or explanations.",
		RequestedFields: []string{"description"},
		EvidenceID:      "raw-1",
		Request:         request,
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal(test-owned product enrichment wire): %v", err)
	}
	return encoded
}

func mustMarshalEnrichmentRequest(t *testing.T, request enrichment.GenerationRequest) []byte {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(request): %v", err)
	}
	return data
}

type enrichmentTextInvokerStub struct {
	output     string
	err        error
	request    TextInvocationRequest
	ctx        context.Context
	calls      int
	onGenerate func()
}

func (s *enrichmentTextInvokerStub) Generate(ctx context.Context, request TextInvocationRequest) (string, error) {
	s.calls++
	s.ctx = ctx
	s.request = request
	if s.onGenerate != nil {
		s.onGenerate()
	}
	return s.output, s.err
}

type successiveEnrichmentErrContext struct {
	errCalls int
	value    any
}

func (*successiveEnrichmentErrContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*successiveEnrichmentErrContext) Done() <-chan struct{}       { return nil }
func (c *successiveEnrichmentErrContext) Err() error {
	c.errCalls++
	if c.errCalls > 1 {
		return context.Canceled
	}
	return nil
}
func (c *successiveEnrichmentErrContext) Value(any) any { return c.value }

func (s *enrichmentTextInvokerStub) contextValue(key any) any {
	if s.ctx == nil {
		return nil
	}
	return s.ctx.Value(key)
}
