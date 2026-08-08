Status: complete

Summary:
- Added a regression test proving `runItemizedBackgroundRemovalRetry` forwards an ordinary design ID (`transparentBackgroundMode: "none"`, `backgroundRemovalStatus: "not_requested"`) without client-side filtering.
- Verified the workbench callback is a direct pass-through to `runItemizedBackgroundRemovalRetry` and does not add its own filtering.
- No production code change was needed.

Test:
- `npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-task-creation-controller.test.ts`
- Result: 41 passed, 0 failed

Commit:
- recorded in git history

Concerns:
- None.

Fix log:
- `npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-task-creation-controller.test.ts`
  - `Test Files  1 passed (1)`
  - `Tests  41 passed (41)`
- `npm.cmd run typecheck`
  - `tsc --noEmit`
  - exited cleanly with no TypeScript errors
