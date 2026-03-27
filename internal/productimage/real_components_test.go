package productimage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
)

func TestDownloadedImageInspectorAndRenderers(t *testing.T) {
	img := imaging.New(1200, 900, color.NRGBA{R: 248, G: 248, B: 248, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	workDir := t.TempDir()
	ctx := context.Background()

	inspector, err := NewDownloadedImageInspector(workDir)
	if err != nil {
		t.Fatalf("NewDownloadedImageInspector: %v", err)
	}
	audit, err := inspector.Inspect(ctx, server.URL+"/hero_white.jpg")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !audit.IsWhiteBackground {
		t.Fatalf("expected white background audit")
	}
	if audit.QualityScore < 0.7 {
		t.Fatalf("quality score = %v, want >= 0.7", audit.QualityScore)
	}

	extractor, err := NewOptimizedSubjectExtractor(workDir)
	if err != nil {
		t.Fatalf("NewOptimizedSubjectExtractor: %v", err)
	}
	subject, err := extractor.Extract(ctx, server.URL+"/hero_white.jpg", nil)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if subject.Metadata["local_path"] == "" {
		t.Fatal("expected local_path metadata")
	}
	if _, err := os.Stat(subject.Metadata["local_path"]); err != nil {
		t.Fatalf("subject output file missing: %v", err)
	}
	if subject.Width != 1200 || subject.Height != 900 {
		t.Fatalf("optimized subject dimensions = %dx%d, want 1200x900", subject.Width, subject.Height)
	}

	cleaner, err := NewDownloadedImageCleaner(workDir)
	if err != nil {
		t.Fatalf("NewDownloadedImageCleaner: %v", err)
	}
	mainImage, err := cleaner.Clean(ctx, subject, nil)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if mainImage.Metadata["local_path"] == "" {
		t.Fatal("expected cleaned local_path")
	}
	if _, err := os.Stat(mainImage.Metadata["local_path"]); err != nil {
		t.Fatalf("cleaned output file missing: %v", err)
	}

	renderer, err := NewWhiteCanvasRenderer(workDir)
	if err != nil {
		t.Fatalf("NewWhiteCanvasRenderer: %v", err)
	}
	whiteBg, err := renderer.Render(ctx, mainImage, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if whiteBg.Metadata["background"] != "white" {
		t.Fatalf("background = %q, want white", whiteBg.Metadata["background"])
	}
	if filepath.Ext(whiteBg.Metadata["local_path"]) != ".jpg" {
		t.Fatalf("white bg output ext = %q, want .jpg", filepath.Ext(whiteBg.Metadata["local_path"]))
	}
	if whiteBg.Width != whiteBg.Height {
		t.Fatalf("white bg dimensions = %dx%d, want square", whiteBg.Width, whiteBg.Height)
	}
}

func TestDownloadedImageInspector_GivesPassableScoreToCleanMediumImage(t *testing.T) {
	img := imaging.New(838, 838, color.NRGBA{R: 246, G: 246, B: 246, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	inspector, err := NewDownloadedImageInspector(t.TempDir())
	if err != nil {
		t.Fatalf("NewDownloadedImageInspector: %v", err)
	}
	audit, err := inspector.Inspect(context.Background(), server.URL+"/clean-medium.jpg")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if audit.QualityScore < 0.65 {
		t.Fatalf("quality score = %v, want >= 0.65 for clean medium image", audit.QualityScore)
	}
}

func TestWatermarkAwareCleaner_RemovesCornerOverlayRegion(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 500, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 500; x++ {
			img.Set(x, y, color.NRGBA{R: 250, G: 250, B: 250, A: 255})
		}
	}
	for y := 0; y < 40; y++ {
		for x := 0; x < 90; x++ {
			img.Set(x, y, color.NRGBA{R: 10, G: 10, B: 10, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode image: %v", err)
	}

	sourcePath := filepath.Join(t.TempDir(), "promo_watermark.jpg")
	if err := os.WriteFile(sourcePath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	cleaner, err := NewWatermarkAwareImageCleaner(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("NewWatermarkAwareImageCleaner: %v", err)
	}

	cleaned, err := cleaner.Clean(context.Background(), &ImageAsset{
		URL:       sourcePath,
		Type:      AssetTypeSubjectCutout,
		SourceURL: "https://example.com/promo_watermark.jpg",
		Metadata: map[string]string{
			"local_path": sourcePath,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}

	data, err := os.ReadFile(cleaned.Metadata["local_path"])
	if err != nil {
		t.Fatalf("read cleaned file: %v", err)
	}
	processed, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode cleaned image: %v", err)
	}

	r, g, b, _ := processed.At(10, 10).RGBA()
	if r>>8 < 100 || g>>8 < 100 || b>>8 < 100 {
		t.Fatalf("expected corner overlay to be lightened, got rgb=(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
	if cleaned.Metadata["logo_overlay_removed"] != "true" {
		t.Fatalf("logo_overlay_removed = %q, want true", cleaned.Metadata["logo_overlay_removed"])
	}
}

func TestOptimizedSubjectExtractor_CropsPrimarySubjectOnWhiteBackground(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 1200, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 1200; x++ {
			base.Set(x, y, color.NRGBA{R: 250, G: 250, B: 250, A: 255})
		}
	}
	for y := 220; y < 760; y++ {
		for x := 360; x < 840; x++ {
			base.Set(x, y, color.NRGBA{R: 30, G: 60, B: 120, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, base, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	extractor, err := NewOptimizedSubjectExtractor(t.TempDir())
	if err != nil {
		t.Fatalf("NewOptimizedSubjectExtractor: %v", err)
	}
	asset, err := extractor.Extract(context.Background(), server.URL+"/subject_white.jpg", nil)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if asset.Metadata["subject_box"] == "" {
		t.Fatal("expected subject_box metadata")
	}
	if !containsAny(asset.Metadata["subject_box"], ",") {
		t.Fatalf("subject_box = %q, want bbox coordinates", asset.Metadata["subject_box"])
	}
	if asset.Width >= 1200 && asset.Height >= 900 {
		t.Fatalf("expected extracted subject asset to be tighter than source, got %dx%d", asset.Width, asset.Height)
	}
}

func TestHybridSubjectExtractor_UsesSegmentationClientWhenAvailable(t *testing.T) {
	cropped := imaging.New(700, 700, color.NRGBA{R: 220, G: 230, B: 240, A: 255})
	var croppedBuf bytes.Buffer
	if err := jpeg.Encode(&croppedBuf, cropped, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode cropped image: %v", err)
	}

	segmenter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["task"] != "subject_extract" {
			t.Fatalf("task = %v, want subject_extract", req["task"])
		}
		resp := map[string]any{
			"image_base64": base64.StdEncoding.EncodeToString(croppedBuf.Bytes()),
			"format":       "jpeg",
			"bbox":         "50,60,650,660",
			"metadata": map[string]string{
				"provider": "test-segmenter",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer segmenter.Close()

	source := imaging.New(1200, 900, color.NRGBA{R: 250, G: 250, B: 250, A: 255})
	var sourceBuf bytes.Buffer
	if err := jpeg.Encode(&sourceBuf, source, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode source image: %v", err)
	}
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(sourceBuf.Bytes())
	}))
	defer sourceServer.Close()

	client, err := NewHTTPSegmentationClient(HTTPSegmentationClientConfig{Endpoint: segmenter.URL})
	if err != nil {
		t.Fatalf("NewHTTPSegmentationClient: %v", err)
	}
	extractor, err := NewHybridSubjectExtractor(t.TempDir(), client)
	if err != nil {
		t.Fatalf("NewHybridSubjectExtractor: %v", err)
	}
	asset, err := extractor.Extract(context.Background(), sourceServer.URL+"/complex_scene.jpg", nil)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if asset.Metadata["mode"] != "segmenter" {
		t.Fatalf("mode = %q, want segmenter", asset.Metadata["mode"])
	}
	if asset.Metadata["provider"] != "test-segmenter" {
		t.Fatalf("provider = %q, want test-segmenter", asset.Metadata["provider"])
	}
	if asset.Metadata["subject_box"] != "50,60,650,660" {
		t.Fatalf("subject_box = %q, want test bbox", asset.Metadata["subject_box"])
	}
	if asset.Width != 700 || asset.Height != 700 {
		t.Fatalf("segmenter subject dimensions = %dx%d, want 700x700", asset.Width, asset.Height)
	}
}

func TestHybridWhiteBackgroundRenderer_UsesExternalRendererWhenAvailable(t *testing.T) {
	rendered := imaging.New(1600, 1600, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	var renderedBuf bytes.Buffer
	if err := jpeg.Encode(&renderedBuf, rendered, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode rendered image: %v", err)
	}

	whitebg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["task"] != "white_background" {
			t.Fatalf("task = %v, want white_background", req["task"])
		}
		resp := map[string]any{
			"image_base64": base64.StdEncoding.EncodeToString(renderedBuf.Bytes()),
			"format":       "jpeg",
			"metadata": map[string]string{
				"provider": "test-whitebg",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer whitebg.Close()

	source := imaging.New(900, 700, color.NRGBA{R: 200, G: 210, B: 220, A: 255})
	sourcePath := filepath.Join(t.TempDir(), "main.jpg")
	var sourceBuf bytes.Buffer
	if err := jpeg.Encode(&sourceBuf, source, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode source image: %v", err)
	}
	if err := os.WriteFile(sourcePath, sourceBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	client, err := NewHTTPWhiteBackgroundClient(HTTPWhiteBackgroundClientConfig{Endpoint: whitebg.URL})
	if err != nil {
		t.Fatalf("NewHTTPWhiteBackgroundClient: %v", err)
	}
	renderer, err := NewHybridWhiteBackgroundRenderer(t.TempDir(), client)
	if err != nil {
		t.Fatalf("NewHybridWhiteBackgroundRenderer: %v", err)
	}
	asset, err := renderer.Render(context.Background(), &ImageAsset{
		URL:       sourcePath,
		Type:      AssetTypeMainImage,
		SourceURL: "https://example.com/main.jpg",
		Metadata: map[string]string{
			"local_path": sourcePath,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if asset.Metadata["background_mode"] != "model" {
		t.Fatalf("background_mode = %q, want model", asset.Metadata["background_mode"])
	}
	if asset.Metadata["provider"] != "test-whitebg" {
		t.Fatalf("provider = %q, want test-whitebg", asset.Metadata["provider"])
	}
	if asset.Width != 1600 || asset.Height != 1600 {
		t.Fatalf("whitebg dimensions = %dx%d, want 1600x1600", asset.Width, asset.Height)
	}
}
