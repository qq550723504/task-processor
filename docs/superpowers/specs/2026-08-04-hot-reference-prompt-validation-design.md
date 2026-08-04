# Hot-reference generation prompt validation

## Problem

SHEIN Studio batches using exactly one hot-selling reference image are allowed to omit the user-facing theme prompt. The batch layer already recognizes this mode, but the lower-level sync and async generation services reject an empty prompt before the hot-reference prompt builder runs. This leaves the batch item failed with `invalid request: prompt is required`.

## Design

Keep the existing mode contract: `theme_prompt` requires a non-empty prompt; `hot_reference` requires exactly one normalized reference image and may have an empty prompt. Centralize this decision in the shared generation validation path so batch execution, direct synchronous generation, and asynchronous generation cannot disagree.

The generated request will continue to use `buildHotReferenceStudioDesignPrompt` to create the internal safety and artwork instructions. The user-facing prompt remains optional in hot-reference mode and, when present, is treated only as supplemental theme guidance.

## Testing

- Add a service-level regression test proving promptless `hot_reference` generation reaches the image generator and succeeds.
- Keep the existing prompt-required behavior for ordinary theme generation.
- Run the focused ListingKit tests, then the broader `internal/listingkit` package tests and `git diff --check`.

## Scope

Only generation validation and its regression coverage change. No batch data migration, production retry, UI mutation, or unrelated refactor is included.
