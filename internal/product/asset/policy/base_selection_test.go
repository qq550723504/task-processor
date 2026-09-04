package policy_test

import (
	"reflect"
	"testing"

	"task-processor/internal/product/asset"
	"task-processor/internal/product/asset/policy"
)

func TestDeferredBaseKindsPrioritizesRoleSpecificInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind asset.Kind
		want []asset.Kind
	}{
		{kind: asset.KindModelImage, want: []asset.Kind{asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindGalleryImage, asset.KindSourceImage}},
		{kind: asset.KindSceneImage, want: []asset.Kind{asset.KindSceneImage, asset.KindGalleryImage, asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage}},
		{kind: asset.KindWhiteBgImage, want: []asset.Kind{asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage, asset.KindGalleryImage}},
	}
	for _, tt := range tests {
		if got := policy.DeferredBaseKinds(tt.kind); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("DeferredBaseKinds(%q) = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestCandidateSourceKindsReturnsIndependentSelection(t *testing.T) {
	t.Parallel()

	first := policy.CandidateSourceKinds()
	first[0] = asset.KindSceneImage
	second := policy.CandidateSourceKinds()
	if second[0] != asset.KindSourceImage {
		t.Fatalf("CandidateSourceKinds() retained caller mutation: %v", second)
	}
	if policy.IsCandidateSourceKind(asset.KindSceneImage) {
		t.Fatal("scene image must not become a source candidate")
	}
}
