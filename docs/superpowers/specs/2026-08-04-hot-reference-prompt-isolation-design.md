# Hot-reference prompt isolation

## Goal

Make Studio image regeneration deterministic for hot-reference batches while preserving the existing theme-prompt flow. A hot-reference regeneration must reuse the batch's saved reference image and extracted artwork description, and must not silently fall back to a product/theme prompt.

## Current behavior and problem

- `theme_prompt` and `hot_reference` are intended to be mutually exclusive.
- The hot-reference draft persists `hotStyleReferenceBrief` and `hotStyleReferencePrompt`.
- The regeneration path currently builds a request from the ordinary `prompt` and reference URL list, while the saved hot-reference prompt fields are not used in the request.
- In hot-reference mode the ordinary prompt is intentionally cleared during draft normalization, so regeneration can omit the extracted artwork description. This leaves the model with only the image reference and makes results sensitive to reference retrieval and provider interpretation.

## Design

Use one mode-specific request builder:

1. `theme_prompt`
   - Send the ordinary theme prompt.
   - Send no reference images.
   - Keep the existing size and product-image behavior unchanged.

2. `hot_reference`
   - Send exactly one saved hot-reference image.
   - Build the text prompt from the saved extracted artwork prompt, falling back to the saved brief only when the prompt is empty.
   - Treat any user-entered text as an optional artwork constraint, not as a product-type description.
   - Do not include product names such as shirt, mug, or poster in the artwork prompt.

The regeneration handler will use the same mode-specific builder as initial generation, then replace only the selected design while preserving its stable design ID and target metadata. The request will remain one-image generation (`count: 1`).

## Alternatives considered

1. Keep using the ordinary prompt and only repair reference URL handling. This is the smallest change, but it leaves hot-reference prompt fields unused and can still produce weak or unrelated results.
2. Make the backend infer all artwork prompts from the image on every request. This centralizes behavior but adds an extra model dependency and makes regeneration less deterministic.
3. Reuse the persisted extracted artwork prompt in the frontend's mode-specific request builder, with backend validation for the mutually exclusive modes. This preserves the existing data model and avoids an extra analysis call, so it is the selected approach.

## Error handling

- Theme mode with reference URLs remains invalid.
- Hot-reference mode requires exactly one reference URL before generation.
- If a hot-reference batch has neither an extracted prompt nor a brief, generation may still use the backend's fixed image-reference instructions, but the UI should report that the result is reference-only rather than pretending a textual artwork prompt was used.

## Tests

- Hot-reference request uses the persisted extracted prompt, then brief fallback.
- Hot-reference request keeps exactly one reference URL and does not include theme-only product text.
- Theme-prompt request remains unchanged and contains no reference URL.
- Regeneration replaces only the requested design and preserves its target metadata.
- Existing backend and frontend suites remain green.
