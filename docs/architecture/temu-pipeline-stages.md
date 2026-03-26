# TEMU Pipeline Stages

## Goal

This note captures the stage language that has emerged while refactoring the
TEMU flows. The purpose is to give future refactors a stable vocabulary, so new
changes can align with the existing structure instead of introducing another
ad-hoc flow.

## SKU Main Chain

The SKU / AI response flow is converging on this order:

1. `generate`
2. `finalize`
3. `prepare`
4. `cleanup / apply`
5. `build`
6. `assign`

### 1. Generate

Responsible for producing AI mapping data.

Representative entry points:

- `GenerateAISkuMapping(...)`
- `generateAISkuMappingSingle(...)`
- `generateAISkuMappingInBatches(...)`

### 2. Finalize

Responsible for post-generation normalization that belongs to the generated
response itself, before build preparation starts.

Representative steps:

- `finalizeGeneratedAIMapping(...)`
- `finalizeMergedAIMapping(...)`
- `normalizeGeneratedAIMapping(...)`
- `normalizeMergedAIMapping(...)`

### 3. Prepare

Responsible for making AI mapping build-ready.

Representative steps:

- `prepareAIMappingForBuild(...)`
- `prepareDefaultAIMappingForBuild(...)`
- `ensureAIMappingMatchesVariants(...)`
- `normalizeAIMappingForBuild(...)`

### 4. Cleanup / Apply

Responsible for correcting mappings and applying spec / property adjustments.

Representative modules:

- `SkuMappingProcessor`
- `SpecDimensionUnifier`
- `SpecResolverService`
- `MixedAttributesProcessor`

Representative steps:

- `buildValidAsins(...)`
- `analyzeInvalidAndDuplicateAsins(...)`
- `collectFilteredMappings(...)`
- `finalizeFilteredMappings(...)`
- `countSkusNeedingDefaultSpecs(...)`
- `applyUnifiedSpecs(...)`
- `countTemporarySpecIDs(...)`
- `refreshResolvedSKUIdentifiers(...)`
- `collectDimensionCombinations(...)`
- `applyUnifiedDimensionToSKU(...)`

### 5. Build

Responsible for converting prepared mapping data into SKU / SKC results.

Representative steps:

- `buildSkcsFromPreparedAIMapping(...)`
- `resolveSkcBuildSpecs(...)`
- `buildMultipleSkcs(...)`
- `buildSingleSkc(...)`
- `buildDefaultSkcFromPreparedMapping(...)`
- `buildDefaultSkuFromAIMapping(...)`

### 6. Assign

Responsible for writing built results back to the task context / TEMU product.

Representative steps:

- `assignBuiltVariantSkcs(...)`

## Publish Result Chain

The publish-result flow is converging on a lighter structure:

1. build input
2. consume input in steps
3. execute side effects

Representative input object:

- `SavePublishResultInput`

Representative step functions:

- `logSubmitResponseWithInput(...)`
- `createProductImportMappingWithInput(...)`
- `recordDailyListingCountWithInput(...)`
- `pauseShopUntilEndOfDayWithInput(...)`

The design rule here is:

- input objects own field access, small rules, and metadata assembly
- steps own orchestration and side effects

## Object Roles

### Input objects

Input objects should own:

- field access
- task scope access
- small rules
- request assembly
- metadata assembly

Examples:

- `SavePublishResultInput`
- `AIBatchInput`

### Response objects

Response objects should own:

- reading
- controlled mutation
- aggregation
- construction

Example:

- `AISkuMappingResponse`

## Refactor Guidance

When continuing TEMU refactors, prefer these moves:

1. Move direct field reads from steps into input / response objects.
2. Keep entry functions thin and orchestration-focused.
3. Introduce stage-named helpers only when they clarify the main chain order.
4. Avoid large abstractions until the stage names and boundaries stay stable
   across multiple paths.

## Current Heuristic

If a new function fits one of these stage names:

- `resolve`
- `generate`
- `finalize`
- `prepare`
- `cleanup`
- `apply`
- `build`
- `assign`

then it likely belongs on the main TEMU pipeline and should be named and placed
accordingly.
