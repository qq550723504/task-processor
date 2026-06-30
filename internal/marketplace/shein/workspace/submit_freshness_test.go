package workspace

import (
	"strings"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestEvaluateAttributeFreshnessIgnoresSizeChartAttributes(t *testing.T) {
	t.Parallel()

	current := &sheinpub.Package{
		AttributeResolution: &sheinpub.AttributeResolution{
			Status: "resolved",
			SizeChartAttributes: []sheinpub.PendingAttributeCandidate{
				{AttributeID: 55, AttributeNameEn: "Length (cm)"},
				{AttributeID: 20, AttributeNameEn: "Bust (cm)"},
			},
		},
		DraftPayload: &sheinpub.RequestDraft{
			SizeAttributeList: []sheinproduct.SizeAttribute{
				{AttributeID: 55, AttributeExtraValue: "87.5", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
				{AttributeID: 20, AttributeExtraValue: "87", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
			},
		},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:        55,
					AttributeNameEn:    "Length (cm)",
					AttributeIsShow:    1,
					AttributeType:      2,
					AttributeMode:      0,
					DataDimension:      2,
					SourceSystemIDList: []int{1, 2, 6, 7},
				},
				{
					AttributeID:        20,
					AttributeNameEn:    "Bust (cm)",
					AttributeIsShow:    1,
					AttributeType:      2,
					AttributeMode:      0,
					DataDimension:      2,
					SourceSystemIDList: []int{1, 2, 6, 7},
				},
			},
		}},
	}

	ok, message := EvaluateAttributeFreshness(current, templates)
	if !ok {
		t.Fatalf("EvaluateAttributeFreshness blocked size chart attributes: %s", message)
	}
	if strings.Contains(message, "Bust") || strings.Contains(message, "Length") {
		t.Fatalf("message = %q, should not mention size chart attributes", message)
	}
}
