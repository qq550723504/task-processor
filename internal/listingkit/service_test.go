package listingkit

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestNormalizeGenerateRequestDefaults(t *testing.T) {
	t.Parallel()

	req := &GenerateRequest{
		Text:      "demo",
		Platforms: []string{" Amazon ", "shein", "amazon", "invalid", "TEMU"},
	}

	normalizeGenerateRequest(req)

	if req.Country != "US" {
		t.Fatalf("country = %q, want US", req.Country)
	}
	if req.Language != "en_US" {
		t.Fatalf("language = %q, want en_US", req.Language)
	}
	if req.Options == nil || !req.Options.ProcessImages {
		t.Fatal("expected default options with process_images=true")
	}
	if got, want := len(req.Platforms), 3; got != want {
		t.Fatalf("platform count = %d, want %d", got, want)
	}
	if req.Platforms[0] != "amazon" || req.Platforms[1] != "shein" || req.Platforms[2] != "temu" {
		t.Fatalf("normalized platforms = %#v", req.Platforms)
	}
}

func TestNormalizeGenerateRequestEnablesProcessImagesWhenSceneOptionsProvided(t *testing.T) {
	t.Parallel()

	req := &GenerateRequest{
		ProductURL: "https://detail.1688.com/offer/123.html",
		Platforms:  []string{"shein"},
		Options: &GenerateOptions{
			Scene: &productimage.SceneGenerationOptions{
				SceneCategory: "shoes",
			},
		},
	}

	normalizeGenerateRequest(req)

	if req.Options == nil {
		t.Fatal("expected options to remain present")
	}
	if !req.Options.ProcessImages {
		t.Fatal("expected process_images=true when scene options are provided")
	}
}

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *GenerateRequest
		wantErr bool
	}{
		{
			name: "valid text request",
			req: &GenerateRequest{
				Text:      "demo",
				Platforms: []string{"amazon"},
			},
		},
		{
			name: "missing inputs",
			req: &GenerateRequest{
				Platforms: []string{"amazon"},
			},
			wantErr: true,
		},
		{
			name: "missing platforms",
			req: &GenerateRequest{
				Text: "demo",
			},
			wantErr: true,
		},
		{
			name: "too many images",
			req: &GenerateRequest{
				ImageURLs: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
				Platforms: []string{"amazon"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequest(tt.req)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
