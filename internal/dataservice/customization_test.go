package dataservice

import (
	"github.com/stretchr/testify/require"
	"task-processor/internal/product/dataacquisition"
	"testing"
)

func TestCustomizationTransitionsRequireConfirmedSpecAndDeliveryReceipt(t *testing.T) {
	input := CustomInput{Name: "日本站商品数据", Query: dataacquisition.Query{Site: "jp", Mode: "keyword", Keyword: "水筒", Limit: 20}, Purpose: "选品", Format: "csv"}
	normalized, err := NormalizeCustomInput(input)
	require.NoError(t, err)
	require.NotEmpty(t, normalized.Query.Fields)
	for _, invalid := range []CustomInput{{Name: "bad", Query: input.Query, Format: "pdf"}, {Name: "bad", Query: input.Query, Format: "csv", Notes: string([]byte{0})}} {
		_, err := NormalizeCustomInput(invalid)
		require.ErrorIs(t, err, ErrInvalid)
	}
	require.ErrorIs(t, ValidateCustomChange(CustomRequest{State: "SUBMITTED"}, CustomPatch{State: "DELIVERED", Note: "button clicked"}), ErrConflict)
	require.ErrorIs(t, ValidateCustomChange(CustomRequest{State: "EVALUATING"}, CustomPatch{State: "SPEC_CONFIRMED", Note: "confirmed"}), ErrInvalid)
	spec := CustomSpec{Description: "20 rows", QuoteNote: "线下报价100元", ConfirmationNote: "客户线下确认", Format: "json", MaximumRows: 20}
	require.NoError(t, ValidateCustomChange(CustomRequest{State: "EVALUATING"}, CustomPatch{State: "SPEC_CONFIRMED", Note: "确认规格与报价", Spec: &spec}))
	require.ErrorIs(t, ValidateCustomChange(CustomRequest{State: "DELIVERED"}, CustomPatch{State: "CLOSED", Note: "overwrite"}), ErrConflict)
	require.ErrorIs(t, ValidateCustomChange(CustomRequest{State: "SUBMITTED"}, CustomPatch{State: "PREPARING", Note: "skip confirmation"}), ErrConflict)
	require.False(t, (DeliveryAuthority{}).Valid(), "a caller cannot manufacture delivery authority")
	require.NoError(t, ValidateCustomChange(CustomRequest{State: "EVALUATING"}, CustomPatch{State: "EVALUATING", Note: "补充评估进度"}))
	require.NoError(t, ValidateCustomChange(CustomRequest{State: "PREPARING", Spec: &CustomSpec{}, SpecRevision: 1}, CustomPatch{State: "PREPARING", Note: "补充制作进度"}))
}
