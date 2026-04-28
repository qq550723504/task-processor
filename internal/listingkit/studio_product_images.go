package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/prompt"
)

const maxStudioProductImageCount = 9

type studioProductImageRole struct {
	Key         string
	Label       string
	Goal        string
	Composition string
}

var defaultStudioProductImageRoles = []studioProductImageRole{
	{Key: "main", Label: "Main image", Goal: "Create the primary Amazon-style hero image. Use a pure white RGB(255,255,255) background, high-definition product photography, product centered and fully displayed, no crop, no shadow, no reflection, no text, no watermark, no logo.", Composition: "Square 1:1 composition, product fills most of the frame while remaining fully visible, soft even lighting, clear material texture, clean and simple, ready to upload as a marketplace main image."},
	{Key: "scene", Label: "Scene image", Goal: "Create a realistic natural usage scene based on the actual SDS product type. Product remains the visual center, scene is clean, compatible with the product, and not cluttered.", Composition: "Square 1:1 product photography, soft lighting, unified color tone, clear material texture, no text, no watermark, no brand, no logo."},
	{Key: "selling_point", Label: "Selling point image", Goal: "Highlight 3 to 4 core product advantages using concise text and minimal icons only when useful. Do not make unsupported claims.", Composition: "White or light gray clean background, product centered or left, selling points on right or bottom, high contrast, text/icons must not cover the product, unified tone, no watermark, no brand, no logo."},
	{Key: "detail", Label: "Detail image", Goal: "Show a key product detail, print quality, material texture, edge clarity, craftsmanship, or surface finish.", Composition: "Square 1:1, white or light gray background, magnified sharp detail. Optional 1 to 2 short text lines plus a minimal icon if it helps explain the detail; do not cover the detail; no watermark, no brand, no logo."},
	{Key: "dimension", Label: "Dimension image", Goal: "Create a dimension reference image only from accurate SDS dimensions or SDS size reference mockups. Do not invent measurements. If exact dimensions are available in cm/mm/in, display only inches and convert accurately.", Composition: "Pure white background, product fully displayed, clean high-contrast dimension lines and numbers, uniform font, no clutter, no watermark, no logo. If no reliable dimension data is available, use the SDS size reference image as guidance or produce a non-numeric scale reference."},
	{Key: "angle", Label: "Alternate angle", Goal: "Show a second useful product angle that helps buyers understand shape, thickness, structure, or construction.", Composition: "Three-quarter or side angle on a clean background, product remains sharp and central, no text overlays unless the user explicitly asks."},
	{Key: "multi_scene", Label: "Multi-scenario image", Goal: "Show multiple real usage scenarios for the same product when that makes sense for the product type.", Composition: "Clean layout with up to 3 clear areas, each showing one actual usage scenario. Optional minimal icon plus one short description per area; text/icons must not cover the product; no clutter, no watermark, no brand, no logo."},
	{Key: "material", Label: "Material image", Goal: "Highlight material, surface finish, fabric, print area, tactile qualities, or construction without exaggerating claims.", Composition: "Macro or close detail, realistic lighting, clean background, optional minimal text/icon only if useful, no watermark, no brand, no logo."},
	{Key: "character_scene", Label: "Character scene image", Goal: "Optional human-use scene. Use a real person only when the actual SDS product is naturally worn, held, used, or demonstrated by a person. If not applicable, create a normal product scene without a person.", Composition: "Natural expression and normal posture when a person is used. Product remains clear and central. Optional simple 2 to 3 lines of text plus minimal icon; text/icon must not cover product or person; no clutter, no watermark, no brand, no logo."},
}

func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	theme := strings.TrimSpace(req.Prompt)
	if theme == "" {
		return nil, fmt.Errorf("invalid request: prompt is required")
	}
	sourceURL := strings.TrimSpace(req.SourceDesignURL)
	if sourceURL == "" {
		return nil, fmt.Errorf("invalid request: source_design_url is required")
	}
	if s.studioImageGenerator == nil {
		return nil, fmt.Errorf("studio image generator is not configured")
	}

	count := req.Count
	if count <= 0 {
		count = maxStudioProductImageCount
	}
	if count > maxStudioProductImageCount {
		count = maxStudioProductImageCount
	}

	roles := selectStudioProductImageRoles(count)
	images := make([]StudioGeneratedImage, len(roles))
	errs := make([]error, len(roles))
	sem := make(chan struct{}, studioProductImageConcurrencyLimit(len(roles)))
	var wg sync.WaitGroup
	for idx, role := range roles {
		idx, role := idx, role
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			promptText := buildStudioProductImagePrompt(req, role, idx+1, len(roles))
			imageURL, err := s.generateOneStudioProductImage(ctx, req, sourceURL, promptText)
			if err != nil {
				errs[idx] = fmt.Errorf("%s: %w", role.Label, err)
				return
			}
			images[idx] = StudioGeneratedImage{
				ID:            uuid.NewString(),
				ImageURL:      imageURL,
				RevisedPrompt: fmt.Sprintf("%s %s", firstNonEmptyString(req.StyleName, req.ProductName, "AI"), role.Label),
				Role:          role.Key,
				RoleLabel:     role.Label,
			}
		}()
	}
	wg.Wait()

	response := &StudioProductImageResponse{
		Images: make([]StudioGeneratedImage, 0, len(roles)),
	}
	for _, image := range images {
		if strings.TrimSpace(image.ImageURL) != "" {
			response.Images = append(response.Images, image)
		}
	}
	if len(response.Images) == 0 {
		return nil, errors.Join(nonNilErrors(errs)...)
	}
	return response, nil
}

func nonNilErrors(errs []error) []error {
	result := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	if len(result) == 0 {
		result = append(result, fmt.Errorf("product image generation returned no usable images"))
	}
	return result
}

func studioProductImageConcurrencyLimit(imageCount int) int {
	if imageCount <= 0 {
		return 1
	}
	return imageCount
}

func (s *service) generateOneStudioProductImage(ctx context.Context, req *StudioProductImageRequest, sourceURL string, basePrompt string) (string, error) {
	inputImages := studioProductImageInputURLs(sourceURL, req.ProductReferenceImageURLs)
	generated, err := s.studioImageGenerator.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          s.studioImageGenerator.GetDefaultModel(),
		Prompt:         strings.TrimSpace(basePrompt),
		ImageURL:       inputImages[0],
		ImageURLs:      inputImages,
		Size:           "1024x1024",
		ResponseFormat: "b64_json",
		N:              1,
	})
	if err != nil {
		generated, err = s.studioImageGenerator.EditImage(ctx, &openaiclient.ImageEditRequest{
			Model:          s.studioImageGenerator.GetDefaultModel(),
			Prompt:         strings.TrimSpace(basePrompt),
			ImageURL:       inputImages[0],
			ImageURLs:      inputImages[:1],
			Size:           "1024x1024",
			ResponseFormat: "b64_json",
			N:              1,
		})
		if err != nil {
			return "", fmt.Errorf("generate product image: %w", err)
		}
	}
	imageURL, _, err := s.persistGeneratedStudioImage(ctx, generated, "studio-product-image.png")
	return imageURL, err
}

func buildStudioProductImagePrompt(req *StudioProductImageRequest, role studioProductImageRole, imageIndex int, imageTotal int) string {
	categoryPath := strings.Join(req.CategoryPath, " > ")
	userInstruction := studioProductImageUserInstruction(req, role)
	fallback := strings.TrimSpace(`Create Amazon-compliant ecommerce product images.
The first input image is the approved POD artwork and is the authoritative design source.
Preserve the approved artwork's exact visual identity: same main subject, layout, colors, typography, symbols, relative positions, and overall composition.
Do not redesign, reinterpret, simplify, replace, translate, add to, or remove elements from the approved artwork.
Use the SDS product reference images only as product shape, material, scale, color-variant, lighting, and context references.
Blend the approved artwork onto or into the product presentation naturally, but keep the artwork recognizable as the same design across every generated image.
Theme: {{.Prompt}}
Product: {{.ProductName}}
Category: {{.CategoryPath}}
Style: {{.StyleName}}
Image {{.ImageIndex}} of {{.ImageTotal}}.
Image role: {{.ImageRoleLabel}}.
Role goal: {{.ImageGoal}}
Composition guidance: {{.ImageComposition}}
{{.UserInstructionLine}}
Requirements: square 1:1 image, clean marketplace composition, product remains the hero, no text overlays, no watermarks, no logos, no policy-sensitive claims.`)
	vars := map[string]any{
		"Prompt":              strings.TrimSpace(req.Prompt),
		"ProductName":         strings.TrimSpace(req.ProductName),
		"CategoryPath":        categoryPath,
		"StyleName":           strings.TrimSpace(req.StyleName),
		"CustomPrompt":        userInstruction,
		"ImageIndex":          imageIndex,
		"ImageTotal":          imageTotal,
		"ImageRole":           role.Key,
		"ImageRoleLabel":      role.Label,
		"ImageGoal":           role.Goal,
		"ImageComposition":    role.Composition,
		"ProductNameLine":     productNameLine(req.ProductName),
		"CategoryLine":        categoryLine(categoryPath),
		"StyleLine":           styleLine(req.StyleName),
		"UserInstructionLine": userInstructionLine(userInstruction),
	}
	if prompt.GlobalRegistry == nil {
		return renderPromptFallback(fallback, vars)
	}
	rendered, err := prompt.GlobalRegistry.Render(prompt.KProductImageStudioGenerationAmazonProductImage, vars, fallback)
	if err != nil {
		return renderPromptFallback(fallback, vars)
	}
	return strings.TrimSpace(rendered)
}

func studioProductImageUserInstruction(req *StudioProductImageRequest, role studioProductImageRole) string {
	if req == nil {
		return ""
	}
	instructions := make([]string, 0, 2)
	if global := strings.TrimSpace(req.CustomPrompt); global != "" {
		instructions = append(instructions, "Global: "+global)
	}
	for _, item := range req.ImagePrompts {
		if !strings.EqualFold(strings.TrimSpace(item.Role), strings.TrimSpace(role.Key)) {
			continue
		}
		if prompt := strings.TrimSpace(item.Prompt); prompt != "" {
			instructions = append(instructions, role.Label+": "+prompt)
			break
		}
	}
	return strings.Join(instructions, "\n")
}

func studioProductImageInputURLs(sourceDesignURL string, referenceURLs []string) []string {
	urls := []string{strings.TrimSpace(sourceDesignURL)}
	seen := map[string]struct{}{}
	result := make([]string, 0, 1+len(referenceURLs))
	for _, rawURL := range append(urls, referenceURLs...) {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
		if len(result) >= 5 {
			break
		}
	}
	return result
}

func selectStudioProductImageRoles(count int) []studioProductImageRole {
	if count <= 0 {
		return nil
	}
	if count > len(defaultStudioProductImageRoles) {
		count = len(defaultStudioProductImageRoles)
	}
	return append([]studioProductImageRole(nil), defaultStudioProductImageRoles[:count]...)
}

func productNameLine(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return "SDS base product: " + trimmed + "."
	}
	return ""
}

func categoryLine(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return "Product category: " + trimmed + "."
	}
	return ""
}

func styleLine(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return "Approved style concept: " + trimmed + "."
	}
	return ""
}

func userInstructionLine(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return "User product image instructions: " + trimmed
	}
	return ""
}

func renderPromptFallback(tmpl string, vars map[string]any) string {
	rendered, err := prompt.NewTemplateRenderer().Render(tmpl, vars)
	if err != nil {
		return tmpl
	}
	return strings.TrimSpace(rendered)
}
