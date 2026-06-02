# SHEIN Tenant Generation Topics Design

## Background

SHEIN currently has a tenant-aware sensitive-word sanitizer that removes risky wording after AI generation, during draft review, before final submit, and during validation-note retry cleanup. This is reliable for deterministic cleanup, but it does not stop the model from generating prohibited themes in the first place.

Some tenants need generation-time avoidance for topic families such as children, babies, food, meals, and knives. Those restrictions vary by tenant. Injecting each tenant's full sensitive-word list into prompts would create unstable prompt behavior and context bloat.

This design introduces a small tenant-scoped "generation topics" policy layer for AI prompt steering while continuing to reuse the existing SHEIN sensitive-word service as the deterministic fallback.

## Goals

- Let each tenant enable different prohibited generation topics for SHEIN AI copy generation.
- Keep prompt context small and stable.
- Reuse the existing tenant-aware sensitive-word sanitizer for post-generation cleanup.
- Avoid parallel policy systems that drift apart.

## Non-Goals

- Replacing the existing `listing_sensitive_word` sensitive-word flow.
- Supporting arbitrary tenant-authored prompt text in the first version.
- Supporting every platform in the first version.
- Building a generic admin UI for topic editing in the first version.

## Problem Summary

The current SHEIN prompt chain has two relevant behaviors:

1. Prompt-like source text is cleaned or structurally rewritten for title extraction.
2. AI-generated title and description are cleaned after generation by the sanitizer.

This means the system is safe after generation, but not proactive during generation. If a tenant wants to avoid entire semantic areas such as baby-related or food-related wording, we need a small prompt-time policy summary rather than a large per-tenant lexical dump.

## Proposed Approach

Use two coordinated layers:

1. Tenant generation-topic policy
   - Stores which topic keys are enabled for a tenant and platform.
   - Used only to build a compact prompt restriction summary.

2. Existing sensitive-word cleanup
   - Reused as the deterministic fallback after generation and before submit.
   - Topic definitions contribute their lexical bundles into the same sensitive-word service when enabled for the tenant.

This gives us:

- Prompt-time avoidance with small context.
- Deterministic post-generation cleanup with the current infrastructure.

## Data Model

Add a new table for tenant generation-topic policy.

Suggested schema:

- `id`
- `tenant_id`
- `platform`
- `topic_key`
- `status`
- `remark`
- `created_at`
- `updated_at`

Suggested constraints:

- Unique key on `(tenant_id, platform, topic_key)`
- First version supports `platform = "shein"` only

Suggested semantics:

- `status = 1` means enabled
- `status = 0` means disabled

## Topic Definition Registry

Topic definitions are stored in code, not in the database, in the first version.

Example shape:

```go
type GenerationTopicDefinition struct {
	Key               string
	PromptDirectives  []string
	LexiconByLanguage map[string][]string
	Priority          int
}
```

Example topic keys:

- `children`
- `baby`
- `food`
- `meals`
- `knives`

Each topic definition has two responsibilities:

1. `PromptDirectives`
   - Short, model-facing restrictions
   - Example: `Do not mention children, babies, or age-specific users.`

2. `LexiconByLanguage`
   - Post-generation lexical fallback used by the sanitizer
   - Contains English and Chinese variants, plus obvious inflections where needed

Why code constants still allow tenant customization:

- The database decides which `topic_key` values each tenant enables.
- The code registry defines what each standard `topic_key` means.

This keeps tenant configuration small while keeping topic behavior consistent.

## Runtime Flow

### Prompt-Time Flow

When a SHEIN AI prompt is built:

1. Read tenant-scoped enabled topic keys for platform `shein`.
2. Resolve them through the topic definition registry.
3. Build a compact prompt restriction summary from `PromptDirectives`.
4. Inject the summary into the relevant SHEIN AI prompts.

Prompt summary constraints:

- Deduplicate directives.
- Limit total directive count, for example `<= 5`.
- Limit total character count, for example `<= 600`.
- Truncate by topic priority if needed.

This prevents prompt bloat even if a tenant enables many topics.

### Post-Generation Flow

When `NewSensitiveWordServiceForContext(ctx)` builds the SHEIN sensitive-word service:

1. Load tenant-scoped `listing_sensitive_word` records as it does today.
2. Load enabled generation-topic keys for the tenant and platform.
3. Resolve each topic's `LexiconByLanguage`.
4. Overlay those words into the service as additional static words.

This means the existing sanitizer automatically gains topic-level fallback coverage without new downstream branching.

## Integration Points

First-version prompt injection points:

1. `internal/publishing/shein/submit_prep.go`
   - `optimizeSubmitContentWithAI(...)`
   - Review/submit-stage AI optimization for title and description

2. `internal/publishing/shein/title_resolution.go`
   - Prompt-like title extraction and title-addition extraction helpers
   - Used when noisy prompt text contaminates title candidates

First-version sanitizer integration point:

1. `internal/shein/submitprep/sensitive_words.go`
   - Extend `NewSensitiveWordServiceForContext(ctx)` to overlay enabled topic lexicons

Because the sanitizer is already reused by preview, review, submit, and validation retry, topic lexicons automatically apply to:

- Listing preview copy cleanup
- Draft payload cleanup
- Submit-time cleanup
- Validation retry cleanup

## Proposed Components

Add a small set of focused components:

1. Topic repository
   - Reads tenant-enabled topic keys from the new table

2. Topic registry
   - Code-defined map of topic metadata and lexicons

3. Prompt summary builder
   - Converts enabled topics into a compact prompt policy block

4. Topic overlay helper
   - Adds enabled topic lexicons into the existing sensitive-word service

## Error Handling

If topic policy lookup fails:

- Do not fail SHEIN generation.
- Skip prompt policy injection for that request.
- Continue using the existing sensitive-word sanitizer and JSON or DB-backed word loading.

If topic lexicon overlay fails:

- Do not fail SHEIN generation or submit.
- Fall back to existing tenant sensitive-word behavior.

This keeps the new policy layer additive rather than availability-critical.

## Testing Strategy

Add tests for:

1. Prompt summary generation
   - Enabled topics produce the expected compact directives
   - Duplicate directives are removed
   - Character and count limits are enforced

2. Tenant variance
   - Different tenants produce different prompt summaries
   - Disabled topics do not appear

3. Sensitive-word overlay
   - Enabled topic lexicons are added to the service
   - Existing tenant sensitive words still load
   - Overlay does not duplicate words excessively

4. End-to-end SHEIN prompt consumers
   - `optimizeSubmitContentWithAI(...)` receives topic policy text
   - `title_resolution` prompt helpers receive topic policy text

5. Existing sanitizer compatibility
   - Preview, draft, submit, and retry cleanup still work
   - Topic lexicons remove blocked words if the model still generates them

## Migration Plan

Phase 1:

- Add tenant topic policy table and repository
- Add topic registry in code
- Add prompt summary builder
- Inject prompt summaries into the two SHEIN AI prompt entry points
- Overlay topic lexicons into `NewSensitiveWordServiceForContext(ctx)`

Phase 2:

- Add admin or internal-management surfaces if needed
- Consider expanding to additional platforms after SHEIN stabilizes

## Risks

### Risk: Topic definitions are too broad

If a topic lexicon is too aggressive, generated text may be over-cleaned.

Mitigation:

- Keep the first topic set small
- Review lexicons carefully
- Add tests for false-positive-sensitive terms

### Risk: Prompt policies conflict with product semantics

Some products may naturally include restricted topic language in source data.

Mitigation:

- Prompt directives should tell the model to omit restricted themes instead of paraphrasing into adjacent risky wording
- Post-generation sanitizer remains the fallback

### Risk: Topic list grows over time

Too many enabled topics could still enlarge prompt context.

Mitigation:

- Hard cap prompt directive count and total characters
- Use topic priority ordering

## Recommendation

Implement tenant-specific SHEIN generation restrictions as:

- Database-configured enabled topic keys per tenant
- Code-defined topic registry for canonical meaning
- Prompt-time summary injection for compact guidance
- Existing sensitive-word service overlay for deterministic lexical fallback

This approach solves tenant variance and context-size concerns without replacing the current sanitizer architecture.

## Implementation Notes

- Prompt summaries are capped at five directives or 600 characters, whichever comes first.
- Prompt injection currently prefixes the compact summary with `Additional tenant content restrictions:`.
- Unknown or unsupported topic keys are ignored instead of being passed through to the model.
- Topic lexicons are overlaid as static words inside `NewSensitiveWordServiceForContext(ctx)`.
- First version supports only `platform = "shein"`.
