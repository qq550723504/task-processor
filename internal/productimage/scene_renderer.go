package productimage

import (
	"context"
	"fmt"

	"github.com/disintegration/imaging"

	"task-processor/internal/pkg/imagex"
)

type localSceneRenderer struct {
	runtime *realImageComponents
}

func NewDefaultSceneRenderer(workDir string) (SceneRenderer, error) {
	rt, err := newRealImageComponents(workDir)
	if err != nil {
		return nil, err
	}
	return &localSceneRenderer{runtime: rt}, nil
}

func (r *localSceneRenderer) Render(ctx context.Context, asset *ImageAsset, productContext *ProductContext) ([]ImageAsset, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset cannot be nil")
	}
	data, sourceName, err := r.runtime.loadAssetBytes(asset)
	if err != nil {
		return nil, err
	}

	sceneAsset, err := r.renderScene(ctx, asset, productContext, sourceName, data)
	if err != nil {
		return nil, err
	}
	return []ImageAsset{*sceneAsset}, nil
}

func (r *localSceneRenderer) renderScene(_ context.Context, asset *ImageAsset, productContext *ProductContext, sourceName string, data []byte) (*ImageAsset, error) {
	img, _, err := imagex.FromBytesWithFormat(data)
	if err != nil {
		return nil, err
	}

	width, height := imagex.Size(img)
	profile := resolveSceneProfile(asset)
	canvasSize := width
	if height > canvasSize {
		canvasSize = height
	}
	if canvasSize < 1600 {
		canvasSize = 1600
	}

	background := imaging.Fill(img, canvasSize, canvasSize, imaging.Center, imaging.Lanczos)
	background = imaging.Blur(background, profile.blurRadius)
	background = imaging.AdjustBrightness(background, profile.backgroundBrightness)
	background = imaging.AdjustContrast(background, profile.backgroundContrast)

	subjectScale := profile.subjectScale
	subject := imaging.Fit(img, int(float64(canvasSize)*subjectScale), int(float64(canvasSize)*subjectScale), imaging.Lanczos)
	layout := buildSceneLayoutMetrics(profile, canvasSize, subject)
	card := imaging.New(layout.cardWidth, layout.cardHeight, profile.cardColor)

	composed := imaging.Overlay(background, card, layout.cardPoint, layout.cardOpacity)
	composed = imaging.Overlay(composed, subject, layout.subjectPoint, 1.0)

	encoded, err := imagex.ToBytes(composed, imagex.FormatJPEG, 92)
	if err != nil {
		return nil, err
	}
	path, info, err := r.runtime.writeProcessed(sourceName, "scene", encoded)
	if err != nil {
		return nil, err
	}

	metadata := cloneMetadata(asset.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["local_path"] = path
	metadata["format"] = info.Format
	metadata["scene_mode"] = "local_canvas"
	metadata["layout_engine"] = layout.layoutEngine
	metadata["quality_grade_candidate"] = layout.qualityGradeCandidate
	metadata["render_profile"] = profile.name
	setScenePresetMetadata(metadata, profile)
	applySellingPointContentPlanMetadata(metadata, profile, productContext)
	applySellingPointFillInputMetadata(metadata, profile, productContext)
	applySellingPointRenderBlocksMetadata(metadata, profile, productContext)
	applySellingPointRenderPlanMetadata(metadata, profile, productContext)
	applySellingPointRenderOutputMetadata(metadata, profile, productContext)
	applySellingPointDrawOutputMetadata(metadata, profile, productContext)
	applySellingPointDrawPreviewMetadata(metadata, profile, productContext)
	if productContext != nil && productContext.ProductType != "" {
		metadata["product_type"] = productContext.ProductType
	}

	return &ImageAsset{
		URL:        path,
		Type:       AssetTypeGalleryImage,
		SourceURL:  asset.SourceURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: append(append([]string{}, asset.Operations...), "render_scene_canvas"),
		Metadata:   metadata,
	}, nil
}
