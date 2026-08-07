# Canonical Product Actions Design

**Date:** 2026-08-07
**Status:** Approved for implementation

## Goal

Allow a user viewing a standard product to continue to the original task status page or the correct platform workspace without losing task context.

## Context

The canonical-product detail page is currently read-only. It shows the normalized product and persisted source lineage, but the only navigation is back to the canonical-product list. The task already has stable routes for status and workspace, and the workspace route selection logic already handles platform ordering and resumable SHEIN workflows.

## Design

### Read-model routing data

Extend the frontend `CanonicalProductDetail` read model with a required `workspaceHref: string`. Build it in `buildCanonicalProductDetail` by reusing `buildTaskWorkspaceHref` with the task ID, result platforms, and SHEIN workflow status already present in `ListingKitTaskResult`:

```ts
workspaceHref: buildTaskWorkspaceHref({
  task_id: summary.taskId,
  platforms: result.result?.platforms,
  shein_workflow_status: result.shein_workflow_status,
}),
```

The existing helper remains the single source of truth for platform routing. The status link is deterministic from `taskId` and stays a page concern: `/listing-kits/${taskId}/status`.

### Detail-page actions

Add a compact action group to the canonical-product detail header:

- `查看原任务` links to the task status page;
- `进入工作台` links to `detail.workspaceHref` and is the primary action;
- both are ordinary same-origin `Link` elements rendered through the existing `Button asChild` pattern.

The actions are available whenever canonical-product detail exists. A missing platform still produces the base workspace path through the existing helper.

### Compatibility and safety

- No backend endpoint or database change.
- No new platform-selection rules.
- Legacy task results without platforms use `/listing-kits/:taskId/workspace`.
- The source-reference card remains read-only and unchanged.
- The status page remains the canonical place for task lifecycle state; the detail page only links to it.

## Alternatives considered

1. **Recommended: reuse `buildTaskWorkspaceHref` in the canonical detail mapper.** This preserves the existing platform-selection behavior and keeps the page declarative.
2. Build the workspace URL directly in the page. This duplicates routing rules and can drift from task-list/home behavior.
3. Add a new backend action endpoint. This would add unnecessary API surface because navigation only needs existing task identity and platform metadata.

## Testing

- Mapper tests verify SHEIN routing, fallback to the first platform, and the base workspace path when no platform exists.
- Page tests verify both action labels and exact status/workspace hrefs.
- Existing `buildTaskWorkspaceHref` tests remain the routing regression suite.
- Full UI tests, typecheck, and lint are required before handoff.

## Scope exclusions

- No new workspace behavior or platform adaptation logic.
- No task mutation, submission, or retry action from the canonical detail page.
- No changes to canonical product persistence or task APIs.
