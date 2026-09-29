package httpapi

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"task-processor/internal/agentconfig"
	"testing"
)

func TestProductAgentTemplateSelectionIsExplicitAndStartOnly(t *testing.T) {
	b := productAgentRequestBody{}
	ref, e := b.templateSelection("start")
	require.NoError(t, e)
	require.Nil(t, ref)
	b.TemplateSelection = json.RawMessage(`{"templateId":"8fc227bb-b572-4138-8e2a-5f1a0be98617","revision":"3"}`)
	ref, e = b.templateSelection("start")
	require.NoError(t, e)
	require.Equal(t, &agentconfig.TemplateRef{TemplateID: "8fc227bb-b572-4138-8e2a-5f1a0be98617", Revision: "3"}, ref)
	_, e = b.templateSelection("resume")
	require.Error(t, e)
	for _, raw := range []string{`null`, `{}`, `{"templateId":"8fc227bb-b572-4138-8e2a-5f1a0be98617","revision":3}`, `{"templateId":"8fc227bb-b572-4138-8e2a-5f1a0be98617","revision":"03"}`, `{"templateId":"8fc227bb-b572-4138-8e2a-5f1a0be98617","revision":"1","revision":"2"}`, `{"templateId":"8fc227bb-b572-4138-8e2a-5f1a0be98617","revision":"1","organizationId":"other"}`} {
		b.TemplateSelection = json.RawMessage(raw)
		_, e = b.templateSelection("start")
		require.Error(t, e, raw)
	}
}
