# ListingKit Hot Style Reference Design

## Goal

Add a reference-image mode to the current POD to SHEIN Studio flow. The user can provide one or more hot-selling product images, let AI extract reusable visual direction such as motif, print style, color palette, composition, and product-fit hints, then generate original POD artwork in a similar market style. The existing downstream flow remains unchanged: review generated artwork, generate SHEIN product images, then create SHEIN listing data.

This feature must not copy a competitor product. It should turn reference images into an abstract, sanitized design brief and use that brief to generate original artwork.

## Current Flow

The current Studio flow already has the needed building blocks:

- Users choose an SDS/POD base product and provide a theme prompt.
- `StudioDesignRequest` generates flat POD artwork.
- `StudioProductImageRequest` uses the approved POD artwork as the authoritative source for SHEIN product images.
- SHEIN task creation consumes the selected artwork/product images through the existing ListingKit result and SHEIN assembler path.

Important existing code anchors:

- `internal/listingkit/ai_contracts.go`: chat/image contracts include image analysis and image editing.
- `internal/listingkit/studio_designs.go`: builds the flat POD artwork prompt.
- `internal/listingkit/task_studio_media_service.go`: generates Studio designs and product images.
- `internal/listingkit/task_studio_media_service_support.go`: routes reference-image design generation through image edit and falls back safely.
- `web/listingkit-ui/src/lib/api/shein-studio.ts`: frontend API wrapper for Studio design/product image generation.
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-form-sections.tsx`: current form surface for POD artwork and SHEIN product image generation settings.

## Recommended Approach

Implement this as a "hot style reference" layer before POD artwork generation, not as a new SHEIN listing pipeline.

The feature adds a small analysis step:

1. User uploads or selects 1 to 5 reference images.
2. Backend analyzes the images and returns a structured, sanitized `reference_style_brief`.
3. Frontend shows the extracted brief and lets the user edit it.
4. Studio artwork generation receives both the brief and the reference image URLs.
5. Existing artwork review, product-image generation, and SHEIN task creation continue unchanged.

This keeps the risky and new behavior isolated to prompt/reference preparation. It avoids changing SHEIN payload assembly, SDS sync, submission readiness, or publishing.

## Data Model

Add request/response models in ListingKit:

```go
type StudioReferenceAnalysisRequest struct {
    ReferenceImageURLs []string `json:"reference_image_urls,omitempty"`
    ProductName        string   `json:"product_name,omitempty"`
    CategoryPath       []string `json:"category_path,omitempty"`
    BasePrompt         string   `json:"base_prompt,omitempty"`
    UserInstruction    string   `json:"user_instruction,omitempty"`
}

type StudioReferenceAnalysisResponse struct {
    ReferenceStyleBrief string   `json:"reference_style_brief,omitempty"`
    SanitizedPrompt     string   `json:"sanitized_prompt,omitempty"`
    Warnings            []string `json:"warnings,omitempty"`
}
```

Extend Studio draft/batch state so the frontend can persist and restore:

- `hot_style_reference_image_urls`
- `hot_style_reference_brief`
- `hot_style_reference_prompt`

Use the existing image upload store and public uploaded image URLs. Do not introduce a new storage subsystem.

## Backend Design

Add a Studio media service method:

```go
AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error)
```

Behavior:

- Require at least one reference image URL.
- Limit to 5 images.
- Analyze each image with the existing `AIChatCompleter.AnalyzeImage`.
- Ask for JSON-only output with fields such as motif, print technique, palette, composition, typography style, density, target buyer signal, product-fit notes, and avoid list.
- Merge multiple image analyses into one final brief. If the model returns malformed JSON, fall back to a concise text brief rather than failing the whole user flow.
- Explicitly remove or avoid brand names, logos, exact slogans, celebrity/person identity, copyrighted characters, and unique composition instructions that would produce a close copy.

The response should be suitable to append to the current managed POD artwork prompt. It should not directly become a raw prompt unless the user chooses raw mode.

## Prompting Rules

The analysis prompt should produce abstract design direction:

- Allowed: "retro varsity layout", "large central animal silhouette", "cream/navy/red palette", "bold distressed typography feel", "balanced badge composition".
- Not allowed: "copy this exact shirt", "same logo", "same wording", "same mascot", "same character", "same product photo".

The generation prompt should include a hard originality instruction:

```text
Use the reference images only to understand broad commercial style, motif category,
palette direction, print density, and composition family. Create a new original POD
artwork. Do not reproduce logos, brand marks, exact text, characters, faces, or the
same unique layout from any reference image.
```

## Frontend Design

Add a compact section inside the current "生成 POD 款式图" area:

- Upload/select reference images.
- Button: "提取热销款风格".
- Read-only preview of extracted brief with an edit option.
- Apply button that writes the sanitized prompt into the theme prompt or appends it below the existing theme.
- Keep current manual prompt workflow available when no reference image is provided.

The user-facing framing should be "提取风格" and "生成相似风格原创图案", not "复制同款".

Persist the reference image URLs and brief in the existing draft/session state so users can leave and resume a batch.

## API Surface

Add an endpoint under the existing ListingKit Studio API group:

```http
POST /api/v1/listing-kits/studio/reference-style/analyze
```

Request:

```json
{
  "reference_image_urls": ["https://..."],
  "product_name": "T-shirt",
  "category_path": ["Apparel", "Tops"],
  "base_prompt": "summer beach style",
  "user_instruction": "more suitable for women"
}
```

Response:

```json
{
  "reference_style_brief": "...",
  "sanitized_prompt": "...",
  "warnings": []
}
```

This endpoint only analyzes and prepares prompt material. It does not create Studio designs or SHEIN tasks by itself.

## Error Handling

- No image URL: return `400 invalid_request`.
- AI analyzer unavailable: return a clear `reference_analysis_unavailable` error and keep the manual prompt path usable.
- Partial image failure: include warnings and use successful image analyses.
- All image failures: return a failure with the first compact cause.
- Unsafe/IP-heavy reference: return a brief that excludes risky elements and warn the user that exact logos/text/characters were omitted.

## Testing

Backend tests:

- Analysis rejects empty reference image lists.
- Analysis limits references to 5 images.
- Analyzer output is sanitized when it contains brand/logo/exact-copy language.
- Malformed analyzer JSON falls back to usable text.
- Studio design generation still works without reference images.
- Studio design generation with reference images continues through the existing edit-image path.

Frontend tests:

- Reference images can be selected/uploaded and persisted in draft state.
- Clicking "提取热销款风格" calls the new API and fills the brief/prompt.
- Manual prompt generation still works when no reference images exist.
- Saved batch hydration restores reference images and brief.

## Rollout Plan

1. Backend model, service, prompt, and handler.
2. Frontend API wrapper and draft state fields.
3. Studio form UI for reference images and extracted brief.
4. Wire extracted brief into existing `generateSheinStudioDesigns` input.
5. Focused backend and frontend tests.

## Non-Goals

- Do not change SHEIN assembler or submission payloads.
- Do not add a new image storage backend.
- Do not automate competitor-product scraping.
- Do not bypass the existing artwork review step.
- Do not promise exact same style, exact same print, or exact same sales performance.

## Open Decisions

- MVP should use uploaded image URLs and existing image selectors only. A later iteration can add a hot-product library or import from external URLs.
- MVP should append the sanitized prompt to the current theme prompt. A later iteration can show a separate structured brief editor if operators need finer control.
