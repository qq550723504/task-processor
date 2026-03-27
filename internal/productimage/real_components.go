package productimage

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"

	amazonimage "task-processor/internal/amazon/image"
	"task-processor/internal/pkg/downloader"
	"task-processor/internal/pkg/imagex"
	"task-processor/internal/pkg/watermark"
	productenrich "task-processor/internal/productenrich"
)

type realImageComponents struct {
	workDir    string
	downloader *downloader.ImageDownloader
	processor  *amazonimage.ImageProcessor
}

func newRealImageComponents(workDir string) (*realImageComponents, error) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return nil, fmt.Errorf("work dir cannot be empty")
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	return &realImageComponents{
		workDir:    workDir,
		downloader: downloader.NewImageDownloader(),
		processor:  amazonimage.NewImageProcessor(),
	}, nil
}

type downloadedImageInspector struct {
	runtime *realImageComponents
}

func NewDownloadedImageInspector(workDir string) (ImageInspector, error) {
	rt, err := newRealImageComponents(workDir)
	if err != nil {
		return nil, err
	}
	return &downloadedImageInspector{runtime: rt}, nil
}

func (i *downloadedImageInspector) Inspect(_ context.Context, imageURL string) (*ImageAudit, error) {
	data, _, err := i.runtime.download(imageURL)
	if err != nil {
		return nil, err
	}
	info, err := i.runtime.processor.GetImageInfo(data)
	if err != nil {
		return nil, err
	}
	img, _, err := imagex.FromBytesWithFormat(data)
	if err != nil {
		return nil, err
	}

	quality := 0.55
	minSide := info.Width
	if info.Height < minSide {
		minSide = info.Height
	}
	switch {
	case minSide >= 1600:
		quality = 0.95
	case minSide >= 1200:
		quality = 0.85
	case minSide >= 1000:
		quality = 0.75
	case minSide >= 800:
		quality = 0.68
	}

	lower := strings.ToLower(imageURL)
	audit := &ImageAudit{
		ImageURL:          imageURL,
		IsWhiteBackground: looksWhiteBackground(img),
		HasOverlayText:    containsAny(lower, "text", "poster", "caption", "label", "desc"),
		HasPromoBadge:     containsAny(lower, "promo", "sale", "discount", "coupon", "price", "badge"),
		HasLogo:           containsAny(lower, "logo", "watermark", "brandmark"),
		IsCollage:         containsAny(lower, "collage", "grid", "contact-sheet", "mosaic"),
		SharpnessScore:    quality,
		QualityScore:      quality,
	}
	if audit.IsWhiteBackground {
		audit.QualityScore += 0.1
	}
	if audit.HasOverlayText {
		audit.QualityScore -= 0.15
		audit.Issues = append(audit.Issues, "overlay_text")
	}
	if audit.HasPromoBadge {
		audit.QualityScore -= 0.2
		audit.Issues = append(audit.Issues, "promo_badge")
	}
	if audit.HasLogo {
		audit.QualityScore -= 0.2
		audit.Issues = append(audit.Issues, "logo_or_watermark")
	}
	if audit.IsCollage {
		audit.QualityScore -= 0.3
		audit.Issues = append(audit.Issues, "collage")
	}
	if !audit.HasOverlayText && !audit.HasPromoBadge && !audit.HasLogo && !audit.IsCollage {
		audit.QualityScore += 0.03
	}
	if audit.QualityScore < 0.1 {
		audit.QualityScore = 0.1
	}
	if audit.QualityScore > 0.98 {
		audit.QualityScore = 0.98
	}
	return audit, nil
}

type optimizedSubjectExtractor struct {
	runtime   *realImageComponents
	segmenter SegmentationClient
}

func NewOptimizedSubjectExtractor(workDir string) (SubjectExtractor, error) {
	return NewHybridSubjectExtractor(workDir, nil)
}

func NewHybridSubjectExtractor(workDir string, segmenter SegmentationClient) (SubjectExtractor, error) {
	rt, err := newRealImageComponents(workDir)
	if err != nil {
		return nil, err
	}
	return &optimizedSubjectExtractor{runtime: rt, segmenter: segmenter}, nil
}

func (e *optimizedSubjectExtractor) Extract(ctx context.Context, imageURL string, analysis *productenrich.ProductAnalysis) (*ImageAsset, error) {
	data, filename, err := e.runtime.download(imageURL)
	if err != nil {
		return nil, err
	}
	if e.segmenter != nil {
		if asset, segErr := e.extractWithSegmenter(ctx, data, filename, imageURL, analysis); segErr == nil {
			return asset, nil
		}
	}
	return e.extractWithLocalCrop(data, filename, imageURL, analysis)
}

type downloadedImageCleaner struct {
	runtime            *realImageComponents
	watermarkProcessor *watermark.Processor
}

func NewDownloadedImageCleaner(workDir string) (ImageCleaner, error) {
	return NewWatermarkAwareImageCleaner(workDir, nil, nil)
}

func NewWatermarkAwareImageCleaner(workDir string, config *watermark.Config, logger *logrus.Logger) (ImageCleaner, error) {
	rt, err := newRealImageComponents(workDir)
	if err != nil {
		return nil, err
	}
	return &downloadedImageCleaner{
		runtime:            rt,
		watermarkProcessor: watermark.NewProcessor(config, logger),
	}, nil
}

func (c *downloadedImageCleaner) Clean(ctx context.Context, asset *ImageAsset, _ *productenrich.ProductAnalysis) (*ImageAsset, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset cannot be nil")
	}
	data, sourceName, err := c.runtime.loadAssetBytes(asset)
	if err != nil {
		return nil, err
	}
	img, format, err := imagex.FromBytesWithFormat(data)
	if err != nil {
		return nil, err
	}

	operations := append([]string{}, asset.Operations...)
	metadata := cloneMetadata(asset.Metadata)
	lower := strings.ToLower(asset.SourceURL)

	regions, hadOverlaySignal := c.detectCleanupRegions(ctx, img, lower)
	processedImg := img
	if len(regions) > 0 {
		removal, removeErr := c.watermarkProcessor.RemoveOnly(context.Background(), img, regions)
		if removeErr != nil {
			return nil, removeErr
		}
		if removal != nil && removal.Image != nil {
			processedImg = removal.Image
			operations = append(operations, "remove_overlay_regions")
			if removal.Quality > 0 {
				metadata["cleanup_quality"] = fmt.Sprintf("%.2f", removal.Quality)
			}
		}
	}

	encoded, err := imagex.ToBytes(processedImg, imagexFormat(format), 92)
	if err != nil {
		return nil, err
	}
	optimized, err := c.runtime.processor.OptimizeForAmazon(encoded)
	if err != nil {
		return nil, err
	}
	path, info, err := c.runtime.writeProcessed(sourceName, "main", optimized)
	if err != nil {
		return nil, err
	}
	metadata["local_path"] = path
	metadata["format"] = info.Format
	if containsAny(lower, "text", "poster", "caption", "label", "desc") {
		metadata["overlay_text_removed"] = "true"
	}
	if containsAny(lower, "promo", "sale", "discount", "coupon", "price", "badge") {
		metadata["promo_badge_removed"] = "true"
	}
	if containsAny(lower, "logo", "watermark", "brandmark") {
		metadata["logo_overlay_removed"] = "true"
	}
	if hadOverlaySignal {
		operations = append(operations, "cleanup_overlay_signal")
	}
	return &ImageAsset{
		URL:        path,
		Type:       AssetTypeMainImage,
		SourceURL:  asset.SourceURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: append(operations, "normalize_for_amazon"),
		Metadata:   metadata,
	}, nil
}

type whiteCanvasRenderer struct {
	runtime  *realImageComponents
	renderer WhiteBackgroundClient
}

func NewWhiteCanvasRenderer(workDir string) (WhiteBackgroundRenderer, error) {
	return NewHybridWhiteBackgroundRenderer(workDir, nil)
}

func NewHybridWhiteBackgroundRenderer(workDir string, renderer WhiteBackgroundClient) (WhiteBackgroundRenderer, error) {
	rt, err := newRealImageComponents(workDir)
	if err != nil {
		return nil, err
	}
	return &whiteCanvasRenderer{runtime: rt, renderer: renderer}, nil
}

func (r *whiteCanvasRenderer) Render(ctx context.Context, asset *ImageAsset, _ *productenrich.ProductAnalysis) (*ImageAsset, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset cannot be nil")
	}
	data, sourceName, err := r.runtime.loadAssetBytes(asset)
	if err != nil {
		return nil, err
	}
	if r.renderer != nil {
		if result, renderErr := r.renderWithClient(ctx, asset, sourceName, data); renderErr == nil {
			return result, nil
		}
	}
	return r.renderWithCanvas(asset, sourceName, data)
}

func (r *whiteCanvasRenderer) renderWithCanvas(asset *ImageAsset, sourceName string, data []byte) (*ImageAsset, error) {
	img, _, err := imagex.FromBytesWithFormat(data)
	if err != nil {
		return nil, err
	}

	width, height := imagex.Size(img)
	canvasSize := width
	if height > canvasSize {
		canvasSize = height
	}
	if canvasSize < 1600 {
		canvasSize = 1600
	}

	canvas := imaging.New(canvasSize, canvasSize, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	fitted := imaging.Fit(img, int(float64(canvasSize)*0.9), int(float64(canvasSize)*0.9), imaging.Lanczos)
	offset := image.Pt((canvasSize-fitted.Bounds().Dx())/2, (canvasSize-fitted.Bounds().Dy())/2)
	composed := imaging.Paste(canvas, fitted, offset)
	encoded, err := imagex.ToBytes(composed, imagex.FormatJPEG, 92)
	if err != nil {
		return nil, err
	}

	path, info, err := r.runtime.writeProcessed(sourceName, "white-bg", encoded)
	if err != nil {
		return nil, err
	}
	metadata := cloneMetadata(asset.Metadata)
	metadata["local_path"] = path
	metadata["background"] = "white"
	metadata["background_mode"] = "white_canvas"
	metadata["format"] = info.Format
	return &ImageAsset{
		URL:        path,
		Type:       AssetTypeWhiteBgImage,
		SourceURL:  asset.SourceURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: append(append([]string{}, asset.Operations...), "compose_on_white_canvas"),
		Metadata:   metadata,
	}, nil
}

func (r *whiteCanvasRenderer) renderWithClient(ctx context.Context, asset *ImageAsset, sourceName string, data []byte) (*ImageAsset, error) {
	result, err := r.renderer.RenderWhiteBackground(ctx, data, asset.SourceURL)
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.ImageData) == 0 {
		return nil, fmt.Errorf("white background result is empty")
	}
	optimized, err := r.runtime.processor.OptimizeForAmazon(result.ImageData)
	if err != nil {
		return nil, err
	}
	path, info, err := r.runtime.writeProcessed(sourceName, "white-bg", optimized)
	if err != nil {
		return nil, err
	}
	metadata := cloneMetadata(asset.Metadata)
	metadata["local_path"] = path
	metadata["background"] = "white"
	metadata["background_mode"] = "model"
	metadata["format"] = info.Format
	for k, v := range result.Metadata {
		metadata[k] = v
	}
	return &ImageAsset{
		URL:        path,
		Type:       AssetTypeWhiteBgImage,
		SourceURL:  asset.SourceURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: append(append([]string{}, asset.Operations...), "render_white_bg_model"),
		Metadata:   metadata,
	}, nil
}

func (r *realImageComponents) download(imageURL string) ([]byte, string, error) {
	data, filename, err := r.downloader.DownloadImage(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("download image %q: %w", imageURL, err)
	}
	return data, filename, nil
}

func (r *realImageComponents) loadAssetBytes(asset *ImageAsset) ([]byte, string, error) {
	if asset == nil {
		return nil, "", fmt.Errorf("asset cannot be nil")
	}
	if localPath := asset.Metadata["local_path"]; localPath != "" {
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, "", fmt.Errorf("read local asset %q: %w", localPath, err)
		}
		return data, filepath.Base(localPath), nil
	}
	if asset.SourceURL != "" {
		return r.download(asset.SourceURL)
	}
	if asset.URL != "" && !strings.HasPrefix(strings.ToLower(asset.URL), "http://") && !strings.HasPrefix(strings.ToLower(asset.URL), "https://") {
		data, err := os.ReadFile(asset.URL)
		if err != nil {
			return nil, "", fmt.Errorf("read asset path %q: %w", asset.URL, err)
		}
		return data, filepath.Base(asset.URL), nil
	}
	return nil, "", fmt.Errorf("asset has no readable source")
}

func (r *realImageComponents) writeProcessed(sourceName, stage string, data []byte) (string, *amazonimage.ImageInfo, error) {
	info, err := r.processor.GetImageInfo(data)
	if err != nil {
		return "", nil, err
	}
	base := strings.TrimSuffix(filepath.Base(sourceName), filepath.Ext(sourceName))
	if base == "" {
		base = "image"
	}
	hash := sha1.Sum(data)
	filename := fmt.Sprintf("%s-%s-%s.%s", base, stage, hex.EncodeToString(hash[:6]), extensionForFormat(info.Format))
	path := filepath.Join(r.workDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", nil, fmt.Errorf("write processed asset: %w", err)
	}
	return path, info, nil
}

func looksWhiteBackground(img image.Image) bool {
	if img == nil {
		return false
	}
	b := img.Bounds()
	points := []image.Point{
		{X: b.Min.X + 2, Y: b.Min.Y + 2},
		{X: b.Max.X - 3, Y: b.Min.Y + 2},
		{X: b.Min.X + 2, Y: b.Max.Y - 3},
		{X: b.Max.X - 3, Y: b.Max.Y - 3},
	}
	whiteCount := 0
	for _, pt := range points {
		r, g, b, _ := img.At(pt.X, pt.Y).RGBA()
		if r>>8 >= 240 && g>>8 >= 240 && b>>8 >= 240 {
			whiteCount++
		}
	}
	return whiteCount >= 3
}

func extensionForFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "jpg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	case "webp":
		return "webp"
	default:
		return "jpg"
	}
}

func (c *downloadedImageCleaner) detectCleanupRegions(ctx context.Context, img image.Image, lowerSourceURL string) ([]*watermark.WatermarkRegion, bool) {
	if c.watermarkProcessor == nil {
		return c.syntheticOverlayRegions(img, lowerSourceURL), containsAny(lowerSourceURL, "text", "poster", "caption", "label", "desc", "promo", "sale", "discount", "coupon", "price", "badge", "logo", "watermark", "brandmark")
	}
	detection, err := c.watermarkProcessor.DetectOnly(ctx, img)
	if err == nil && detection != nil && len(detection.Regions) > 0 {
		return detection.Regions, true
	}
	regions := c.syntheticOverlayRegions(img, lowerSourceURL)
	return regions, len(regions) > 0
}

func (c *downloadedImageCleaner) syntheticOverlayRegions(img image.Image, lowerSourceURL string) []*watermark.WatermarkRegion {
	if img == nil {
		return nil
	}
	if !containsAny(lowerSourceURL, "text", "poster", "caption", "label", "desc", "promo", "sale", "discount", "coupon", "price", "badge", "logo", "watermark", "brandmark") {
		return nil
	}
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()
	regionW := max(width/5, 80)
	regionH := max(height/7, 50)
	if regionW > width {
		regionW = width
	}
	if regionH > height {
		regionH = height
	}
	return []*watermark.WatermarkRegion{
		{
			X:           b.Min.X,
			Y:           b.Min.Y,
			Width:       regionW,
			Height:      regionH,
			Type:        watermark.WatermarkTypeText,
			Position:    watermark.PositionTopLeft,
			Confidence:  0.7,
			Description: "synthetic overlay cleanup region",
		},
	}
}

func imagexFormat(format string) imagex.Format {
	switch strings.ToLower(format) {
	case "png":
		return imagex.FormatPNG
	default:
		return imagex.FormatJPEG
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func extractPrimarySubject(img image.Image) (image.Image, image.Rectangle) {
	if img == nil {
		return nil, image.Rectangle{}
	}
	bounds := img.Bounds()
	bg := estimateBackgroundColor(img)
	tolerance := uint32(28)

	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !isNearBackground(img.At(x, y), bg, tolerance) {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
				found = true
			}
		}
	}

	if !found {
		return img, bounds
	}

	paddingX := max((maxX-minX+1)/12, 12)
	paddingY := max((maxY-minY+1)/12, 12)
	rect := image.Rect(
		max(bounds.Min.X, minX-paddingX),
		max(bounds.Min.Y, minY-paddingY),
		min(bounds.Max.X, maxX+paddingX+1),
		min(bounds.Max.Y, maxY+paddingY+1),
	)
	if rect.Empty() {
		return img, bounds
	}
	return imaging.Crop(img, rect), rect
}

func estimateBackgroundColor(img image.Image) color.NRGBA {
	b := img.Bounds()
	points := []image.Point{
		{X: b.Min.X + 1, Y: b.Min.Y + 1},
		{X: b.Max.X - 2, Y: b.Min.Y + 1},
		{X: b.Min.X + 1, Y: b.Max.Y - 2},
		{X: b.Max.X - 2, Y: b.Max.Y - 2},
		{X: b.Min.X + b.Dx()/2, Y: b.Min.Y + 1},
		{X: b.Min.X + b.Dx()/2, Y: b.Max.Y - 2},
	}

	var sr, sg, sb, sa uint32
	count := uint32(0)
	for _, pt := range points {
		if !pt.In(b) {
			continue
		}
		r, g, bl, a := img.At(pt.X, pt.Y).RGBA()
		sr += r >> 8
		sg += g >> 8
		sb += bl >> 8
		sa += a >> 8
		count++
	}
	if count == 0 {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.NRGBA{
		R: uint8(sr / count),
		G: uint8(sg / count),
		B: uint8(sb / count),
		A: uint8(sa / count),
	}
}

func isNearBackground(c color.Color, bg color.NRGBA, tolerance uint32) bool {
	r, g, b, _ := c.RGBA()
	dr := absDiff(r>>8, uint32(bg.R))
	dg := absDiff(g>>8, uint32(bg.G))
	db := absDiff(b>>8, uint32(bg.B))
	return dr <= tolerance && dg <= tolerance && db <= tolerance
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func (e *optimizedSubjectExtractor) extractWithSegmenter(ctx context.Context, data []byte, filename, imageURL string, analysis *productenrich.ProductAnalysis) (*ImageAsset, error) {
	result, err := e.segmenter.SegmentSubject(ctx, data, imageURL)
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.ImageData) == 0 {
		return nil, fmt.Errorf("segmentation result is empty")
	}
	optimized, err := e.runtime.processor.OptimizeForAmazon(result.ImageData)
	if err != nil {
		return nil, err
	}
	path, info, err := e.runtime.writeProcessed(filename, "subject", optimized)
	if err != nil {
		return nil, err
	}
	metadata := map[string]string{
		"mode":       "segmenter",
		"local_path": path,
		"format":     info.Format,
	}
	for k, v := range result.Metadata {
		metadata[k] = v
	}
	if result.BBox != "" {
		metadata["subject_box"] = result.BBox
	}
	if analysis != nil && analysis.Representation != nil && analysis.Representation.ProductType != "" {
		metadata["product_type"] = analysis.Representation.ProductType
	}
	return &ImageAsset{
		URL:        path,
		Type:       AssetTypeSubjectCutout,
		SourceURL:  imageURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: []string{"download_source", "extract_subject_segmenter", "optimize_for_amazon"},
		Metadata:   metadata,
	}, nil
}

func (e *optimizedSubjectExtractor) extractWithLocalCrop(data []byte, filename, imageURL string, analysis *productenrich.ProductAnalysis) (*ImageAsset, error) {
	img, format, err := imagex.FromBytesWithFormat(data)
	if err != nil {
		return nil, err
	}
	cropped, bbox := extractPrimarySubject(img)
	encoded, err := imagex.ToBytes(cropped, imagexFormat(format), 92)
	if err != nil {
		return nil, err
	}
	optimized, err := e.runtime.processor.OptimizeForAmazon(encoded)
	if err != nil {
		return nil, err
	}
	path, info, err := e.runtime.writeProcessed(filename, "subject", optimized)
	if err != nil {
		return nil, err
	}
	metadata := map[string]string{
		"mode":        "download_backed",
		"local_path":  path,
		"format":      info.Format,
		"subject_box": fmt.Sprintf("%d,%d,%d,%d", bbox.Min.X, bbox.Min.Y, bbox.Max.X, bbox.Max.Y),
	}
	if analysis != nil && analysis.Representation != nil && analysis.Representation.ProductType != "" {
		metadata["product_type"] = analysis.Representation.ProductType
	}
	return &ImageAsset{
		URL:        path,
		Type:       AssetTypeSubjectCutout,
		SourceURL:  imageURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: []string{"download_source", "extract_subject_bbox", "optimize_for_amazon"},
		Metadata:   metadata,
	}, nil
}
