package imageagentapp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/catalog"
)

func TestImageProductEvidenceReadsExplicitCapturedAttributesWithoutInference(t *testing.T) {
	product := catalog.ProductSnapshot{Attributes: []catalog.Attribute{
		{Name: "规格尺寸", Value: "1024 × 1024 像素的虚拟测试卡"},
		{Name: "使用步骤", Value: "选择原图、确认计划、检查结果、批准素材"},
		{Name: "包装配件", Value: "虚拟测试卡，无实物包装配件"},
	}}
	evidence, err := ImageProductEvidence(product)
	require.NoError(t, err)
	require.Equal(t, product.Attributes[0].Value, evidence["specifications"])
	require.Equal(t, product.Attributes[1].Value, evidence["instructions"])
	require.Equal(t, product.Attributes[2].Value, evidence["accessories"])
	for _, name := range []string{"unrecognized", "颜色", "产品说明"} {
		evidence, err = ImageProductEvidence(catalog.ProductSnapshot{Attributes: []catalog.Attribute{{Name: name, Value: "不能推断规格和配件"}}})
		require.NoError(t, err)
		require.Empty(t, evidence)
	}
	evidence, err = ImageProductEvidence(catalog.ProductSnapshot{Attributes: []catalog.Attribute{{Name: "使用步骤", Value: "  "}}})
	require.NoError(t, err)
	require.Empty(t, evidence)
	evidence, err = ImageProductEvidence(catalog.ProductSnapshot{Attributes: []catalog.Attribute{{Name: "instructions", Value: "第一条原文"}, {Name: "使用步骤", Value: "第二条原文"}}})
	require.NoError(t, err)
	require.Equal(t, "第一条原文\n第二条原文", evidence["instructions"])
	product.Specifications = &catalog.Specifications{Technical: map[string]string{"instructions": "已确认的结构化说明"}}
	evidence, err = ImageProductEvidence(product)
	require.NoError(t, err)
	require.Equal(t, "已确认的结构化说明", evidence["instructions"])
	product.Attributes[0].Value = strings.Repeat("a", 8193)
	_, err = ImageProductEvidence(product)
	require.ErrorIs(t, err, imageagent.ErrValidation)
}
