# Hot-reference Prompt Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make hot-reference generation and per-design regeneration reuse the persisted artwork description without changing theme-prompt generation.

**Architecture:** Keep `theme_prompt` and `hot_reference` as mutually exclusive reference modes. Update the shared frontend generation-input builder so hot-reference mode selects the saved `hotStyleReferencePrompt`, falls back to `hotStyleReferenceBrief`, appends optional supplemental artwork constraints, and keeps exactly one hot-reference URL. Apply the same composition to the backend batch request path. The existing regeneration handler already calls this builder and replaces only the selected design, so no new API is needed.

**Tech Stack:** Next.js/TypeScript, Vitest, existing Studio generation API, Go backend validation unchanged.

## Global Constraints

- `theme_prompt` sends no reference images.
- `hot_reference` sends exactly one hot-reference image when available.
- Hot-reference text comes from the saved extracted artwork prompt, then the saved brief, followed by optional supplemental artwork constraints.
- Do not inject product types such as shirt, mug, or poster into the artwork prompt; preserve only the user's explicit supplemental artwork constraints.
- Regeneration remains one-image generation and preserves the target design metadata.

---

### Task 1: Add failing prompt-mode regression tests

**Files:**
- Modify: `web/listingkit-ui/src/lib/shein-studio/generation-controller.test.ts:282-335`
- Test: `web/listingkit-ui/src/lib/shein-studio/generation-controller.test.ts`

**Interfaces:**
- Consumes: `buildHotStyleReferenceGenerationInput(input)`.
- Produces: executable expectations for the shared mode-specific input builder.

- [ ] **Step 1: Replace the hot-reference expectation that currently uses `prompt`**

Add a case where `prompt` contains supplemental artwork constraints and `hotStyleReferencePrompt` contains the extracted artwork description. Assert that the returned prompt contains both in order and the returned references contain only the first hot-reference URL:

```ts
expect(buildHotStyleReferenceGenerationInput({
  artworkGenerationMode: "hot_reference",
  prompt: "Use the red and cream palette with no text",
  hotStyleReferenceBrief: "bold retro eagle badge",
  hotStyleReferencePrompt: "Original eagle badge artwork, red and cream palette",
  hotStyleReferenceImageUrls: [
    " https://example.com/hot-ref.png ",
    "https://example.com/other.png",
  ],
})).toEqual({
  prompt: "Original eagle badge artwork, red and cream palette\nAdditional artwork constraints: Use the red and cream palette with no text",
  productReferenceImageUrls: ["https://example.com/hot-ref.png"],
});
```

- [ ] **Step 2: Add a brief fallback case**

Assert that a blank saved extracted prompt uses the saved brief and still preserves the supplemental artwork constraints.

- [ ] **Step 3: Keep the theme-mode assertion explicit**

Assert that theme mode still returns the ordinary prompt and an empty `productReferenceImageUrls` array even when hot-reference fields are populated.

- [ ] **Step 4: Run the focused tests and verify RED**

Run:

```powershell
pnpm --dir web/listingkit-ui test -- src/lib/shein-studio/generation-controller.test.ts
```

Expected: the new hot-reference assertions fail because the builder currently returns the ordinary prompt.

### Task 2: Implement the shared mode-specific prompt builder

**Files:**
- Modify: `web/listingkit-ui/src/lib/shein-studio/generation-controller.ts:250-275`
- Test: `web/listingkit-ui/src/lib/shein-studio/generation-controller.test.ts`

**Interfaces:**
- Consumes: existing `BuildHotStyleReferenceGenerationInput` fields `artworkGenerationMode`, `prompt`, `hotStyleReferenceBrief`, `hotStyleReferencePrompt`, and `hotStyleReferenceImageUrls`.
- Produces: `{ prompt: string; productReferenceImageUrls: string[] }` with mode isolation.

- [ ] **Step 1: Implement the minimal hot-reference selection**

In the `hot_reference` branch, select the first non-empty trimmed value from `hotStyleReferencePrompt` and `hotStyleReferenceBrief`, then append `input.prompt` as optional supplemental artwork constraints when present. Continue normalizing and limiting reference URLs to one.

```ts
const artworkPrompt = [
  input.hotStyleReferencePrompt?.trim() || input.hotStyleReferenceBrief?.trim() || "",
  input.prompt.trim() ? `Additional artwork constraints: ${input.prompt.trim()}` : "",
].filter(Boolean).join("\n");
return {
  prompt: artworkPrompt,
  productReferenceImageUrls: normalizedHotStyleReferences.slice(0, 1),
};
```

- [ ] **Step 2: Run the focused tests and verify GREEN**

Run the same Vitest command from Task 1. Expected: all generation-controller tests pass.

- [ ] **Step 3: Run the related draft-input tests**

Run:

```powershell
pnpm --dir web/listingkit-ui test -- src/lib/shein-studio/draft-input.test.ts src/lib/shein-studio/batch-hydration.test.ts
```

Expected: existing mode-exclusivity and batch hydration tests pass, confirming saved hot-reference fields and supplemental artwork constraints remain available.

### Task 3: Verify regeneration uses the corrected builder and preserves replacement behavior

**Files:**
- Inspect: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts:352-495`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.test.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/generation-controller.test.ts:254-275`

**Interfaces:**
- Consumes: the corrected `buildHotStyleReferenceGenerationInput` result and the backend batch prompt composer.
- Produces: one-image regeneration request and stable design replacement.

- [ ] **Step 1: Reuse the existing replacement-semantics regression test**

Keep the existing `replaceRegeneratedDesign` test at `generation-controller.test.ts:254-275` as the guard that the original design ID, `targetGroupKey`, and `targetGroupLabel` survive replacement. Do not change replacement behavior while changing prompt selection.

- [ ] **Step 2: Run the focused workbench tests**

Run:

```powershell
pnpm --dir web/listingkit-ui test -- src/components/listingkit/shein-studio/shein-studio-workbench-actions.test.ts src/lib/shein-studio/generation-controller.test.ts
```

Expected: all focused tests pass, confirming regeneration still calls the shared builder and replaces only the selected design.

### Task 4: Run full verification and inspect the final diff

**Files:**
- Modify: none beyond the implementation and tests above.

**Interfaces:**
- Consumes: all completed changes from Tasks 1-3.
- Produces: verified branch ready for review.

- [ ] **Step 1: Run the complete frontend test suite**

Run:

```powershell
pnpm --dir web/listingkit-ui test
```

Expected: exit code 0 with no failed tests.

- [ ] **Step 2: Run frontend typecheck and lint**

Run:

```powershell
pnpm --dir web/listingkit-ui typecheck
pnpm --dir web/listingkit-ui lint
```

Expected: both commands exit 0.

- [ ] **Step 3: Run the backend suite because the mode contract is shared with Go validation**

Run:

```powershell
go test ./... -count=1
```

Expected: exit code 0.

- [ ] **Step 4: Review diff hygiene**

Run:

```powershell
git diff --check
git status --short
git diff --stat
```

Expected: no whitespace errors, only scoped files changed, and no unrelated worktree edits.

- [ ] **Step 5: Commit the implementation**

```powershell
git add -- web/listingkit-ui/src/lib/shein-studio/generation-controller.ts web/listingkit-ui/src/lib/shein-studio/generation-controller.test.ts web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.test.ts
git commit -m "fix: isolate hot reference regeneration prompts"
```
