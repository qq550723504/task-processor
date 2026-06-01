# ListingKit SDS Responsive Design Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the SDS frontend responsive across mobile, tablet, and desktop for homepage, recent batch dashboard, batch workbench shell, batch detail shell, and batch run progress.

**Architecture:** Keep the existing SDS component structure and business logic, but refactor layout classes and a few component boundaries to be mobile-first. First normalize shared shells and summary regions, then adapt page-specific cards, controls, and lists so they progressively expand from one column to larger breakpoint layouts.

**Tech Stack:** Next.js App Router, React, Tailwind CSS, Vitest, Chrome/in-app browser checks

---

## File Structure

- Modify: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.tsx`
  - homepage hero, featured summaries, run-progress container
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
  - filter blocks, bulk-action blocks, batch card grid, action layout
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-page-shell.tsx`
  - shared SDS header shell, metric layout, quick actions
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-page-shell.tsx`
  - batch shell spacing and header behavior
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-run-progress.tsx`
  - progress metrics, item row stacking, action/header layout
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.tsx`
  - detail summary stacking and section responsiveness
- Optional Modify: supporting SDS / Shein Studio components that contain blocking width rules discovered during implementation
- Modify: existing related frontend tests where responsive-driven rendering changes affect snapshots or button text expectations

## Task 1: Normalize Shared SDS Shell Layout

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-page-shell.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-page-shell.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-page-shell.test.tsx`

- [ ] Inspect header, metric, and quick-action sections for desktop-only width assumptions.
- [ ] Write or update a focused test for any conditional text/action rendering touched while refactoring shell layout.
- [ ] Refactor shell containers to be mobile-first:
  - single-column hero on narrow screens
  - metric grid that starts stacked and expands at `sm/md`
  - wrapped or stacked quick actions on small screens
- [ ] Remove or reduce blocking width rules such as large `min-w` values in top-level shell regions.
- [ ] Run: `npm test -- src/components/listingkit/shein-studio/shein-studio-page-shell.test.tsx`
- [ ] Commit: `git commit -m "feat: make SDS shell responsive"`

## Task 2: Make SDS Homepage Responsive

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/sds/sds-homepage-entry.test.tsx`

- [ ] Add or update tests covering homepage action visibility and batch-run progress entry points if layout refactors touch those branches.
- [ ] Refactor homepage hero to a mobile-first single-column layout with stacked actions on narrow widths.
- [ ] Convert featured summary cards to a progressive grid that grows from one to three columns.
- [ ] Normalize loading, error, and empty-state shells to use the same responsive spacing.
- [ ] Verify long summary titles and status labels do not force card overflow.
- [ ] Run: `npm test -- src/components/listingkit/sds/sds-homepage-entry.test.tsx`
- [ ] Commit: `git commit -m "feat: make SDS homepage responsive"`

## Task 3: Make Recent Batches Dashboard Responsive

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.tsx`
- Test: existing dashboard tests if affected, especially `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`

- [ ] Inspect dashboard regions in source and identify filter bars, bulk-action bars, summary cards, and minimum-width rules that will block phone widths.
- [ ] Add or update at least one focused test if action labels, grouping text, or conditional controls change.
- [ ] Refactor filter and bulk-action sections to stack vertically on small screens and align horizontally only at larger breakpoints.
- [ ] Convert batch cards and risk/detail blocks to progressive grids and vertically ordered action areas on narrow screens.
- [ ] Fix overflow-prone text and width rules (`min-w`, nowrap-like behavior, cramped action clusters).
- [ ] Run: `npm test -- src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`
- [ ] Commit: `git commit -m "feat: make SDS recent batches dashboard responsive"`

## Task 4: Make Batch Run Progress Responsive

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-run-progress.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-run-progress.test.tsx`

- [ ] Update tests for cancel-state and progress rendering if layout-driven wording or grouping changes require it.
- [ ] Refactor header/actions so mobile uses a vertical stack and desktop keeps inline controls.
- [ ] Convert metric summary cards into a mobile-safe grid and ensure IDs can wrap.
- [ ] Restack batch item rows for narrow widths so status and errors remain readable.
- [ ] Run: `npm test -- src/components/listingkit/shein-studio/shein-studio-batch-run-progress.test.tsx`
- [ ] Commit: `git commit -m "feat: make SDS batch run progress responsive"`

## Task 5: Make Batch Detail Responsive

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`

- [ ] Inspect dual-column detail summaries and top-level sections for mobile overflow risks.
- [ ] Update tests if visible labels or section ordering assumptions change.
- [ ] Refactor batch detail summary areas to stack on small screens and expand progressively on larger widths.
- [ ] Ensure route links, product metadata, and printable-area details wrap safely.
- [ ] Run: `npm test -- src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx`
- [ ] Commit: `git commit -m "feat: make SDS batch detail responsive"`

## Task 6: Sweep Supporting Components for Blocking Width Rules

**Files:**
- Modify: supporting files discovered while implementing, likely among:
  - `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-busy-overlay.tsx`
  - `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-selection-overview.tsx`
  - `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-grouped-selection-panel.tsx`
  - `web/listingkit-ui/src/components/listingkit/sds/sds-console-metrics.tsx`

- [ ] Search for remaining SDS-related `min-w`, hard multi-column grids, or overflow-prone wrappers after the main page refactors land.
- [ ] Apply the smallest focused responsive fixes needed to align support components with the new shell behavior.
- [ ] Run targeted tests for any touched support component.
- [ ] Commit: `git commit -m "fix: align SDS support components with responsive shell"`

## Task 7: Verification

**Files:**
- No new source files required unless a verification finding forces a small fix

- [ ] Run the SDS-related frontend test set:
  - `npm test -- src/components/listingkit/sds/sds-homepage-entry.test.tsx src/components/listingkit/shein-studio/shein-studio-page-shell.test.tsx src/components/listingkit/shein-studio/shein-studio-batch-run-progress.test.tsx src/components/listingkit/shein-studio/shein-studio-batch-detail.test.tsx src/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard.test.tsx`
- [ ] Run: `npm run typecheck`
- [ ] Launch the app and inspect responsive behavior at roughly `390px`, `768px`, and `1280px` widths.
- [ ] Validate:
  - no page-level horizontal overflow in SDS homepage, batch detail, workbench shell, or run progress
  - homepage actions remain reachable on phone width
  - recent batches dashboard filters and bulk actions remain operable on phone width
  - run IDs / batch IDs wrap safely
  - workbench shell reads as a vertical workflow on mobile
- [ ] Fix any concrete verification issue found before declaring completion.
- [ ] Commit any final verification fix with a focused message.

## Spec Coverage Check

- SDS homepage responsiveness: covered by Task 2 and Task 7.
- Recent batches dashboard responsiveness: covered by Task 3 and Task 7.
- Batch workbench shell responsiveness: covered by Task 1 and Task 7.
- Batch run progress responsiveness: covered by Task 4 and Task 7.
- Batch detail / shared shell responsiveness: covered by Task 5 and Task 7.
- Support-component width cleanup: covered by Task 6.

## Placeholder Scan

- No `TODO`, `TBD`, or deferred placeholder tasks remain.
- Every task names concrete files and verification commands.

## Type and Naming Check

- All targeted files exist in the current SDS frontend structure.
- The plan keeps existing component names and limits scope to layout and rendering structure rather than domain logic changes.
