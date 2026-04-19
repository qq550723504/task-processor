# Prompt Observability Design

## Summary

This design adds prompt observability to the runtime metadata produced by the model-backed generation pipeline.

The immediate goal is operational visibility, not UI productization. The system should expose enough prompt lineage in backend task and asset metadata to answer:

- which prompt key was used
- whether the prompt came from the registry or a fallback string
- which prompt version identifier was attached at generation time

This work is intentionally limited to backend runtime metadata. It does not add new UI fields, dashboards, or editing workflows.

## Goals

- Make prompt lineage explicit in model-backed generation metadata.
- Preserve that lineage across `productimage`, `asset/generation`, and `listingkit`.
- Distinguish registry-backed prompt execution from code-fallback prompt execution.
- Keep the implementation compatible with current task/result contracts.
- Improve production debugging for prompt-related regressions without requiring log forensics.

## Non-Goals

- Adding prompt observability UI in ListingKit.
- Building a prompt version management product.
- Replacing existing provenance structures such as `productenrich.FieldTrace`.
- Migrating every model-using subsystem in one pass.
- Introducing a new persistence store dedicated to prompt metadata.

## Current State

Prompt usage is partially observable today:

- `productimage` already records `prompt_ref` in generation metadata and asset metadata.
- `productimage` and `productenrich` now read primary prompt content from the prompt registry when available.
- Old slash-style prompt refs are normalized to registry-style keys.

However, important runtime evidence is still missing:

- there is no explicit `prompt_source`
- there is no explicit `prompt_version`
- `prompt_ref` alone does not tell whether registry rendering succeeded or whether the fallback string was used
- prompt lineage is not consistently propagated through all downstream task/result metadata

This makes debugging prompt regressions slower than necessary, especially once multiple model providers and prompt migrations coexist.

## Selected Approach

Add prompt observability as an extension of existing runtime metadata, starting from `productimage.GenerationMetadata`, then mirror those fields into downstream asset and task metadata.

The design uses three explicit fields:

- `prompt_key`
- `prompt_source`
- `prompt_version`

The initial source vocabulary is intentionally small:

- `registry`
- `fallback`

The initial versioning model is also intentionally simple:

- a stable string stored at generation time
- no centralized prompt version registry is required in this phase

## Metadata Model

### New Fields

Model-backed generation metadata should carry:

- `PromptKey`
- `PromptSource`
- `PromptVersion`

These fields complement, rather than replace:

- `PromptRef`
- `ModelProvider`
- `ModelFamily`
- `GenerationMode`

### Field Semantics

#### `prompt_key`

The normalized prompt registry key associated with the operation.

Examples:

- `productimage.subject.extract`
- `productimage.white_background.default`
- `productimage.scene.default`
- `productimage.review.default`

If a legacy slash-style key is provided, it is normalized before storage.

#### `prompt_source`

The effective source of the rendered prompt content.

Allowed values in this phase:

- `registry`
- `fallback`

`registry` means:

- the registry was available
- the key resolved
- template rendering succeeded

`fallback` means any of:

- registry unavailable
- key missing
- template rendering failed

#### `prompt_version`

A stable string associated with the prompt content at generation time.

This phase does not introduce a central versioning system. The value may initially be:

- an explicit version string attached by the prompt template helper
- or a stable default placeholder such as `default`

The important property is that the field exists and is persisted consistently, so versioning can evolve later without changing contracts again.

## System Boundaries

### ProductImage

`productimage` is the authoritative source of prompt observability for model-backed image generation.

Responsibilities:

- determine normalized prompt key
- determine whether prompt content came from registry or fallback
- attach prompt version
- preserve these fields in `GenerationMetadata`
- mirror them into emitted asset metadata

Applicable operations:

- subject extraction
- white-background generation
- scene generation
- review prompt execution

### Asset Generation

`asset/generation` must preserve prompt observability when it converts model-backed results into persisted task records.

Responsibilities:

- carry forward `prompt_key`, `prompt_source`, and `prompt_version`
- keep them alongside existing model metadata
- avoid collapsing them back into only `prompt_ref`

### ListingKit

`listingkit` must retain prompt observability when consuming generated assets and generation tasks.

Responsibilities:

- preserve prompt metadata in task/result structures
- not strip prompt lineage from preview/review-related metadata paths

This phase does not require new top-level API fields. Reusing existing metadata maps is sufficient.

### ProductEnrich

`productenrich` should not force prompt lineage into `FieldTrace`.

Reason:

- `FieldTrace` models product provenance and inference
- prompt lineage is execution metadata
- mixing them would blur two distinct concepts

For this phase, prompt observability in `productenrich` should be added only where runtime generation metadata already exists or can be added without distorting canonical provenance structures.

## Prompt Resolution Rules

Prompt resolution should produce an explicit outcome object in effect, even if it is not exposed as a standalone type:

- normalized key
- source classification
- version string
- rendered text

Resolution rules:

1. Normalize the requested key.
2. Attempt registry render.
3. If registry render succeeds:
   - source = `registry`
4. If registry render fails or registry is unavailable:
   - source = `fallback`
5. Always persist the normalized key and chosen source.
6. Always attach a version string.

This removes ambiguity that currently exists when a prompt falls back silently.

## Storage and Propagation

### In-Memory Runtime Structures

Extend `productimage.GenerationMetadata` to carry the new fields.

Any clone/copy logic must preserve them.

### Asset Metadata

Model-produced asset records should receive mirrored metadata entries:

- `prompt_key`
- `prompt_source`
- `prompt_version`

This ensures downstream consumers can inspect prompt lineage even when they do not deserialize the richer typed metadata object.

### Persisted Task Metadata

Generation task persistence should also retain these fields so operators can inspect prompt lineage at the task level, not only the final asset level.

### ListingKit Result Paths

When ListingKit builds generation queues, previews, and review state, prompt metadata should remain present in the relevant generation-task and asset metadata maps.

This phase does not require new result-specific structs unless existing metadata paths cannot carry the fields cleanly.

## Compatibility

### Legacy Prompt References

Legacy slash-style references remain supported.

Examples:

- `productimage/scene/default`
- `productimage/white-background/default`

They are normalized before:

- registry lookup
- metadata persistence
- downstream propagation

### Backward Compatibility

Existing consumers that only read `prompt_ref` continue to work.

The new fields are additive:

- no contract removal
- no required UI change
- no required migration for existing callers

## Error Handling

Prompt observability must remain available even on degraded prompt execution paths.

Expected degraded cases:

- registry unavailable
- key not found
- template render failure

These should not suppress metadata. Instead:

- `prompt_source` records `fallback`
- `prompt_key` records the normalized requested/default key
- `prompt_version` still records a stable value

This is essential for debugging fallback-heavy systems.

## Testing Strategy

### Unit Tests

Add focused tests for:

- registry hit sets `prompt_source=registry`
- fallback path sets `prompt_source=fallback`
- normalized prompt key is persisted
- cloned metadata preserves new fields

### Integration Tests

Add integration coverage for:

- `productimage` asset metadata propagation
- `asset/generation` task metadata propagation
- `listingkit` preservation of prompt lineage in generation task/asset metadata

### Runtime Verification

Run at least one real model-backed task after implementation and inspect:

- asset metadata
- generation task metadata
- resulting ListingKit-consumable metadata

The runtime evidence should show the new fields without relying on logs.

## Risks

- introducing inconsistent field names between typed metadata and metadata maps
- filling only some model-backed paths and creating partial observability
- overreaching into provenance models that serve a different purpose

The main design guardrail is:

- prompt observability is execution metadata, not canonical product provenance

## Implementation Notes

This phase should stay focused and avoid:

- new UI work
- prompt editing workflows
- version registry infrastructure
- broad metadata schema redesign

The desired outcome is a minimal but durable observability layer that future prompt tooling can build on.
