package listingkit

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

type localSDSMockupRenderInput struct {
	SourceURL        string
	MockupImageURLs  []string
	BlankDesignURL   string
	TemplateImageURL string
	MaskImageURL     string
}

func (s *service) renderLocalSDSMockups(ctx context.Context, input localSDSMockupRenderInput) ([]string, error) {
	if s == nil || s.uploadStore == nil {
		return nil, nil
	}
	sourceURL := strings.TrimSpace(input.SourceURL)
	mockupURLs := localSDSMockupBaseURLs(input)
	if sourceURL == "" || len(mockupURLs) == 0 {
		return nil, nil
	}

	source, err := downloadImageForComposite(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(mockupURLs))
	for index, mockupURL := range uniqueNonEmptyStrings(mockupURLs) {
		mockup, err := downloadImageForComposite(ctx, mockupURL)
		if err != nil {
			continue
		}
		rendered := compositeSDSMockup(mockup, source)
		var buf bytes.Buffer
		if err := png.Encode(&buf, rendered); err != nil {
			continue
		}
		stored, err := s.uploadStore.Save(ctx, &ImageUploadInput{
			Filename:    fmt.Sprintf("sds-mockup-%02d.png", index+1),
			ContentType: "image/png",
			Data:        buf.Bytes(),
		})
		if err != nil {
			continue
		}
		if strings.TrimSpace(stored.PublicURL) != "" {
			result = append(result, stored.PublicURL)
			continue
		}
		result = append(result, buildUploadedImagePath(stored.Key))
	}
	return result, nil
}

func localSDSMockupBaseURLs(input localSDSMockupRenderInput) []string {
	urls := uniqueNonEmptyStrings(input.MockupImageURLs)
	blank := strings.TrimSpace(input.BlankDesignURL)
	if blank == "" {
		return urls
	}
	if len(urls) == 0 {
		return []string{blank}
	}
	urls[0] = blank
	return uniqueNonEmptyStrings(urls)
}

func compositeSDSMockup(mockup image.Image, source image.Image) image.Image {
	base := imaging.Clone(mockup)
	width := base.Bounds().Dx()
	height := base.Bounds().Dy()
	if width <= 0 || height <= 0 {
		return base
	}

	side := int(float64(minInt(width, height)) * 0.76)
	if side <= 0 {
		return base
	}
	design := imaging.Fit(source, side, side, imaging.Lanczos)
	designWidth := design.Bounds().Dx()
	designHeight := design.Bounds().Dy()

	x := (width - designWidth) / 2
	y := (height - designHeight) / 2
	draw.Draw(base, image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x+designWidth, y+designHeight)}, design, image.Point{}, draw.Over)
	return base
}

func downloadImageForComposite(ctx context.Context, imageURL string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: status %d", imageURL, resp.StatusCode)
	}
	img, err := imaging.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(imageURL), err)
	}
	return img, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
