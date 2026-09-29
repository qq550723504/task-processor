package httpapi

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"testing"
)

func TestProductAgentKnowledgeSelectionIsStrictAndOptional(t *testing.T) {
	for _, body := range []string{
		`{"targetPlatform":"shein"}`,
		`{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"11111111-1111-4111-8111-111111111111"}}`,
	} {
		var input productAgentRequestBody
		require.NoError(t, json.Unmarshal([]byte(body), &input))
		selection, err := input.knowledgeSelection("start")
		require.NoError(t, err)
		if len(input.KnowledgeSelection) == 0 {
			require.Empty(t, selection)
		} else {
			require.Equal(t, "11111111-1111-4111-8111-111111111111", selection)
		}
	}
	for _, raw := range []string{`null`, `{}`, `{"knowledgeBaseId":""}`, `{"knowledgeBaseId":"not-canonical"}`, `{"knowledgeBaseId":"00000000-0000-0000-0000-000000000000"}`, `{"knowledgeBaseId":"11111111-1111-4111-8111-111111111111","sourceIds":[]}`, `{"knowledgeBaseId":"11111111-1111-4111-8111-111111111111","knowledgeBaseId":"22222222-2222-4222-8222-222222222222"}`} {
		t.Run(raw, func(t *testing.T) {
			input := productAgentRequestBody{KnowledgeSelection: json.RawMessage(raw)}
			_, err := input.knowledgeSelection("start")
			require.ErrorIs(t, err, agent.ErrInvalid)
		})
	}
	for _, action := range []string{"read", "resume", "review"} {
		input := productAgentRequestBody{KnowledgeSelection: json.RawMessage(`null`)}
		_, err := input.knowledgeSelection(action)
		require.ErrorIs(t, err, agent.ErrInvalid)
	}
}
