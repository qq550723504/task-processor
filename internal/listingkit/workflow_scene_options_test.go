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

func TestToImageProcessRequestDoesNotInjectPlatformSceneDefaults(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{
			Platforms: []string{"amazon"},
			Options: &GenerateOptions{
				ProcessImages: true,
			},
		},
	}

	req := toImageProcessRequest(task)
	if req.Scene != nil {
		t.Fatalf("expected scene options to stay empty until productimage runtime resolution, got %+v", req.Scene)
	}
}

func TestToImageProcessRequestCopiesExplicitSceneOptionsWithoutMutatingThem(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			Options: &GenerateOptions{
				ProcessImages: true,
				Scene: &productimage.SceneGenerationOptions{
					SceneCategory: "bags",
					Composition:   "multi_angle",
				},
			},
		},
	}

	req := toImageProcessRequest(task)
	if req.Scene == nil {
		t.Fatal("expected copied scene options")
	}
	if req.Scene.SceneCategory != "bags" {
		t.Fatalf("expected explicit scene category to win, got %+v", req.Scene)
	}
	if req.Scene.Composition != "multi_angle" {
		t.Fatalf("expected explicit composition to win, got %+v", req.Scene)
	}
	if req.Scene.SceneStyle != "" ||
		req.Scene.BackgroundTone != "" ||
		req.Scene.PropsLevel != "" ||
		req.Scene.AudienceHint != "" {
		t.Fatalf("expected only explicit fields to be copied, got %+v", req.Scene)
	}
}
