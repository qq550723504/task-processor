package agentcustomization

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPrivateDeliveryRequiresDeliveredStageAndConfirmedProposal(t *testing.T) {
	for _, r := range []Request{{Stage: Developing}, {Stage: Developing, Proposal: "方案"}, {Stage: Proposed, Proposal: "方案", OfflineConfirmation: "确认"}} {
		u := Update{Stage: r.Stage, Note: "发布", DeliverQualityAgent: true}
		require.Error(t, ApplyUpdate(&r, u, true))
	}
	r := Request{Stage: Developing, Proposal: "方案", OfflineConfirmation: "确认"}
	require.NoError(t, ApplyUpdate(&r, Update{Stage: Delivered, Note: "交付质检智能体", DeliverQualityAgent: true}, true))
	r.Stage = Delivered
	require.NoError(t, ApplyUpdate(&r, Update{Stage: Delivered, Note: "追加使用说明", DeliverQualityAgent: true}, true))
}
