package listingkit

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func (s *service) sanitizeStudioImageInputURLs(ctx context.Context, inputURLs []string) ([]string, error) {
	return s.taskStudioMediaOrDefault().sanitizeStudioImageInputURLs(ctx, inputURLs)
}

func downloadAndConvertStudioInputImage(ctx context.Context, imageURL string, index int) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build studio input request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download studio input image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("download studio input image returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read studio input image: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode studio input image: %w", err)
	}
	bounds := img.Bounds()
	flattened := image.NewRGBA(bounds)
	draw.Draw(flattened, bounds, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(flattened, bounds, img, bounds.Min, draw.Over)

	buf := bytes.NewBuffer(nil)
	if err := jpeg.Encode(buf, flattened, &jpeg.Options{Quality: 95}); err != nil {
		return nil, "", fmt.Errorf("encode sanitized studio input image: %w", err)
	}
	return buf.Bytes(), sanitizedStudioInputFilename(imageURL, index), nil
}

func sanitizedStudioInputFilename(imageURL string, index int) string {
	parsed, err := url.Parse(strings.TrimSpace(imageURL))
	if err != nil {
		return fmt.Sprintf("studio-input-%d.jpg", index+1)
	}
	base := strings.TrimSpace(path.Base(parsed.Path))
	if base == "" || base == "." || base == "/" {
		return fmt.Sprintf("studio-input-%d.jpg", index+1)
	}
	base = strings.TrimSuffix(base, path.Ext(base))
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("studio-input-%d", index+1)
	}
	return base + ".jpg"
}

func isStudioInputFormatError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "image format is incorrect") ||
		strings.Contains(message, "issues with the image format") ||
		strings.Contains(message, "incorrect image format")
}
