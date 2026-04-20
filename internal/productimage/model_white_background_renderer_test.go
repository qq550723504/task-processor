package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

func TestModelWhiteBackgroundRendererUsesFaithfulEditor(t *testing.T) {
	editor := &faithfulEditorStub{
		result: &productimage.FaithfulEditResult{
			Asset: &productimage.ImageAsset{
				URL:      "white.jpg",
				Type:     productimage.AssetTypeWhiteBgImage,
				Metadata: map[string]string{},
			},
			Metadata: &productimage.GenerationMetadata{
				Provider:       "openai",
				ModelFamily:    "gpt-image",
				GenerationMode: "white_background",
				PromptRef:      "productimage.white_background.default",
				PromptKey:      "productimage.white_background.default",
				PromptSource:   "registry",
				PromptVersion:  "default",
			},
		},
	}

	renderer := productimage.NewModelWhiteBackgroundRenderer(editor)
	asset, err := renderer.Render(context.Background(), &productimage.ImageAsset{URL: "subject.png"}, &productimage.ProductContext{ProductType: "dress"})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if editor.lastReq == nil || editor.lastReq.Operation != "render_white_background" {
		t.Fatalf("last request = %+v", editor.lastReq)
	}
	if editor.lastReq.PromptRef != "productimage.white_background.default" {
		t.Fatalf("last request = %+v", editor.lastReq)
	}
	if asset == nil || asset.URL != "white.jpg" {
		t.Fatalf("asset = %+v", asset)
	}
	if asset.Metadata["generation_mode"] != "white_background" || asset.Metadata["prompt_ref"] != "productimage.white_background.default" {
		t.Fatalf("asset metadata = %+v", asset.Metadata)
	}
	if asset.Metadata["prompt_key"] != "productimage.white_background.default" || asset.Metadata["prompt_source"] != "registry" || asset.Metadata["prompt_version"] != "default" {
		t.Fatalf("asset metadata = %+v", asset.Metadata)
	}
}
