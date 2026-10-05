package googleinteractions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These are the still-valid #586 qualification fixtures extracted from the
// retired Manager facade. They now exercise the current raw protocol mapper.
func TestObserveGoogleUsageQualification(t *testing.T) {
	for _, tc := range []struct {
		name, usage string
		known       bool
	}{
		{"text", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[{"modality":"text","tokens":3}],"output_tokens_by_modality":[{"modality":"text","tokens":4}]}`, true},
		{"text cache", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"total_cached_tokens":1,"cached_tokens_by_modality":[{"modality":"text","tokens":1}]}`, true},
		{"missing thoughts", `{"total_input_tokens":3,"total_output_tokens":4,"total_tokens":7,"total_tool_use_tokens":0}`, false},
		{"missing tool counter", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14}`, false},
		{"inconsistent", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":13,"total_tool_use_tokens":0}`, false},
		{"tool use", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":1}`, false},
		{"tool modality despite zero", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"tool_use_tokens_by_modality":[{"modality":"text","tokens":1}]}`, false},
		{"image input", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[{"modality":"text","tokens":2},{"modality":"image","tokens":1}]}`, false},
		{"audio output", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"output_tokens_by_modality":[{"modality":"text","tokens":3},{"modality":"audio","tokens":1}]}`, false},
		{"image cache", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"total_cached_tokens":1,"cached_tokens_by_modality":[{"modality":"image","tokens":1}]}`, false},
		{"inconsistent text breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[{"modality":"text","tokens":2}]}`, false},
		{"empty input breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[]}`, false},
		{"empty output breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"output_tokens_by_modality":[]}`, false},
		{"empty cache breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"total_cached_tokens":1,"cached_tokens_by_modality":[]}`, false},
		{"unpriced dimension", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"total_other_tokens":1}`, false},
		{"over cap", `{"total_input_tokens":3,"total_output_tokens":120,"total_thought_tokens":9,"total_tokens":132,"total_tool_use_tokens":0}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"id":"interaction-1","model":"gemini-3.8-flash","status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"ok"}]}],"usage":` + tc.usage + `}`
			result := Observe([]byte(raw), 128)
			require.Equal(t, tc.known, result.UsageKnown)
			if tc.known {
				require.Equal(t, 3, result.PromptTokens)
				require.Equal(t, 11, result.CompletionTokens)
				require.Equal(t, 14, result.TotalTokens)
				require.Equal(t, "stop", result.FinishReason)
			} else {
				require.NotEmpty(t, result.Diagnostic)
			}
		})
	}
}

func TestSafeProviderRequestReference(t *testing.T) {
	require.Equal(t, "interaction-1:abc", SafeRequestID("interaction-1:abc"))
	for _, value := range []string{"", "https://provider.test/private", "id\nsecret", "思考", strings.Repeat("a", 129)} {
		require.Empty(t, SafeRequestID(value))
	}
}

func TestTopLevelGoogleMediaCannotBePricedAsText(t *testing.T) {
	for _, field := range []string{"output_image", "output_audio", "output_video"} {
		for _, text := range []bool{false, true} {
			name := field + "/media only"
			steps := `[]`
			if text {
				name = field + "/mixed text"
				steps = `[{"type":"model_output","content":[{"type":"text","text":"ok"}]}]`
			}
			t.Run(name, func(t *testing.T) {
				raw := `{"id":"interaction-1","model":"gemini-3.8-flash","status":"completed","steps":` + steps + `,"` + field + `":{"uri":"https://example.test/media","mime_type":"application/octet-stream"},"usage":{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0}}`
				result := Observe([]byte(raw), 128)
				require.False(t, result.UsageKnown, "top-level media cannot settle at text rates")
				require.Empty(t, result.Content)
			})
		}
	}
}
