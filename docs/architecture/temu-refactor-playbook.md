# TEMU Refactor Playbook

## Goal

This note turns the current TEMU architecture patterns into practical editing
rules. It is meant to answer a narrower question than the other architecture
notes:

- not "what is the pipeline?"
- not "what patterns exist?"
- but "when making a change, where should this logic go?"

Use this document together with:

- `temu-pipeline-stages.md`
- `temu-architecture-patterns.md`

## Quick Decision Order

When touching TEMU code, prefer making placement decisions in this order:

1. can this live on an input object?
2. can this live on a response object?
3. does it belong to an adjacent module with a clear local role?
4. does the main chain need a stage-named helper?
5. if none of the above are true, keep it local

This order is intentional. It prevents the codebase from growing too many
top-level helpers before object boundaries and module boundaries are fully used.

## 1. Put Logic On Input Objects When It Answers "What Does This Input Know?"

Prefer input objects when the logic is fundamentally about:

- field access
- nested field access
- scope access
- count helpers
- iteration helpers
- request assembly
- metadata assembly
- small rules that depend only on the input

Good examples:

- `SavePublishResultInput.SubmitResponseLogFields(...)`
- `SavePublishResultInput.TenantAndStoreIDs()`
- `SavePublishResultInput.DailyLimitIncrement(...)`
- `SavePublishResultInput.ApplyImportMappingMetadata(...)`

Signals that logic belongs on an input object:

- the same nested field access appears in more than one step
- a step is reading three or more fields to compute one small rule
- the logic does not need orchestration context beyond the input itself

Do not put logic on an input object when:

- it needs to coordinate multiple modules
- it performs side effects
- it owns a visible pipeline phase

## 2. Put Logic On Response Objects When It Answers "How Is This Result Read Or Mutated?"

Prefer response objects when the logic is fundamentally about:

- reading result items
- iterating results
- controlled mutation
- aggregation
- construction

Good examples:

- `AISkuMappingResponse.SkuCount()`
- `AISkuMappingResponse.ForEachSKU(...)`
- `AISkuMappingResponse.ReplaceSKUs(...)`
- `AISkuMappingResponse.AppendResponse(...)`
- `NewAISkuMappingResponse(...)`

Signals that logic belongs on a response object:

- callers repeat direct slice access
- callers rebuild the same result container shape
- callers aggregate results in slightly different local ways

Do not put logic on a response object when:

- the logic is a workflow phase
- the logic depends on unrelated context objects
- the logic performs domain-specific side effects

## 3. Use Adjacent Modules For Local Transformations

If the logic is not just object access, ask whether it belongs to a nearby
module with a stable local role.

Current examples:

- `SkuMappingProcessor` for mapping cleanup
- `SpecResolverService` for temporary spec-id resolution
- `SpecDimensionUnifier` for dimension normalization
- `MixedAttributesProcessor` for property unification

Use adjacent modules when:

- the transformation is real domain work, not just orchestration
- it already clusters around one local concern
- the main chain benefits from keeping that detail out of the entry function

Avoid moving logic to adjacent modules if the result is only a pass-through
wrapper with no clearer ownership.

## 4. Introduce Stage-Named Helpers Only When They Clarify Flow Order

A new stage helper is worth introducing only when it makes the main chain easier
to read in order.

Good examples:

- `finalizeGeneratedAIMapping(...)`
- `prepareAIMappingForBuild(...)`
- `resolveAndBuildVariantSkcs(...)`
- `prepareAndBuildVariantSkcs(...)`
- `assignBuiltVariantSkcs(...)`

Signals that a stage-named helper is worth it:

- the entry function is mixing two or more phases
- the helper name makes the flow order more obvious
- the same phase idea now appears across multiple paths

Signals that a stage-named helper is not worth it:

- the name is vague (`handle`, `process`, `doWork`)
- the helper only saves a few lines but hides the order
- the helper mixes multiple unrelated phases

## 5. Prefer Consistent Stage Names Over New Synonyms

When possible, stay inside the current TEMU vocabulary:

- `resolve`
- `generate`
- `finalize`
- `prepare`
- `cleanup`
- `apply`
- `build`
- `assign`

Preferred examples:

- `resolveDefaultBuildProduct(...)`
- `prepareDefaultAIMappingForBuild(...)`
- `buildDefaultSkcFromPreparedMapping(...)`
- `assignBuiltVariantSkcs(...)`

Avoid inventing new phase names unless they express a genuinely new layer.

Avoid names like:

- `handleX`
- `processX`
- `doX`
- `manageX`

unless the helper is truly generic and not part of the pipeline vocabulary.

## 6. Keep Entry Functions Readable First

When editing entry functions, optimize for reading order before deduplication.

The preferred experience is that a reader can skim the function and understand
the chain without opening every helper.

Entry functions should usually:

1. validate or resolve
2. hand off to one phase helper
3. hand off to the next phase helper
4. assign or return

Good entry functions after recent refactors:

- `BuildVariantSkcs(...)`
- `CreateDefaultSkc(...)`
- `GenerateAISkuMapping(...)`

If a refactor shortens a function but makes the order harder to see, it is
usually a regression, even if the helper count increases.

## 7. Parallel Paths Should Tell The Same Story

When a default path, batch path, or single path exists, prefer asking:

"Can this path use the same stage language as the main path?"

Examples of healthy convergence:

- single and batch generation both expose `generate -> finalize`
- variant and default build paths both expose `resolve -> prepare -> build`
- publish steps increasingly rely on one input object

This is often more valuable than line-count reduction.

## 8. What Not To Extract

Do not extract a helper just because:

- a function is somewhat long
- there is a local `if` block
- two lines are repeated once

Extraction is usually not worth it when:

- the helper would only wrap one call
- the helper name adds no stage meaning
- the original function is still readable in order
- the logic has not stabilized across multiple paths

It is acceptable to leave some local detail inline if extracting it would make
the flow harder to read.

## 9. Review Questions Before Committing

Before finalizing a TEMU refactor, check:

1. Did this change strengthen object boundaries?
2. Did it make an entry function more orchestration-focused?
3. Did the new name clarify the main chain order?
4. Did it move a shared concern to the right owner?
5. Did it keep default / batch / main paths aligned?
6. Did it avoid introducing a vague abstraction?

If the answer to most of these is "yes", the refactor is probably reinforcing
the architecture instead of just rearranging code.

## Current Safe Bias

When uncertain, the safest TEMU refactor bias is:

- prefer promotion over duplication
- prefer stable stage names over new names
- prefer orchestration clarity over helper count reduction
- prefer path convergence over local cleverness

That bias has produced the most reliable gains in the current codebase.
