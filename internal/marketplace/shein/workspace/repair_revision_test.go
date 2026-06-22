package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestCloneRepairPatchPayloadDeepCopiesNestedSlices(t *testing.T) {
	t.Parallel()

	categoryID := 123
	stockCount := -7
	payload := &RepairPatchPayload{
		CategoryResolution: &CategoryResolutionPatch{
			CategoryID:     &categoryID,
			CategoryIDList: []int{123, 456},
		},
		AttributeResolution: &AttributeResolutionPatch{
			PendingAttributes: []common.Attribute{{Name: "Material", Value: "Cotton"}},
			PendingAttributeCandidates: []sheinpub.PendingAttributeCandidate{{
				AttributeID: 11,
				AttributeValueList: []sheinpub.AttributeValueCandidate{{
					AttributeValueID: 22,
					Value:            "Cotton",
				}},
			}},
			RecommendedAttributeCandidates: []sheinpub.PendingAttributeCandidate{{
				AttributeID: 33,
				AttributeValueList: []sheinpub.AttributeValueCandidate{{
					AttributeValueID: 44,
					Value:            "Blend",
				}},
			}},
		},
		SKCPatches: []SKCRevisionPatch{{
			SupplierCode: "SKC-1",
			SKUPatches: []SKURevisionPatch{{
				SupplierSKU: "SKU-1",
				Attributes:  map[string]string{"Size": "M"},
				StockCount:  &stockCount,
			}},
		}},
		Images: &common.ImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
			Gallery:   []string{"https://cdn.example.com/1.jpg"},
		},
		ReviewNotes: []string{"manual review"},
	}

	cloned := CloneRepairPatchPayload(payload)
	if cloned == nil {
		t.Fatal("CloneRepairPatchPayload() = nil, want clone")
	}

	payload.CategoryResolution.CategoryIDList[0] = 999
	payload.AttributeResolution.PendingAttributes[0].Value = "Polyester"
	payload.AttributeResolution.PendingAttributeCandidates[0].AttributeValueList[0].Value = "Silk"
	payload.AttributeResolution.RecommendedAttributeCandidates[0].AttributeValueList[0].Value = "Wool"
	payload.SKCPatches[0].SKUPatches[0].Attributes["Size"] = "L"
	*payload.SKCPatches[0].SKUPatches[0].StockCount = 1
	payload.Images.Gallery[0] = "https://cdn.example.com/changed.jpg"
	payload.ReviewNotes[0] = "changed"

	if cloned.CategoryResolution.CategoryIDList[0] != 123 {
		t.Fatalf("CategoryIDList[0] = %d, want deep clone", cloned.CategoryResolution.CategoryIDList[0])
	}
	if cloned.AttributeResolution.PendingAttributes[0].Value != "Cotton" {
		t.Fatalf("PendingAttributes[0].Value = %q, want deep clone", cloned.AttributeResolution.PendingAttributes[0].Value)
	}
	if cloned.AttributeResolution.PendingAttributeCandidates[0].AttributeValueList[0].Value != "Cotton" {
		t.Fatalf("PendingAttributeCandidates value = %q, want deep clone", cloned.AttributeResolution.PendingAttributeCandidates[0].AttributeValueList[0].Value)
	}
	if cloned.AttributeResolution.RecommendedAttributeCandidates[0].AttributeValueList[0].Value != "Blend" {
		t.Fatalf("RecommendedAttributeCandidates value = %q, want deep clone", cloned.AttributeResolution.RecommendedAttributeCandidates[0].AttributeValueList[0].Value)
	}
	if cloned.SKCPatches[0].SKUPatches[0].Attributes["Size"] != "M" {
		t.Fatalf("SKU Attributes[Size] = %q, want deep clone", cloned.SKCPatches[0].SKUPatches[0].Attributes["Size"])
	}
	if got := *cloned.SKCPatches[0].SKUPatches[0].StockCount; got != -7 {
		t.Fatalf("StockCount = %d, want deep clone", got)
	}
	if cloned.Images.Gallery[0] != "https://cdn.example.com/1.jpg" {
		t.Fatalf("Images.Gallery[0] = %q, want deep clone", cloned.Images.Gallery[0])
	}
	if cloned.ReviewNotes[0] != "manual review" {
		t.Fatalf("ReviewNotes[0] = %q, want deep clone", cloned.ReviewNotes[0])
	}
}

func TestBuildRepairRevisionSeedBuildsMinimalSkeleton(t *testing.T) {
	t.Parallel()

	status := "resolved"
	categoryID := 123
	seed := BuildRepairRevisionSeed("resolve_category", &RepairPatchPayload{
		CategoryResolution: &CategoryResolutionPatch{
			Status:     &status,
			CategoryID: &categoryID,
		},
		ReviewNotes: []string{"确认类目"},
	})

	if seed.Input == nil {
		t.Fatal("Input = nil, want full input")
	}
	if seed.Skeleton == nil || seed.Skeleton.Shein == nil {
		t.Fatalf("Skeleton = %+v, want minimal skeleton", seed.Skeleton)
	}
	if seed.Skeleton.Platform != "shein" || seed.Skeleton.Actor != "desktop-client" || seed.Skeleton.Reason != "repair: resolve_category" {
		t.Fatalf("Skeleton metadata = %+v, want SHEIN desktop repair metadata", seed.Skeleton)
	}
	if seed.Skeleton.Shein.CategoryResolution == nil || seed.Skeleton.Shein.CategoryResolution.CategoryID == nil || *seed.Skeleton.Shein.CategoryResolution.CategoryID != categoryID {
		t.Fatalf("Skeleton SHEIN category patch = %+v, want category id", seed.Skeleton.Shein.CategoryResolution)
	}
}

func TestBuildRepairRevisionSeedSkipsEmptyPayload(t *testing.T) {
	t.Parallel()

	seed := BuildRepairRevisionSeed("", &RepairPatchPayload{})

	if seed.Input != nil || seed.Skeleton != nil {
		t.Fatalf("seed = %+v, want empty seed", seed)
	}
}

func TestRepairRevisionBundleCarriesAppRequest(t *testing.T) {
	t.Parallel()

	type input struct {
		Value string
	}
	type skeleton struct {
		Value string
	}
	type request struct {
		Value string
	}

	bundle := RepairRevisionBundle[input, skeleton, request]{
		Input:    &input{Value: "input"},
		Skeleton: &skeleton{Value: "skeleton"},
		Request:  &request{Value: "request"},
	}

	if bundle.Input.Value != "input" || bundle.Skeleton.Value != "skeleton" || bundle.Request.Value != "request" {
		t.Fatalf("bundle = %+v, want input/skeleton/request", bundle)
	}
}

func TestRepairArtifactsCarriesClonedArtifacts(t *testing.T) {
	t.Parallel()

	type patch struct {
		Value string
	}
	type skeleton struct {
		Value string
	}
	type request struct {
		Value string
	}
	type validation struct {
		Valid bool
	}

	artifacts := RepairArtifacts[patch, skeleton, request, validation]{
		Patch:      &patch{Value: "patch"},
		Skeleton:   &skeleton{Value: "skeleton"},
		Request:    &request{Value: "request"},
		Validation: &validation{Valid: true},
	}

	if artifacts.Patch.Value != "patch" || artifacts.Skeleton.Value != "skeleton" || artifacts.Request.Value != "request" || !artifacts.Validation.Valid {
		t.Fatalf("artifacts = %+v, want repair artifacts", artifacts)
	}
}

func TestBuildRepairReason(t *testing.T) {
	t.Parallel()

	if got := BuildRepairReason(""); got != "repair suggested issue" {
		t.Fatalf("BuildRepairReason(empty) = %q", got)
	}
	if got := BuildRepairReason("resolve_attributes"); got != "repair: resolve_attributes" {
		t.Fatalf("BuildRepairReason(action) = %q", got)
	}
}
