package listingkit

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestToImageProcessRequestCopiesSceneOptions(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{
			ProductURL: "https://detail.1688.com/offer/123.html",
			Platforms:  []string{"amazon"},
			Country:    "US",
			Options: &GenerateOptions{
				ProcessImages: true,
				Scene: &productimage.SceneGenerationOptions{
					SceneCategory:   "shoes",
					SceneStyle:      "lifestyle",
					BackgroundTone:  "warm",
					Composition:     "close_up",
					PropsLevel:      "light",
					AudienceHint:    "sporty",
					CustomSceneHint: "show subtle motion energy",
				},
			},
		},
	}

	req := toImageProcessRequest(task)
	if req.Scene == nil {
		t.Fatal("expected scene options to be copied")
	}
	if req.Scene.SceneCategory != "shoes" ||
		req.Scene.SceneStyle != "lifestyle" ||
		req.Scene.BackgroundTone != "warm" ||
		req.Scene.Composition != "close_up" ||
		req.Scene.PropsLevel != "light" ||
		req.Scene.AudienceHint != "sporty" ||
		req.Scene.CustomSceneHint != "show subtle motion energy" {
		t.Fatalf("scene options = %+v", req.Scene)
	}
}

func TestShouldProcessImagesAllowsProductURLSource(t *testing.T) {
	req := &GenerateRequest{
		ProductURL: "https://detail.1688.com/offer/123.html",
		Options: &GenerateOptions{
			ProcessImages: true,
		},
	}

	if !shouldProcessImages(req) {
		t.Fatal("expected product_url source to be eligible for image processing")
	}
}
