package einoruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/agent"
)

const contextTestID = "11111111-1111-4111-8111-111111111111"
const contextTestCitationID = "22222222-2222-4222-8222-222222222222"

// Exercise the durable JSON contract before its Go fields exist. Dropping an
// opaque ref on decode must never turn a Knowledge run into an ordinary run.
func contextTestRequest(t *testing.T, request agent.Request, digest string) agent.Request {
	t.Helper()
	raw, _ := json.Marshal(request)
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	wire["ContextSnapshotRef"] = map[string]string{"kind": "knowledge", "id": contextTestID, "digest": digest}
	raw, _ = json.Marshal(wire)
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	return request
}

type contextTestModel struct {
	model  *fakeModel
	inputs []string
}

func (m *contextTestModel) Quote(ctx context.Context, in agent.ModelInput) (agent.Quote, error) {
	raw, _ := json.Marshal(in)
	m.inputs = append(m.inputs, string(raw))
	return m.model.Quote(ctx, in)
}
func (m *contextTestModel) Decide(ctx context.Context, in agent.ModelInput) (agent.ModelResult, error) {
	result, err := m.model.Decide(ctx, in)
	if err != nil {
		return result, err
	}
	raw, _ := json.Marshal(result)
	var wire map[string]any
	_ = json.Unmarshal(raw, &wire)
	wire["ContextCitationRefs"] = []any{map[string]any{
		"Snapshot": map[string]string{"kind": "knowledge", "id": contextTestID, "digest": strings.Repeat("a", 64)},
		"ID":       contextTestCitationID,
	}}
	raw, _ = json.Marshal(wire)
	_ = json.Unmarshal(raw, &result)
	return result, nil
}

func TestContextRefReachesModelAndCitationSurvivesDurableRun(t *testing.T) {
	runtime, request, model, _, _, _, _ := fixture(t, proposal("supported title"))
	wrapped := &contextTestModel{model: model}
	runtime.config.Model = wrapped
	request = contextTestRequest(t, request, strings.Repeat("a", 64))
	record, err := runtime.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(record)
	if record.State.Phase != agent.HumanReviewRequired || len(wrapped.inputs) != 1 ||
		!strings.Contains(wrapped.inputs[0], contextTestID) || !strings.Contains(string(raw), contextTestCitationID) {
		t.Fatal("opaque context/citation was dropped between request, model and durable run")
	}
}

func TestContextRefIsPartOfCompleteStartIdentity(t *testing.T) {
	runtime, request, model, _, _, _, _ := fixture(t, proposal("supported title"))
	request = contextTestRequest(t, request, strings.Repeat("a", 64))
	first, err := runtime.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := runtime.Start(context.Background(), request)
	if err != nil || replayed.State.RunID != first.State.RunID || model.calls != 1 {
		t.Fatal("same-context retry dispatched again", err)
	}
	changed := contextTestRequest(t, request, strings.Repeat("b", 64))
	if _, err = runtime.Start(context.Background(), changed); !errors.Is(err, agent.ErrConflict) {
		t.Fatal("changed context under the same Start key was adopted", err)
	}
}

func TestPartialContextRefRejectedBeforeStoreClaim(t *testing.T) {
	runtime, request, _, _, _, _, store := fixture(t, proposal("supported title"))
	request = contextTestRequest(t, request, "")
	_, err := runtime.Start(context.Background(), request)
	if !errors.Is(err, agent.ErrInvalid) || store.exists {
		t.Fatal("partial context was silently discarded or durably admitted", err)
	}
}
