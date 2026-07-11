# ListingKit Studio Reference Analysis Boundary Design

## Status

Approved design direction. This document defines the first targeted `internal/listingkit` ownership reduction after the current boundary checkpoint.

## Goal

Extract the pure reference-style interpretation and safety policy from `internal/listingkit/studio_reference_analysis.go` into a focused package under `internal/listing/studio`, while keeping ListingKit as the unchanged API and orchestration facade.

The refactor must preserve current behavior exactly. It must not change the SDS POD production flow, HTTP contracts, task persistence, Temporal behavior, image resolution, AI invocation, or SHEIN submission behavior.

## Problem

`internal/listingkit/studio_reference_analysis.go` currently combines two responsibilities with different owners and change rates:

1. ListingKit orchestration:
   - request validation;
   - uploaded-image and public-URL resolution;
   - AI client availability and invocation;
   - ListingKit response and error translation.
2. Stable reference-analysis policy:
   - parsing structured and malformed AI output;
   - abstracting reusable visual signals;
   - filtering protected identities, brand marks, exact artwork, watermarks, and suspicious named phrases;
   - constructing the reference style brief and sanitized generation prompt;
   - deciding whether safe reusable signals remain and whether warnings are required.

The second group is platform-neutral studio policy. Keeping it in the ListingKit root makes ListingKit own rules that can be tested and evolved independently, and makes a 1,218-line orchestration file the only home for a substantial pure transformation pipeline.

The root cause is mixed ownership, not file size. This design moves only the stable policy boundary and leaves runtime-dependent behavior in place.

## Prior Art and Reuse

Use the established functional-core/imperative-shell pattern rather than introducing a new framework:

- ListingKit remains the imperative shell that performs I/O and translates API-facing errors.
- `internal/listing/studio/referenceanalysis` becomes the deterministic functional core.
- Existing `encoding/json`, `regexp`, and string-processing behavior is retained.
- Existing characterization tests are reused as the compatibility oracle.

No dependency injection framework, generic policy engine, plugin system, or third-party content-moderation abstraction is introduced. The rules are domain-specific and already implemented; this refactor changes ownership, not algorithms.

## Scope

### Move to `internal/listing/studio/referenceanalysis`

The new package owns:

- the structured analysis model used to decode AI output;
- the abstracted safe-style model;
- structured and malformed-output parsing;
- vocabulary and pattern definitions used solely by reference-style interpretation;
- motif, palette, typography, density, product-fit, mood, placement, and composition abstraction;
- protected-identity and unsafe-signal removal;
- safe-signal availability decisions;
- style-brief construction;
- sanitized-prompt construction;
- policy outcomes that indicate unsafe or malformed input.

### Remain in `internal/listingkit`

ListingKit continues to own:

- `AnalyzeStudioReferenceStyle` service entrypoints;
- `StudioReferenceAnalysisRequest` and `StudioReferenceAnalysisResponse` compatibility DTOs;
- request-count validation and the existing single-image limit;
- reference URL normalization where it is part of the API contract;
- uploaded asset lookup and public HTTPS URL resolution;
- `promptDiversifier.AnalyzeImage` calls;
- current public error codes/messages and Chinese warning strings;
- conversion from the pure policy result to the existing response.

The exact URL-helper boundary may remain in ListingKit even if some helpers are pure, because those helpers participate in ListingKit upload and object-store conventions. They are not part of the first extraction.

## Package API

The new package exposes this small operation-oriented API:

```go
package referenceanalysis

type Result struct {
    StyleBrief       string
    SanitizedPrompt  string
    HadUnsafeInput   bool
    HadMalformedInput bool
}

var (
    ErrNoInput         = errors.New("reference analysis input is empty")
    ErrNoSafeDirection = errors.New("reference analysis has no reusable safe style direction")
    ErrEmptyPrompt     = errors.New("reference analysis generated an empty prompt")
)

func Interpret(rawAnalyses []string) (Result, error)
```

`Interpret` receives raw AI responses so JSON parsing, malformed-result recovery, abstraction, filtering, and output construction stay in one cohesive pure pipeline. It performs no network, storage, logging, task, marketplace, or persistence work.

The package error contract distinguishes only the policy failures needed by the caller:

- `ErrNoInput` for no analyzable input;
- `ErrNoSafeDirection` when no reusable safe style direction survives;
- `ErrEmptyPrompt` when the sanitized prompt is empty.

ListingKit maps these failures to the existing public error strings. The new package must not define HTTP status codes or ListingKit DTOs.

## Data Flow

```text
StudioReferenceAnalysisRequest
  -> ListingKit validates request and resolves image URLs
  -> ListingKit invokes the existing AI image analyzer
  -> raw AI response strings
  -> referenceanalysis.Interpret
       -> parse or conservatively recover
       -> abstract reusable visual signals
       -> remove unsafe/protected signals
       -> build style brief and sanitized prompt
       -> report unsafe/malformed policy flags
  -> ListingKit maps flags to existing warnings
  -> StudioReferenceAnalysisResponse
```

Failed AI calls remain an orchestration concern. ListingKit keeps the current behavior of collecting successful results, emitting compact failure warnings, and failing only when no image can be analyzed.

## Dependency Rules

`internal/listing/studio/referenceanalysis` may depend only on the Go standard library and, if genuinely necessary, another platform-neutral `internal/listing/studio` type.

It must not import:

- `internal/listingkit`;
- `internal/marketplace/*` or `internal/publishing/*`;
- `internal/app`, `internal/platform`, or `internal/infra`;
- Gin, GORM, Temporal, object-storage clients, or AI clients;
- SDS runtime or login packages.

Add an import-boundary guard so these constraints are executable rather than documentary.

## Error Handling and Compatibility

Public behavior is frozen for this slice:

- existing request validation messages remain unchanged;
- existing `reference_analysis_unavailable` and `reference_analysis_failed` messages remain unchanged;
- existing warning strings and their conditions remain unchanged;
- malformed output keeps the current conservative fallback behavior;
- unsafe content is removed under the same rules;
- requests with no surviving safe signal still fail;
- response JSON shape remains unchanged.

The pure package may use typed or sentinel internal errors. ListingKit is responsible for translating them to the existing public messages, preventing domain errors from leaking into the API contract.

## Testing Strategy

### Characterization first

Before moving implementation, preserve representative tests for:

- valid structured JSON;
- malformed JSON recovery;
- safe off-vocabulary cues;
- protected brands, people, characters, exact artwork, and watermarks;
- safe title-case phrases versus suspicious named phrases;
- mood and garment placement;
- no safe signals surviving;
- unsafe and malformed warning conditions.

These cases become table-driven unit tests in `internal/listing/studio/referenceanalysis` and must assert exact output strings and policy flags.

### ListingKit facade tests

Keep focused ListingKit tests for:

- request and image-count validation;
- public HTTPS and uploaded-image URL resolution;
- unavailable analyzer behavior;
- partial and total AI failure behavior;
- mapping policy flags to the current warnings;
- unchanged response DTOs and public errors.

Large existing ListingKit tests can be reduced only after equivalent policy coverage exists in the new package. Test movement must not reduce behavioral coverage.

### Verification

Run, at minimum:

```powershell
go test ./internal/listing/studio/... -count=1
go test ./internal/listingkit/... -count=1
go test ./tests/... -count=1
```

Then run `go test ./... -count=1` before completion. If repository-wide tests contain an unrelated known instability, record the exact failure and ensure all affected packages pass independently.

## Migration Sequence

1. Add characterization tests for the new pure package using current outputs.
2. Implement `referenceanalysis.Interpret` by moving existing policy without rewriting it.
3. Add the package import-boundary guard.
4. Change ListingKit orchestration to call the new package.
5. Keep compatibility mapping for current errors and warnings.
6. Remove the now-duplicated private policy implementation from ListingKit.
7. Run focused and repository-wide verification.
8. Update the ListingKit boundary checkpoint with the completed ownership move.

Each step should be mechanically reviewable. Algorithm changes, vocabulary changes, and new safety rules are out of scope and belong in later behavior PRs.

## Non-Goals

- Refactoring the SDS POD synchronization or generation flow.
- Moving ListingKit task, batch, repository, or persistence ownership.
- Changing Temporal workflows or activity retry semantics.
- Changing AI prompts, model selection, or analyzer interfaces.
- Changing URL or object-store behavior.
- Moving all Studio code out of ListingKit.
- Refactoring SHEIN pricing, submission, preview, or workspace behavior.
- Reducing file counts through cosmetic splitting.

## Risks and Mitigations

### Accidental output drift

Moving interdependent parsing and sanitization helpers can subtly change ordering or whitespace.

Mitigation: exact-string characterization tests run before and after the move; do not rewrite algorithms during extraction.

### Error contract leakage

New internal error text could accidentally reach HTTP clients.

Mitigation: ListingKit retains explicit error translation and facade tests assert current public messages.

### Hidden runtime coupling

A seemingly pure helper may depend on ListingKit URL or request conventions.

Mitigation: leave URL normalization, upload resolution, prompts, and runtime calls in ListingKit during this slice.

### Stable SDS flow regression

Studio reference analysis may be used by a broader SDS production workflow.

Mitigation: do not alter SDS orchestration; run existing ListingKit/SDS regression tests and keep the public service method unchanged.

## Success Criteria

The slice is complete when:

- reference interpretation and safety policy live under `internal/listing/studio/referenceanalysis`;
- the new package has no dependency on ListingKit or runtime/marketplace packages;
- ListingKit retains only orchestration, compatibility DTOs, runtime calls, URL handling, and public error/warning translation for this feature;
- current outputs, errors, and warnings remain unchanged;
- SDS POD, Temporal, persistence, and submission code are untouched;
- focused tests, boundary tests, and repository-wide verification pass;
- the ListingKit boundary checkpoint records the ownership reduction.
