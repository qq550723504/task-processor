package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

type faithfulEditorStub struct {
	lastReq *productimage.FaithfulEditRequest
	result  *productimage.FaithfulEditResult
}

func (s *faithfulEditorStub) Edit(_ context.Context, req *productimage.FaithfulEditRequest) (*productimage.FaithfulEditResult, error) {
	s.lastReq = req
	return s.result, nil
}

func TestModelSubjectExtractorUsesFaithfulEditor(t *testing.T) {
	editor := &faithfulEditorStub{
		result: &productimage.FaithfulEditResult{
			Asset: &productimage.ImageAsset{
				URL:      "subject.png",
				Type:     productimage.AssetTypeSubjectCutout,
				Metadata: map[string]string{},
			},
			Metadata: &productimage.GenerationMetadata{
				Provider:       "openai",
				ModelFamily:    "gpt-image",
				GenerationMode: "subject_extraction",
				PromptRef:      "productimage/subject/extract",
			},
		},
	}

	extractor := productimage.NewModelSubjectExtractor(editor)
	asset, err := extractor.Extract(context.Background(), "https://img.example/source.jpg", &productimage.ProductContext{ProductType: "dress"})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if editor.lastReq == nil || editor.lastReq.Operation != "extract_subject" {
		t.Fatalf("last request = %+v", editor.lastReq)
	}
	if asset == nil || asset.URL != "subject.png" {
		t.Fatalf("asset = %+v", asset)
	}
	if asset.Metadata["generation_mode"] != "subject_extraction" || asset.Metadata["model_family"] != "gpt-image" {
		t.Fatalf("asset metadata = %+v", asset.Metadata)
	}
}
