package quality

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestChecksFactsWithoutInventingProductInformation(t *testing.T) {
	in := Input{Name: "收纳盒", Dimensions: "20×10", Specifications: []Specification{{Name: "颜色", Value: "红"}, {Name: "颜色", Value: "蓝"}, {Name: "尺寸", Value: "10cm"}, {Name: "尺寸", Value: "10cm"}}}
	report, err := Check(in)
	require.NoError(t, err)
	codes := []string{}
	for _, v := range report.Findings {
		codes = append(codes, v.Code)
	}
	require.Contains(t, codes, "MISSING_MATERIAL")
	require.Contains(t, codes, "MISSING_DESCRIPTION")
	require.Contains(t, codes, "DIMENSION_UNIT_UNSPECIFIED")
	require.Contains(t, codes, "CONFLICTING_SPECIFICATION")
	require.Contains(t, codes, "DUPLICATE_SPECIFICATION")
	require.Equal(t, "product-quality-rules-v1", report.RuleVersion)
	complete := Input{Name: "收纳盒", Material: "塑料", Dimensions: "20×10 cm", Description: "用于收纳文具", Specifications: []Specification{{Name: "颜色", Value: "红"}}}
	report, err = Check(complete)
	require.NoError(t, err)
	require.Empty(t, report.Findings)
	require.NotContains(t, report.Summary, "合规")
}
func TestRejectsUnboundedAndInvalidInputButReportsMissingFields(t *testing.T) {
	for _, in := range []Input{{}, {Name: strings.Repeat("名", 121)}, {Name: "x", Description: "\x00"}, {Name: "x", Specifications: make([]Specification, 21)}, {Name: "  x"}} {
		_, err := Check(in)
		require.Error(t, err)
	}
	report, err := Check(Input{Description: "仅提供描述"})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(report.Findings), 4)
}
