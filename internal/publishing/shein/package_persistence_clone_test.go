package shein

import (
	"reflect"
	"sort"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestClonePackageForPersistencePreservesSemanticAliasesAndPrivateAssignmentsWithoutSharingState(t *testing.T) {
	draft := &RequestDraft{SpuName: "original-draft"}
	preview := &sheinproduct.Product{SPUName: "original-preview"}
	submission := &SubmissionReport{LastStatus: "original-submission"}
	finalDraft := &FinalDraft{MainImageURL: "https://img.example/original.jpg"}
	valueID := 701
	privateValueID := 702
	resolution := &SaleAttributeResolution{
		SKCValueAssignments: map[string]ResolvedSaleAttribute{"blue": {Value: "Blue", AttributeValueID: &valueID}},
		SKUValueAssignments: map[string]ResolvedSaleAttribute{"xl": {Value: "XL", AttributeValueID: &valueID}},
		skcAssignments:      map[string]ResolvedSaleAttribute{"SKC-1": {Value: "Blue", AttributeValueID: &privateValueID}},
		skuAssignments:      map[string][]ResolvedSaleAttribute{"SKU-1": {{Value: "XL", AttributeValueID: &privateValueID}}},
		skcValueAssignments: map[string]ResolvedSaleAttribute{"blue": {Value: "Blue", AttributeValueID: &privateValueID}},
		skuValueAssignments: map[string]ResolvedSaleAttribute{"xl": {Value: "XL", AttributeValueID: &privateValueID}},
	}
	pkg := &Package{
		RequestDraft:            draft,
		DraftPayload:            draft,
		PreviewProduct:          preview,
		PreviewPayload:          preview,
		Submission:              submission,
		SubmissionState:         submission,
		FinalDraft:              finalDraft,
		FinalSubmissionDraft:    finalDraft,
		SaleAttributeResolution: resolution,
		Metadata:                map[string]string{"owner": "source"},
	}

	cloned, err := ClonePackageForPersistence(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if cloned.RequestDraft != cloned.DraftPayload || cloned.RequestDraft == pkg.RequestDraft {
		t.Fatal("draft semantic aliases were split or still share source state")
	}
	if cloned.PreviewProduct != cloned.PreviewPayload || cloned.PreviewProduct == pkg.PreviewProduct {
		t.Fatal("preview semantic aliases were split or still share source state")
	}
	if cloned.Submission != cloned.SubmissionState || cloned.Submission == pkg.Submission {
		t.Fatal("submission semantic aliases were split or still share source state")
	}
	if cloned.FinalDraft != cloned.FinalSubmissionDraft || cloned.FinalDraft == pkg.FinalDraft {
		t.Fatal("final-draft semantic aliases were split or still share source state")
	}
	got := cloned.SaleAttributeResolution
	if got == nil || got.skcAssignments["SKC-1"].Value != "Blue" || got.skuAssignments["SKU-1"][0].Value != "XL" || got.skcValueAssignments["blue"].Value != "Blue" || got.skuValueAssignments["xl"].Value != "XL" {
		t.Fatalf("private assignment state was not preserved: %#v", got)
	}
	cloned.DraftPayload.SpuName = "mutated-clone"
	cloned.PreviewPayload.SPUName = "mutated-preview"
	cloned.Metadata["owner"] = "clone"
	clonedID := got.skcAssignments["SKC-1"].AttributeValueID
	*clonedID = 999
	if pkg.DraftPayload.SpuName != "original-draft" || pkg.PreviewPayload.SPUName != "original-preview" || pkg.Metadata["owner"] != "source" || *pkg.SaleAttributeResolution.skcAssignments["SKC-1"].AttributeValueID != privateValueID {
		t.Fatal("clone mutation leaked into source package")
	}
}

func TestSaleAttributeResolutionPrivateCloneSchemaIsExplicit(t *testing.T) {
	typeOf := reflect.TypeOf(SaleAttributeResolution{})
	var privateFields []string
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		if field.PkgPath != "" {
			privateFields = append(privateFields, field.Name)
		}
	}
	sort.Strings(privateFields)
	want := []string{"skcAssignments", "skcValueAssignments", "skuAssignments", "skuValueAssignments"}
	if !reflect.DeepEqual(privateFields, want) {
		t.Fatalf("private clone schema changed: got %v, want %v; update ClonePackageForPersistence explicitly", privateFields, want)
	}
}
