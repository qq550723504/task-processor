# Task 3 report — Studio manual background removal review UI

Date: 2026-08-08
Worktree: `C:\Users\Henry\code\task-processor\.worktrees\studio-manual-background-removal`

## Scope followed

- Implemented only Task 3 in the frontend review UI.
- Did not modify backend behavior.
- Did not modify `design-image.ts`; only consumed `resolveGeneratedDesignOriginalSrc` and `resolveGeneratedDesignFinalSrc`.
- Kept approval, regeneration, lightbox opening, create-task controls, and read-only behavior intact aside from the required two-image review presentation and updated labels.

## TDD record

### RED

Added failing tests in:

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx`
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx`

Focused failures were confirmed with:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx
npm.cmd test -- src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
```

Observed expected failures:

- preview grid still rendered a single preview card;
- `重新抠图` was not available except in failed state;
- read-only mode did not show separate original/final previews;
- lightbox still used the old `原图` / `查看生成原图` copy.

### GREEN

Implemented the smallest UI change in:

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx`
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-lightbox.tsx`

Focused passing verification:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
```

Result: `2 passed, 9 passed`.

## Implementation summary

- Preview grid now always shows `重新抠图` for non-read-only cards when the callback is available.
- Removal-mode cards now render two labeled panes:
  - `原图` from `resolveGeneratedDesignOriginalSrc`
  - `抠图后` from `resolveGeneratedDesignFinalSrc`
- When no final removed image exists, the right pane shows the required exact copy:
  - `尚未抠图`
  - `抠图处理中`
  - `抠图失败，当前显示原图`
- Failed state keeps the original image visible and surfaces `backgroundRemovalError` when present.
- Lightbox labels now use:
  - `抠图后`
  - `查看原图`
  - `查看抠图后`

## Self-review

- Confirmed the change is scoped to Task 3 files plus this report.
- Confirmed no backend files or shared source helpers were edited.
- Confirmed focused tests cover:
  - succeeded removal with original/final URLs and retry action;
  - not-requested status with retry action and placeholder state;
  - read-only two-image visibility without action button;
  - lightbox original/final label copy and normalized original URL.

## Concerns

- Focused component tests pass, but I did not run broader Studio suites in this task because the brief explicitly asked for focused test coverage.

## Task 3 fix round 1

Review finding: when `resolveGeneratedDesignFinalSrc` returned an empty string for a pending, failed, or not-requested removal, the lightbox fell back to raw `imageUrl`/`dataUrl` values and could render a legacy `/api/v1/...` upload path.

### TDD RED

Added a pending-state lightbox regression test using:

```text
/api/v1/listing-kits/uploads/files/pending-image.png
```

Command:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
```

Result: `1 failed | 1 passed`; the rendered image still had the legacy `/api/v1/listing-kits/uploads/files/pending-image.png` source, confirming the expected failure.

### TDD GREEN

Changed the lightbox fallback to `resolveGeneratedDesignSrc(design)`, preserving the succeeded final-image preference while normalizing the current image in every removal state.

Command:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
```

Result: `2 passed`; `10 passed` tests.

No backend files or `design-image.ts` changes were made.

## Final whole-branch review fix wave

### Findings addressed

- Ordinary `none` / `not_requested` cards now render the same original and removed panes as removal-mode cards, including the neutral `尚未抠图` state.
- Pending and failed cards keep their exact `抠图处理中` and `抠图失败，当前显示原图` states; failed removal errors remain visible without hiding the original pane.
- The lightbox design-view label is `抠图后` only when `resolveGeneratedDesignFinalSrc` returns a confirmed final URL; ordinary, pending, and failed current images are labeled `原图` while their normalized current source remains intact.
- Regeneration and background-removal retry are mutually disabled for the same design ID, while each action keeps its independent loading label.
- `重新抠图` is now gated by `readOnly` and the optional callback independently of `canRegenerate`; `canRegenerate` only controls `重新生成`.
- `not_requested` and succeeded-without-final-result states use neutral status styling and `尚未抠图`, never the success styling.

### TDD RED

Added or updated focused regression tests in:

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx`
- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx`

Command:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
```

Result: expected RED — `5 failed | 9 passed` across the two files. The failures covered the missing ordinary dual panes/action, missing cross-operation disabling, and static lightbox `抠图后` labels for ordinary, pending, and failed states.

### TDD GREEN and typecheck

Implemented the minimal UI changes only in the preview grid and lightbox. The focused suite then passed:

```powershell
npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
```

Result: `2 passed`; `14 passed` tests.

Typecheck:

```powershell
npm.cmd run typecheck
```

Result: exit code `0`; `tsc --noEmit` completed successfully.

No backend files or `design-image.ts` changes were made in this fix wave.
