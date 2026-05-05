package shein

import (
	"strings"
	"testing"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestNarrowDisplayAttributeValueOptionsPrioritizesOverlappingCandidates(t *testing.T) {
	t.Parallel()

	attr := sheinattribute.AttributeInfo{
		AttributeID:     160,
		AttributeNameEn: "Material",
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 1, AttributeValueEn: "Linen"},
			{AttributeValueID: 2, AttributeValueEn: "Polyester"},
			{AttributeValueID: 3, AttributeValueEn: "Cotton"},
			{AttributeValueID: 4, AttributeValueEn: "Acrylic"},
		},
	}

	options := narrowDisplayAttributeValueOptions(
		attr,
		"Material",
		"Poly blend",
		[]common.Attribute{{Name: "Description", Value: "Polyester fabric wall clock"}},
		3,
	)
	if len(options) != 3 {
		t.Fatalf("options len = %d, want 3", len(options))
	}
	if options[0].AttributeValueID != 2 {
		t.Fatalf("first option id = %d, want polyester first", options[0].AttributeValueID)
	}
}

func TestBuildTemplateAttributeValueBatchMappingPromptNarrowsCandidates(t *testing.T) {
	t.Parallel()

	values := make([]sheinattribute.AttributeValue, 0, 10)
	for i, name := range []string{"Linen", "Silk", "Wool", "Acrylic", "Polyester", "Nylon", "Rayon", "Canvas", "PVC", "Latex"} {
		values = append(values, sheinattribute.AttributeValue{
			AttributeValueID: 100 + i,
			AttributeValueEn: name,
		})
	}
	prompt := buildTemplateAttributeValueBatchMappingPrompt([]unresolvedDisplayAttributeValue{{
		Source: common.Attribute{Name: "Material", Value: "Poly blend"},
		Attr: sheinattribute.AttributeInfo{
			AttributeID:          160,
			AttributeNameEn:      "Material",
			AttributeValueInfoList: values,
		},
	}}, []common.Attribute{{Name: "Description", Value: "Polyester fabric wall clock"}})

	if !strings.Contains(prompt, `attribute_value_id=104 value="" value_en="Polyester"`) {
		t.Fatalf("prompt missing polyester candidate: %q", prompt)
	}
	if strings.Contains(prompt, `attribute_value_id=109 value="" value_en="Latex"`) {
		t.Fatalf("prompt still contains low-signal trailing candidate: %q", prompt)
	}
}

func TestDescribeDisplayAttributeCandidatesUsesNarrowedOptions(t *testing.T) {
	t.Parallel()

	attr := sheinattribute.AttributeInfo{
		AttributeID:     160,
		AttributeNameEn: "Material",
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 1, AttributeValueEn: "Linen"},
			{AttributeValueID: 2, AttributeValueEn: "Polyester"},
			{AttributeValueID: 3, AttributeValueEn: "Cotton"},
			{AttributeValueID: 4, AttributeValueEn: "Acrylic"},
		},
	}
	desc := describeDisplayAttributeCandidates(
		attr,
		"Material",
		"Poly blend",
		[]common.Attribute{{Name: "Description", Value: "Polyester fabric wall clock"}},
		3,
	)
	if !strings.Contains(desc, "Polyester(2)") {
		t.Fatalf("candidate description = %q, want polyester candidate", desc)
	}
	if strings.Contains(desc, "Acrylic(4)") {
		t.Fatalf("candidate description = %q, want narrowed result without acrylic", desc)
	}
}

func TestDescribeDisplayAttributeEvidenceFieldsUsesUniqueNames(t *testing.T) {
	t.Parallel()

	fields := describeDisplayAttributeEvidenceFields([]common.Attribute{
		{Name: "Title", Value: "Wall clock"},
		{Name: "Description", Value: "Composite board wall clock"},
		{Name: "Title", Value: "Wall clock duplicate"},
		{Name: "Material", Value: "Composite board"},
	}, 3)

	if !strings.Contains(fields, "Title") || !strings.Contains(fields, "Description") || !strings.Contains(fields, "Material") {
		t.Fatalf("fields = %q, want title/description/material", fields)
	}
	if strings.Count(fields, "Title") != 1 {
		t.Fatalf("fields = %q, want deduped title", fields)
	}
}
