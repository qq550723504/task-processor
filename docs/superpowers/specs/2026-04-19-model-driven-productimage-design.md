# Model-Driven Product Image Pipeline Design

## Summary

This design replaces the current local-canvas-centric `productimage` rendering path with a model-driven image pipeline.

The target behavior is:

- product understanding stays model-driven
- subject extraction and white-background generation move to a faithful image editing model
- scene and selling-point image generation move to a generative image model
- review becomes model-first, with rules kept only as hard validation guards
- local canvas rendering is no longer the primary production path

The selected approach is a layered dual-model architecture:

- faithful editor for product-preserving image edits
- scene generator for scene and selling-point outputs

This fits the business constraint that main product imagery must preserve identity, while scene imagery may be more flexible.

## Goals

- Remove local canvas composition from the primary image generation path.
- Preserve product identity for subject extraction and white-background outputs.
- Allow controlled generative rendering for scene and selling-point outputs.
- Keep the existing `listingkit` generation/review protocol usable with minimal contract churn.
- Make model choice, prompt lineage, and generation metadata explicit in task execution records.

## Non-Goals

- Building a freeform prompt-based image creation product.
- Removing all rule-based validation from the system.
- Replacing the entire `listingkit` review/navigation protocol.
- Defining a vendor-specific model implementation in this spec.

## Current State

Today the pipeline mixes model-based understanding with mostly local rendering:

- `productenrich/enrich/understanding.go` uses text and vision models to analyze images, extract text attributes, and fuse multimodal product representation.
- `productenrich/llm_scorer.go` uses text and vision models to score text and images.
- `productimage/default_components.go` provides default subject extraction and white-background rendering that are placeholder or rule-oriented.
- `productimage/scene_renderer.go` uses local canvas composition and image operations for scene rendering.
- `productimage/default_components.go` review assessment is threshold and trace driven, not model-first.

This means the system uses AI for understanding and scoring, but not as the primary production mechanism for final image assets.

## Selected Architecture

### High-Level Structure

The new image pipeline has four model-aware layers:

1. Product understanding
2. Faithful edit generation
3. Scene generation
4. Review and validation

### Layer 1: Product Understanding

This layer remains model-driven and continues to produce structured product context from:

- product URL content
- product title and description
- uploaded source images

Primary outputs:

- `ProductContext`
- structured product representation
- quality-relevant extracted attributes

This layer stays conceptually close to current `productenrich` understanding and does not require a new business contract. The main requirement is to make provider usage and generation lineage explicit.

### Layer 2: Faithful Edit Generation

This layer is responsible for outputs that must preserve product identity.

Responsibilities:

- subject extraction
- background removal
- white-background generation
- product-preserving cleanup and normalization

This layer replaces the current primary implementations behind:

- `SubjectExtractor`
- `WhiteBackgroundRenderer`

Required properties:

- high product identity preservation
- deterministic enough for commerce main-image usage
- generation metadata recorded for audit and retry

### Layer 3: Scene Generation

This layer is responsible for outputs that can be more generative.

Responsibilities:

- gallery scene images
- selling-point visual outputs
- controlled background and composition generation

This layer replaces the current primary implementation behind:

- `SceneRenderer`

Required properties:

- accepts source reference image or extracted subject asset
- accepts product context and style preset
- supports constrained generation modes per slot/platform/capability

Local canvas composition is retained only as a development fallback path, not the default production renderer.

### Layer 4: Review and Validation

This layer becomes model-first for qualitative judgment, with rules retained as hard guards.

Model-first review handles:

- product identity drift
- subject loss
- excessive scene dominance
- style deviation
- visual plausibility

Rule validation handles:

- resolution
- missing asset variants
- image count requirements
- platform-specific format constraints

The review outcome remains compatible with current review queues and operator workflows, but the rationale should distinguish:

- model review reasons
- rule validation reasons

## Interfaces and Boundaries

### Business Interfaces to Keep

The following business-level interfaces remain useful and should be preserved:

- `SubjectExtractor`
- `WhiteBackgroundRenderer`
- `SceneRenderer`
- `ReviewAssessor`

The default production implementations behind these interfaces change from local implementations to model-backed implementations.

### New Provider-Level Interfaces

Introduce explicit model-provider contracts to avoid coupling business interfaces to one vendor:

- `ProductImageModelProvider`
- `FaithfulEditor`
- `SceneGenerator`
- `ImageReviewModel`

Responsibilities:

- business interfaces stay stable for pipeline orchestration
- provider interfaces isolate vendor-specific request/response logic
- model configuration, retries, and metadata collection stay out of orchestration code

### File and Module Boundaries

Do not continue adding logic into:

- `productimage/default_components.go`
- `productimage/scene_renderer.go`

Instead split by responsibility:

- provider contracts
- faithful edit adapter
- scene generation adapter
- model review adapter
- orchestration helpers

This keeps rendering logic, prompt construction, metadata capture, and pipeline control from collapsing into a few oversized files.

## Pipeline Design

### Proposed Stages

The primary processing flow becomes:

1. `understand_product`
2. `extract_subject_model`
3. `render_white_bg_model`
4. `render_scene_model`
5. `review_generation_model`
6. `rule_validation_guard`

### Stage Semantics

#### `understand_product`

Uses vision and text models to build product context and structured attributes.

#### `extract_subject_model`

Uses the faithful editor to produce a subject-preserving extracted asset.

Outputs:

- subject image
- optional mask or edit metadata

#### `render_white_bg_model`

Uses the faithful editor to create white-background imagery suitable for main listing use.

Outputs:

- white-background image
- provenance metadata

#### `render_scene_model`

Uses the scene generator to produce scene or selling-point assets.

Inputs:

- subject image or source reference
- product context
- style preset
- slot/platform constraints

#### `review_generation_model`

Uses a model to assess qualitative output fitness.

Outputs:

- review decision
- confidence
- retry hints
- reason codes

#### `rule_validation_guard`

Runs strict non-subjective checks and can block invalid outputs even when model review passes.

## ListingKit Integration

`listingkit` does not need a protocol rewrite, but its task execution metadata must become model-aware.

### Execution Metadata Changes

Generation task records should explicitly carry:

- execution mode
- model family
- generation mode
- source asset references
- prompt or preset references
- review confidence
- provider lineage

The old meaning of `renderer_backed` is no longer sufficient. The execution record needs a model-oriented distinction such as:

- faithful_edit_backed
- scene_generation_backed

### Preview and Review Compatibility

Existing `listingkit` constructs remain usable:

- generation queue
- review session
- review preview
- dispatch
- recovery

The change is mostly beneath those contracts: richer generation metadata and clearer model-originated review reasons.

## Style Control

Introduce a unified style contract for scene generation:

- `style_family`
- `tone`
- `background_style`
- `layout_density`
- `scene_intent`
- `preserve_product_identity`

Rules:

- white-background and primary product outputs default to high identity preservation
- scene outputs may be more flexible
- selling-point outputs use constrained presets, not arbitrary freeform prompting

This keeps image generation controllable and avoids turning commerce outputs into unbounded prompt art.

## Error Handling

### Provider Failures

Provider failures should be explicit and recoverable:

- upstream model failure
- malformed model response
- identity preservation failure
- review rejection
- asset persistence failure

These failures should map into existing task/recovery surfaces instead of introducing a separate error channel.

### Fallback Policy

Local canvas rendering is not a production success path. It may exist only as:

- development fallback
- controlled emergency fallback behind explicit configuration

It must not silently replace model generation in normal production execution, otherwise the architecture regresses to the current state.

## Testing Strategy

### Unit Tests

Add focused tests for:

- provider adapters
- prompt and preset mapping
- execution metadata mapping
- review result translation
- fallback gating

### Integration Tests

Add integration coverage for:

- faithful editor path
- scene generator path
- listingkit generation task metadata
- review and retry behavior

### End-to-End Validation

Required scenarios:

- uploaded image to white-background output
- uploaded image to scene output
- 1688/product URL to generated assets
- failed model generation to operator recovery path

## Migration Plan

### Phase 1

Replace primary `SceneRenderer` with a model-backed implementation.

Reason:

- this removes the clearest local-canvas generation behavior first
- it yields immediate alignment with the product requirement

### Phase 2

Replace `WhiteBackgroundRenderer` with a faithful editor implementation.

### Phase 3

Replace `SubjectExtractor` with a faithful model-backed extractor.

### Phase 4

Replace review assessment with model-first review plus rule guards.

This sequence reduces blast radius and keeps each rollout attributable.

## Risks

- identity drift in scene and edit models
- provider latency and quota instability
- increased generation cost
- overloading `listingkit` metadata without tightening schemas
- accidental silent fallback to local rendering

The main architectural guardrail is that local rendering must no longer be treated as an equal success path.

## Open Decisions Deferred from This Spec

These are intentionally left for implementation planning, not design ambiguity:

- exact vendor/model selection
- prompt asset storage format
- per-provider retry budgets
- whether masks are stored as first-class assets
- exact naming of new execution modes in persisted task metadata

The architecture above is stable without locking those implementation details now.
# Implementation tracking

Implementation status is tracked in:

- `D:\code\task-processor\docs\superpowers\plans\2026-04-19-model-driven-productimage.md`
