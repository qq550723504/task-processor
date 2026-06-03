# ListingKit SDS Studio Itemized Cutover Closure

Date: 2026-06-04
Status: Practically complete

## Summary

The SDS Studio itemized cutover has reached a practical closure point.

The remaining work is no longer about missing architecture ownership. It is
mostly UI-local composition and compatibility state that lives inside the
workbench page itself.

## What Landed

The main ownership corrections are now in place:

1. Legacy session-centered write entry removal
   - `ReplaceStudioSessionDesignsRequest` removed
   - `ReplaceStudioSessionDesigns(...)` removed
   - itemized batch flow remains the only public write path

2. Detail-first workbench projection
   - `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts`
   - itemized batch detail is the primary source for selection, grouped
     selections, approved designs, and most batch semantics

3. Detail-first saved-batch compatibility adapter
   - `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
   - itemized detail is now the primary source when rebuilding persisted
     saved-batch compatibility fields

4. Review task creation contract narrowing
   - `web/listingkit-ui/src/lib/shein-studio/create-review-tasks.ts`
   - grouped and non-grouped task creation now converge on
     `approvedDesigns`, while legacy `designs + selectedIds` input remains
     accepted as a compatibility boundary

5. Dedicated batch fallback projection isolation
   - `projectWorkbenchStateToSavedBatch(...)` now owns the page-local
     fallback projection from workbench state to `SheinStudioSavedBatch`

## Why This Is A Closure Point

The remaining flat fields still present in frontend types and state, such as
`designs`, `selectedIds`, `createdTasks`, and `generationJobs`, are not a
single missing cutover seam anymore.

They now serve one of these roles:

- UI-local workbench state
- persisted saved-batch compatibility shape
- view-model fields for review/task flows

That means further extraction would mostly optimize for symmetry rather than
eliminate a real ownership hotspot.

## Remaining Residuals

The following areas still carry compatibility-era fields, but they no longer
look like separate architecture bugs:

- `web/listingkit-ui/src/lib/types/shein-studio.ts`
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`

These should be treated as normal product/UI cleanup, not as a continuation of
the itemized cutover architecture line.

## Recommended Next Step

Do not continue this line with more cutover phases by default.

If future work touches SDS Studio again, evaluate it as one of:

- product behavior changes
- UI state cleanup
- persistence model cleanup

Only open a new architecture line if a new multi-consumer ownership hotspot is
found.
