package agentcustomization

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func validInput() Input {
	return Input{Name: "标题优化", Scenario: "SHEIN商品维护", Direction: "PRODUCT_SUPPLY", Description: "希望形成可检查的标题建议", ContactName: "试用联系人", ContactMethod: "test-wechat", Consent: true}
}
func TestSubmissionRequiresConsentAndBoundedPrivateFiles(t *testing.T) {
	require.NoError(t, ValidateInput(validInput()))
	in := validInput()
	in.Consent = false
	require.ErrorIs(t, ValidateInput(in), ErrInvalid)
	in = validInput()
	in.Files = []Upload{{Name: "../x", Data: []byte("资料")}}
	require.ErrorIs(t, ValidateInput(in), ErrInvalid)
	in.Files = []Upload{{Name: "x.html", Data: []byte("<html><script>alert(1)</script></html>")}}
	require.ErrorIs(t, ValidateInput(in), ErrInvalid)
	in.Files = []Upload{{Name: "资料.csv", Data: []byte("名称,说明\n商品,标题")}}
	require.NoError(t, ValidateInput(in))
	in.Files = append(in.Files, Upload{Name: "a", Data: make([]byte, MaxFileBytes+1)})
	require.ErrorIs(t, ValidateInput(in), ErrInvalid)
}
func TestHumanProgressRequiresPlatformAndExplicitOfflineConfirmation(t *testing.T) {
	r := Request{Stage: Submitted, Version: "1"}
	u := Update{Stage: Evaluating, Note: "专员开始评估"}
	require.ErrorIs(t, ApplyUpdate(&r, u, false), ErrForbidden)
	require.NoError(t, ApplyUpdate(&r, u, true))
	require.Equal(t, Evaluating, r.Stage)
	u = Update{Stage: Developing, Note: "跳过方案"}
	require.ErrorIs(t, ApplyUpdate(&r, u, true), ErrConflict)
	u = Update{Stage: Proposed, Note: "已评估"}
	require.ErrorIs(t, ApplyUpdate(&r, u, true), ErrInvalid)
	u.Proposal = "范围：标题建议；周期：另行确认；报价：线下文档"
	require.NoError(t, ApplyUpdate(&r, u, true))
	u = Update{Stage: Developing, Note: "准备开发"}
	require.ErrorIs(t, ApplyUpdate(&r, u, true), ErrInvalid)
	u.OfflineConfirmation = "专员记录：双方在线下确认方案与费用"
	require.NoError(t, ApplyUpdate(&r, u, true))
	require.NoError(t, ApplyUpdate(&r, Update{Stage: Delivered, Note: "交付说明及使用入口已线下交接"}, true))
	require.ErrorIs(t, ApplyUpdate(&r, Update{Stage: Submitted, Note: "回退"}, true), ErrConflict)
}
