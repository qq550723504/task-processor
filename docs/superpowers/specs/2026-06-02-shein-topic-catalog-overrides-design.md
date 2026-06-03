# SHEIN Topic Catalog Tenant Overrides Design

## Status

Status as of 2026-06-04: Design largely realized in implementation.

The recommended two-layer model from this design now exists in practice:

1. Platform default topic catalog in code
2. Tenant-level override records in the database

The merged runtime path is now consumed by both prompt-policy assembly and sanitizer lexicon overlay logic.
This file remains useful as design history, but it is no longer a speculative future-state document.

## Background

Current SHEIN generation topics are defined in code at `internal/shein/generationtopics/registry.go`.

This gives us a stable platform-level baseline for:

- prompt directives used during AI generation
- lexicon overlays used by the post-generation sanitizer

At the moment, admin users can only enable or disable `topic_key` values per tenant through `listing_generation_topic_policy`.
They cannot inspect the detailed words behind each key in the UI, and they cannot customize topic behavior for a tenant.

We want to add tenant-level customization without losing the platform baseline or creating multiple drifting sources of truth.

## Goals

- Keep a single platform default topic catalog for SHEIN.
- Expose that catalog to the admin UI through a read-only API.
- Allow each tenant to add override content per topic key.
- Reuse the same merged topic definition for:
  - AI prompt policy summaries
  - sanitizer lexicon overlays
- Avoid making platform baseline rules deletable from the UI in the first version.

## Non-Goals

- Do not move the platform default topic catalog from code into the database.
- Do not allow deleting or mutating the platform default words in the first version.
- Do not add non-SHEIN platform support in this iteration.
- Do not add a free-form topic-key creation flow in the first version.

## Recommended Approach

Use a two-layer model:

1. Platform default topic catalog in code
2. Tenant-level override records in the database

The runtime merges the two layers when building prompt summaries and sanitizer lexicons.

This keeps the default catalog stable while still allowing tenant-specific customization.

## Data Model

Add a new tenant-scoped table for topic overrides.

Suggested table: `listing_generation_topic_override`

Suggested fields:

- `id`
- `tenant_id`
- `platform`
- `topic_key`
- `additional_prompt_directives_json`
- `additional_lexicon_json`
- `status`
- `remark`
- `creator`
- `updater`
- `created_at`
- `updated_at`
- `deleted`

Notes:

- `additional_prompt_directives_json` stores an array of strings.
- `additional_lexicon_json` stores a map like `{ "en": ["foo"], "zh": ["条目"] }`.
- Unique active row constraint should be `(tenant_id, platform, topic_key)`.
- First version only supports `platform = shein`.

## Runtime Merge Rules

For a given tenant and `topic_key`:

1. Load the platform default definition from `generationtopics`.
2. Load the tenant override row if it exists and is enabled.
3. Merge as follows:
   - `PromptDirectives` = default directives + tenant additional directives
   - `LexiconByLanguage` = default lexicon + tenant additional lexicon words
4. Deduplicate merged directives and words.
5. Preserve deterministic ordering:
   - default items first
   - tenant additions second

If no override row exists, the default definition is used as-is.

## API Design

### 1. Read-only Topic Catalog

Add a new admin endpoint:

- `GET /api/v1/listing-kits/admin/generation-topic-catalog?platform=shein`

Response should include:

- `key`
- `priority`
- `promptDirectives`
- `lexiconByLanguage`
- `tenantOverride`
  - `id`
  - `status`
  - `remark`
  - `additionalPromptDirectives`
  - `additionalLexiconByLanguage`
- `effectiveDefinition`
  - merged directives
  - merged lexicon

This endpoint becomes the source of truth for the admin page.

### 2. Tenant Override CRUD

Add admin endpoints for override management:

- `GET /api/v1/listing-kits/admin/generation-topic-overrides`
- `GET /api/v1/listing-kits/admin/generation-topic-overrides/:id`
- `POST /api/v1/listing-kits/admin/generation-topic-overrides`
- `PUT /api/v1/listing-kits/admin/generation-topic-overrides/:id`
- `PATCH /api/v1/listing-kits/admin/generation-topic-overrides/:id/status`
- `DELETE /api/v1/listing-kits/admin/generation-topic-overrides/:id`

Input validation:

- `platform` required and must be `shein`
- `topic_key` required and must exist in the platform catalog
- `additional_prompt_directives_json` must decode to string array
- `additional_lexicon_json` must decode to map of language -> string array

## UI Changes

Update `/listing-kits/admin/generation-topic-policies` to load the catalog API and show:

- topic key
- default prompt directives
- default lexicon by language
- tenant override additions
- effective merged preview

The form should no longer hardcode topic-key options.
Instead it should read available topic keys from the catalog API.

For each topic key, the page should support:

- viewing default definition
- viewing current tenant override
- adding tenant prompt directives
- adding tenant lexicon words by language

First version editing scope:

- append/replace tenant additional directives
- append/replace tenant additional lexicon
- enable/disable override row

First version should not support:

- deleting platform default directives
- deleting platform default lexicon words
- creating brand-new topic keys

## Runtime Integration

The merged topic definition should be reused in both paths:

1. Prompt generation path
   - tenant prompt summary builder uses merged directives
2. Sanitizer path
   - tenant lexicon overlay uses merged lexicon

This avoids divergence between "generate-time avoidance" and "post-generation cleanup".

## Error Handling

- Unknown `topic_key` in override create/update should return `400`.
- Invalid JSON structure for directives or lexicon should return `400`.
- Missing repository/runtime dependency should return `503`.
- Disabled override rows should be ignored during runtime merge.
- If override loading fails at runtime, fall back to platform defaults and log the error.

## Testing Strategy

### Backend

- topic catalog endpoint returns default definitions from code
- override create/update validates topic key existence
- merged definition returns default + tenant additions
- prompt summary builder includes tenant additional directives
- sanitizer overlay includes tenant additional lexicon words
- disabled override rows do not affect runtime behavior

### Frontend

- page loads topic catalog from API
- topic key selector is populated from API, not hardcoded
- catalog section renders default words and directives
- override editor saves additional directives and lexicon
- effective preview updates from merged API response

### Integration

- tenant A and tenant B can define different additions for the same topic key
- AI prompt for tenant A includes only tenant A additions
- sanitizer for tenant B includes only tenant B additions

## Rollout Notes

This feature should be introduced without removing the existing `listing_generation_topic_policy` behavior.

Recommended rollout order:

1. add catalog read API
2. add override storage and CRUD
3. switch prompt and sanitizer merge logic to consume overrides
4. update admin page to show catalog and overrides

## Risks

- Letting tenant users fully rewrite topic definitions could weaken platform safety rules.
- Putting the entire catalog in DB would increase drift risk and operational complexity.
- Prompt size could grow if tenant additional directives are not capped.

Mitigations:

- keep defaults in code
- allow additive overrides only in v1
- cap total prompt summary size using existing directive limits

## Recommendation

Implement a read-only platform catalog plus tenant additive overrides.

This solves the immediate need to inspect and customize topic behavior while preserving a stable baseline and avoiding duplicate definitions across frontend and backend.
