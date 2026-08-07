# Studio Generation Panel Controller Design

## Status

- Date: 2026-08-08
- Baseline: `master@b5861665`
- Scope: behavior-preserving frontend refactor in `web/listingkit-ui`

## Problem

`SheinStudioWorkbench` already delegates baseline warmup and batch-run context to
`shein-studio-generation-controller.ts`, but it still assembles the complete
`SheinStudioGenerationPanel` contract inline in JSX. That assembly mixes view
composition with generation-specific projection rules, including:

- choosing normal generation versus failed-batch retry;
- deriving button labels and retry notices;
- projecting failed, reused, and created task state;
- calculating the displayed batch product count and selection readiness;
- grouping the panel's action, form, and status models.

The panel itself is tested, but these Workbench-level projection decisions are
not owned by a focused, directly testable boundary. Adding another generation
state or action therefore continues to enlarge the Workbench.

## Goals

1. Make the Workbench consume one complete GenerationPanel props object.
2. Move generation-specific model projection into the existing generation
   controller module.
3. Preserve every current callback, label, notice, request, persistence action,
   and rendered result.
4. Add focused tests that fail if normal-generation and failed-retry projection
   drift.
5. Keep the change small enough for one independently reviewable PR.

## Non-goals

- Do not change reducer or state ownership.
- Do not introduce React Context, a feature store, or a state-machine library.
- Do not change API modules or split `shein-studio-batch-drafts.ts`.
- Do not change UI structure, styling, copy, or accessibility behavior.
- Do not change async-job, persistence, batch-run, task-creation, or retry
  sequencing.
- Do not move unrelated Workbench sections.

## Considered approaches

### 1. Pure projection in the existing controller — selected

Add a pure builder to `shein-studio-generation-controller.ts`. It accepts the
current state values and callbacks and returns the exact props consumed by
`SheinStudioGenerationPanel`.

Benefits:

- follows the existing controller direction;
- requires no new runtime state or effects;
- makes derived rules directly testable;
- produces the smallest behavioral risk and diff.

Trade-off: the Workbench still owns the underlying state and must supply the
builder inputs. This is intentional for this first slice.

### 2. Custom hook with `useMemo`

A hook could return the same props object, but it would require a large and
fragile dependency list without delivering meaningful performance or ownership
benefits. It would also make the projection harder to test as a plain value
transformation.

### 3. Context or independent generation store

This would reduce explicit props further, but it changes state ownership and
would mix a behavioral migration into a structural refactor. It is outside the
approved scope.

## Design

### Panel contract

`shein-studio-generation-panel.tsx` will export a named
`SheinStudioGenerationPanelProps` type composed of the existing public models:

```ts
export type SheinStudioGenerationPanelProps = {
  actions: SheinStudioGenerationActions;
  form: SheinStudioGenerationFormModel;
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  status: SheinStudioGenerationStatusModel;
};
```

The component signature will consume this type. The rendered component and its
internal behavior remain unchanged.

### Projection boundary

`shein-studio-generation-controller.ts` will add a named pure builder:

```ts
export function buildSheinStudioGenerationPanelProps(
  input: SheinStudioGenerationPanelProjectionInput,
): SheinStudioGenerationPanelProjectedProps;
```

The input will be grouped into three explicit sections:

- `actions`: current handlers and setters, plus the normal-generate and
  retry-failed-batch handlers needed to select `onGenerate`;
- `form`: current controlled form values;
- `status`: current task/batch state plus the raw values needed for derived
  labels, notices, failed items, product count, and readiness.

The builder will own only deterministic projection. It will not call APIs,
dispatch reducer actions, mutate refs, perform persistence, or create new
callbacks with hidden side effects.

`promptInputRef` is an imperative React handle, not form data. It remains a
top-level Panel prop and does not pass through the pure builder. Callback
wrappers that intentionally mutate Workbench-owned refs before calling a setter
remain explicit inputs. For example, the selected-SDS-image callback continues
to mark `hasCustomizedSdsSelectionRef` in the Workbench before updating the
selection.

### Workbench composition

The Workbench will build a typed raw projection input before the JSX return and
hand it to a thin React boundary:

```tsx
<SheinStudioGenerationPanelBoundary
  input={generationPanelInput}
  promptInputRef={promptInputRef}
/>
```

The boundary calls the pure builder and renders `SheinStudioGenerationPanel`.
This component boundary is required by React 19's ref rules: Workbench callbacks
that capture refs must cross JSX rather than an opaque render-time function
call. It has no state or effects. The Workbench no longer contains the three
large inline JSX object literals and retains:

- reducer and local state;
- refs and their mutations;
- API and persistence hooks;
- batch-generation and task-creation orchestration;
- navigation and section composition.

### Data flow

```text
Workbench state, refs, and existing callbacks
  -> SheinStudioGenerationPanelBoundary JSX props
  -> buildSheinStudioGenerationPanelProps
  -> { actions, form, status }
  -> SheinStudioGenerationPanel + promptInputRef
  -> existing callbacks back into Workbench-owned orchestration
```

No context provider, cache, or state synchronization step is introduced. The
single boundary render layer exists only to preserve React's ref semantics.

## Preserved projection rules

The builder must preserve these current rules exactly:

1. When retryable failed items exist, `onGenerate` invokes failed-batch retry;
   otherwise it invokes normal generation.
2. Failed-batch mode uses the `重试失败批次` label and the existing count-based
   retry notice.
3. Failed batch items come only from itemized batch entries whose item status is
   `failed`.
4. Failed, rejected, and reused task lists use the current itemized batch detail
   fallbacks.
5. The create-task label uses the existing selected-product count calculation.
6. `selectionReady` remains based on the active selection's `variantId`.
7. Current store, subscription, generation, task-creation, save, and retry
   messages remain unchanged.

## Error handling

The projection builder is synchronous and cannot produce a new operational
error. Existing errors and notices are passed through unchanged. API failures,
retry failures, persistence failures, and invalid selection behavior remain in
their current owners.

The builder will use the same optional chaining and empty-array fallbacks as the
current JSX so missing itemized batch detail continues to render safely.

## Testing strategy

Implementation follows red-green-refactor.

### Focused controller tests

Add tests to `shein-studio-generation-controller.test.tsx` that exercise the
real builder:

1. Normal mode selects the normal generation callback and preserves form/status
   values.
2. Failed-batch mode selects the retry callback, derives the retry label and
   notice, and exposes only failed items.
3. Task creation label and selection readiness match current behavior.

Each controller test is written first and observed failing because the builder
does not yet exist or does not yet implement the asserted projection. The React
boundary is covered through the existing Panel and Workbench behavior suites;
no source-text structural test is added.

### Regression tests

Run the focused controller, panel, and Workbench suites after the migration:

```powershell
npm test -- `
  src/components/listingkit/shein-studio/shein-studio-generation-controller.test.tsx `
  src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx `
  src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
```

Then run:

```powershell
npm run lint
npm run typecheck
npm test
npm run build
```

The backend baseline is not affected. The isolated worktree started from a clean
`go test ./... -count=1` result on `b5861665`.

## Acceptance criteria

- Workbench renders GenerationPanel through one React-safe boundary; that
  boundary obtains one controller-produced props object.
- The inline `actions`, `form`, and `status` literals are removed from the JSX.
- Focused tests cover normal generation and failed-batch retry projection.
- No product behavior, API call, persistence sequence, UI copy, or visual layout
  changes.
- Frontend lint, typecheck, tests, and build pass.
- The diff contains only the design/plan, GenerationPanel contract and boundary,
  generation controller, Workbench integration, and focused tests.

## Follow-up boundary

Splitting `shein-studio-batch-drafts.ts` remains a separate future PR. Deeper
generation state ownership changes require a new design and are not implied by
this controller projection slice.
