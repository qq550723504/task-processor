package agentconfig

import (
	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"testing"
	"time"
)

func TestFrozenLimitsCannotGrowOrSurviveTightening(t *testing.T) {
	frozen := agent.Limits{Steps: 8, ModelCalls: 3, Tokens: 100, CostMicros: 100, Currency: "CNY", Runtime: time.Minute}
	ceiling := frozen
	require.True(t, LimitsAdmissible(frozen, ceiling))
	ceiling.Tokens++
	require.True(t, LimitsAdmissible(frozen, ceiling))
	require.EqualValues(t, 100, frozen.Tokens)
	ceiling.ModelCalls--
	require.False(t, LimitsAdmissible(frozen, ceiling))
	ceiling = frozen
	ceiling.Currency = "USD"
	require.False(t, LimitsAdmissible(frozen, ceiling))
}

func TestTypedTemplateValidation(t *testing.T) {
	require.True(t, TemplateInput{Name: " 标题模板 ", TargetPlatform: "shein"}.Valid())
	for _, input := range []TemplateInput{{Name: "", TargetPlatform: "shein"}, {Name: "bad\nname", TargetPlatform: "shein"}, {Name: "ok", TargetPlatform: "unknown"}, {Name: "ok", TargetPlatform: "amazon", DefaultKnowledgeBaseID: "not-a-uuid"}} {
		require.False(t, input.Valid())
	}
}
