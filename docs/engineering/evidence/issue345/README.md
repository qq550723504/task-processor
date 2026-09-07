# Issue #345 browser acceptance evidence

The current Product title review screenshots and both `browser-*-evidence.json`
reports were captured from committed source and Go HEAD
`e364d74818cb6a13b03944c77d6005a81de1d5a6`. This combination includes #343
`3d59d669c5eda511c0724f66042bb54c2803be06` and #344
`c95b27679bd4cd2c1e4aed06da0290fd05586273` through normal merges.
Final delivery HEAD, CI and independent review are maintained in PR #351.

Both UI scripts ran sequentially against a fresh #344 fixture: 9 main-flow
acceptance groups and 6 lifecycle groups passed. The actual Go fixture test
passed without SKIP. Cleanup verified `portReleased=true`, `goExit=0` and
`containerStopped=true`. No session, cookie, manifest or raw trace is included.

| Artifact | Evidence |
| --- | --- |
| `pending-1440.png` | Existing Task Center entry, actual collection/detail and title evidence |
| `confirm-1440.png`, `confirm-390.png` | Separate native Apply confirmation at desktop and narrow widths |
| `pending-long-title-390.png` | Accepted proposal edited back to pending; long text without horizontal overflow |
| `receipt-1440.png` | Durable receipt and new Product version after explicit Apply |
| `unknown-receipt-1440.png` | Actual commit with lost response remains uncertain despite a later receipt read |
| `revoked-1440.png` | Actual LiveWrite denial clears the sensitive projection |
| `regression-diagnostic-after-apply-1440.png` | Existing diagnostic remains readable after title Apply |
| `browser-evidence.json` | Main flow, four axe checks with zero violations, durable versions and unchanged non-title/Listing observations |
| `browser-lifecycle-evidence.json` | Organization/user changes, double click, cancellation, unmount, late response and operator permissions |

These runs use the actual browser, Next BFF, Go handlers and isolated PostgreSQL.
The session/token issuer, grant provider and CandidateGenerator are controlled
external substitutes. Lifecycle fault injection delays delivery of the actual
BFF response after the real Go operation; it does not replace the response body.
The fixture's `restart` action reconstructs the application in the same Go
process. It does not prove OS process restart behavior. Production, real IAM and
paid model calls were not run.

`figma-task-center-1440.png` is the separately captured design reference.
Older `regression-completed-*` and diagnostic regression artifacts document the
earlier read-only regression and are not Product mutation acceptance evidence.
See [the implementation and reproduction notes](../../product-title-review-ui.md)
for ownership boundaries, approved design mapping and exact commands.
