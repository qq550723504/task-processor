# TEMU Architecture Patterns

## Goal

This note captures the higher-level structural patterns that now recur across
the TEMU code paths. The purpose is to make future refactors cheaper: instead
of rediscovering local conventions file by file, we can align new work with the
patterns that have already proven stable.

This document complements `temu-pipeline-stages.md`:

- `temu-pipeline-stages.md` explains the stage language of the SKU pipeline
- this note explains the object roles, entry roles, and refactor moves that now
  define the broader TEMU style

## Core Pattern

The TEMU codebase is converging on a repeated arrangement:

1. objects own boundaries
2. entry functions own orchestration
3. adjacent steps own local transformations
4. stage names describe flow order

In practice, that means:

- input objects own field access, scope access, request assembly, and small
  rules
- response objects own reading, controlled mutation, aggregation, and
  construction
- processor / step / builder entry points stay thin and express the order of
  the flow
- stage-named helpers are introduced when they clarify the chain instead of
  hiding it

## Pattern 1: Objects Own Boundaries

### Input objects

Input objects are no longer plain field bags. They increasingly own the
reusable read-side and rule-side concerns needed by steps.

Current examples:

- `SavePublishResultInput`
- `AIBatchInput`

Typical responsibilities:

- field access
- task scope access
- request assembly
- metadata assembly
- small rule calculation
- iteration and count helpers

Representative methods on `SavePublishResultInput`:

- `HasProduct()`
- `SKCCount()`
- `SKUCount()`
- `ForEachSKU(...)`
- `TaskLogFields()`
- `SubmitResponseLogFields(...)`
- `TenantAndStoreIDs()`
- `BuildImportMappingCreateReq(...)`
- `ApplyImportMappingMetadata(...)`
- `DailyLimitConfig()`
- `DailyLimitIncrement(...)`
- `DailyLimitExceededReason(...)`

### Response objects

Response objects are increasingly treated as actual result objects rather than
shared mutable structs.

Current example:

- `AISkuMappingResponse`

Typical responsibilities:

- read helpers
- controlled mutation
- aggregation
- construction

Representative methods on `AISkuMappingResponse`:

- `SkuCount()`
- `FirstSKU()`
- `SKUAt(...)`
- `ForEachSKU(...)`
- `ForEachSKUIndexed(...)`
- `ReplaceSKUs(...)`
- `AppendSKU(...)`
- `AppendSKUs(...)`
- `AppendResponse(...)`
- `NewAISkuMappingResponse(...)`
- `NewEmptyAISkuMappingResponse()`

### Design rule

If a step or processor repeatedly reads the same nested fields or repeats the
same small rule, prefer moving that concern into the input / response object
before introducing a new top-level helper.

## Pattern 2: Entry Functions Own Orchestration

Entry functions should increasingly read like orchestration, not like mixed
orchestration + implementation.

Current SKU-side examples:

- `GenerateAISkuMapping(...)`
- `BuildVariantSkcs(...)`
- `BuildSkcsFromAIMapping(...)`
- `CreateDefaultSkc(...)`

Current publish-side examples:

- `logSubmitResponseWithInput(...)`
- `createProductImportMappingWithInput(...)`
- `recordDailyListingCountWithInput(...)`
- `pauseShopUntilEndOfDayWithInput(...)`

The preferred shape is:

1. validate / resolve what the step needs
2. hand work to a clearly named phase helper
3. write back results or trigger side effects

This is why names like these are valuable:

- `resolveAndBuildVariantSkcs(...)`
- `assignBuiltVariantSkcs(...)`
- `resolveAndBuildDefaultSkc(...)`
- `buildDefaultSkcFromResolvedProduct(...)`

They preserve the order of the main flow instead of hiding it.

## Pattern 3: Stage Names Are Flow Vocabulary

The SKU pipeline now repeatedly uses the same phase words:

1. `resolve`
2. `generate`
3. `finalize`
4. `prepare`
5. `cleanup`
6. `apply`
7. `build`
8. `assign`

These names matter because they let us place code by role rather than by local
convenience.

### Practical interpretation

- `resolve`: obtain required source data or scope
- `generate`: produce AI mapping data
- `finalize`: normalize generated result objects after generation
- `prepare`: make data build-ready
- `cleanup`: correct mismatches or invalid mappings
- `apply`: push spec / property adjustments into the data
- `build`: convert prepared data into SKU / SKC outputs
- `assign`: write built outputs back to context / product state

### Guidance

If a new helper naturally fits one of these names, prefer using that vocabulary
instead of inventing a new synonym. Consistent stage naming is part of the
architecture, not just a naming preference.

## Pattern 4: Parallel Paths Should Converge On The Same Story

The most important structural gain in the recent refactors is not just cleaner
functions. It is that different paths are starting to tell the same story.

Examples:

- single AI mapping path and batch AI mapping path both now expose
  `generate -> finalize`
- variant build path and default build path both now expose
  `resolve / prepare / build`
- publish steps increasingly rely on one input object instead of each step
  re-reading context in its own way

This is the preferred direction:

- default paths should not feel structurally special
- batch paths should not drift into separate conventions
- main paths should not carry hidden rules that never got promoted into shared
  helpers or objects

## Pattern 5: Adjacent Chains Own Local Transformations

The main pipeline does not need every step collapsed into a single file. A
better pattern has emerged:

- the main entry expresses the order
- adjacent modules own local transformations

Examples:

- `SkuMappingProcessor` owns mapping cleanup
- `SpecResolverService` owns spec-id resolution
- `SpecDimensionUnifier` owns spec-dimension normalization
- `MixedAttributesProcessor` owns property unification

The pipeline stays readable as long as each adjacent module has a clear role in
that chain.

## Pattern 6: Prefer Promotion Over Duplication

When a concern appears in multiple steps, prefer promoting it upward into one of
these buckets:

1. input object capability
2. response object capability
3. stage-named helper on the owning processor

Prefer this order because it keeps duplicated read logic and duplicated local
rules out of steps first, and only then introduces new flow helpers if needed.

## What To Avoid

Avoid these moves unless there is a strong reason:

- introducing a helper whose name does not clarify stage order
- hiding three or four distinct stages behind one vague abstraction
- adding new direct field reads in steps when an input / response object already
  owns related access
- letting default or batch paths invent their own stage names
- creating broad shared abstractions before the stage language is stable across
  multiple paths

## Refactor Checklist

When editing TEMU code, these questions are a useful filter:

1. Does this logic belong on an input object or response object instead of in a
   step?
2. Is this entry function still acting like an orchestrator?
3. Does this helper name clarify the main chain order?
4. Is this path drifting away from the same stage language used elsewhere?
5. Is the change strengthening a stable pattern, or only cleaning one local
   function?

## Current Working Model

At the moment, the safest model for continuing TEMU refactors is:

- objects own boundaries
- entries own orchestration
- adjacent modules own local transformations
- stage names describe the flow
- default and batch paths should converge on the same story

As long as new changes reinforce those five rules, the TEMU code should keep
getting easier to extend without needing a large framework rewrite.
