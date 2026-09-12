# ACC-3: resources and optional SourceAccount management

Execution: #411. Parent: #408. Product authority is
`docs/product/final-ui-ia-authority.md` and the current non-archived Figma
`tg48P46SSXl6TBy9lZwg63`, node `432:4483` (design context and screenshot read).

The page belongs under **我的账户 → 企业空间 → 资源与额度**. SourceAccount
management is a content child, not an additional sidebar tier. Figma example
balances, store counts and member allocations are not backend facts.

## Current capability boundaries

| Projection | Authority and transport | v1 behavior |
| --- | --- | --- |
| Granted entitlements and subscription usage | Existing `api/commercial.ts` → commercial BFF → RUN-1 `CommercialReadService` / `GormRepository` | Reuse `EntitlementsOverview`; preserve exact integers, units, explicit grant scope, unknown usage and effective status. Read only. |
| Resource balances | `ledger/orgresource` owns platform resource facts; no resource balance read route in the current RUN-1 route contract | AI points, data allowance and cash remain unavailable. Subscription usage is never converted to a balance. |
| Store | `storecenter` exists, but Store routes are absent from the RUN-1 contract | Actual count, service state and connection are unavailable. `store_count` is a grant limit, not a count of bound stores. |
| SourceAccount | Existing unique SA2 `api/source-accounts.ts` → strict BFF → `sourceaccountregistry` + its PostgreSQL repository | List, register, detail, enable and disable. Optional Organization resource. |

Commercial reads use the existing `listingkit.admin.read` permission. SourceAccount
uses `workbench.source_account.read/manage`. The current viewer may read source
accounts but not commercial facts or perform management actions; operator/admin
affordances follow the existing role matrix. The backend remains authorization
authority. Expected actor and Organization headers are drift checks, never trusted
identity or grants.

`enabled` means internal resource management only. `pending_connection` is shown
as an unverified connection projection, not a registration error. Neither state
proves acquisition readiness. Anonymous public acquisition does not require a
SourceAccount. No Connection, 1688 login, provider mutation or acquisition UI is
introduced.

## Request and mutation behavior

- Reads use the existing strict clients and server bounds. Query keys include
  actor, Effective Organization and roles. Refetching hides prior facts; context
  changes unmount their scoped request subtree and consume cancellation signals.
- Registration validates the existing schema. A user submission creates one
  operation identity. Lifecycle operations carry the detail response's exact
  strong ETag. No optimistic status or version is calculated.
- Confirmed mutation results trigger a fresh list/detail read. A replay response
  is not allowed to replace later current facts locally.
- One submitted intent is retained in ordinary component memory while its result
  remains unresolved. Other scopes hide its content and cannot replay it. A
  restored original actor/Organization can explicitly retry the same method,
  account, body, key and ETag if management is allowed.
- A rejected/not-sent later attempt does not resolve the original uncertainty.
  The retry button says **使用原请求重试** and explains that, without a prior
  receipt, the retry may complete the original write. List/detail refreshes are
  read only. There is no automatic mutation retry, browser ledger, local storage
  persistence or additional verification endpoint.
- Full page navigation/reload does not persist that memory and is not evidence
  that a write failed. The UI asks users to read current facts before deciding on
  a subsequent action. Access rejection hides old facts and management controls.

## Ownership and legacy

This slice owns resources/SourceAccount routes, components and their tests only.
AccountShell/navigation and shared architecture maps belong to #409. Global
catch-all/proxy/current application, CI, guards and lock files are not modified.

The paused #397 branch was inspected at `aff6b8def46cf7c2d989408d115fa3c9628e585e`;
it had no committed UI delta. Its recorded `120b` worktree was absent. Historical
dirty changes and test receipts were not treated as available code or current
verification. #411 implements against actual main; no extraction of those
missing files is claimed and no old workspace is restored or cleaned.

Legacy decision: RETIRE obsolete Task-first/quota/required-login assumptions.
Reusable behavior: existing current Console primitives, Commercial projection,
and SA2 clients. Current owners remain those listed above. No wrapper, migration,
fallback, second balance or account source is introduced.

## Verification

Focused commands (from `web/listingkit-ui`):

```text
pnpm exec vitest run src/components/workbench/resources/resources-page.test.tsx src/components/workbench/source-accounts/source-accounts-page.test.tsx
pnpm typecheck
pnpm lint
pnpm build
```

The permanent tests cover exact/unknown values, failed reads, organization and
actor changes, role loss, stale responses, server pagination, exact ETags,
response loss, rejected/not-sent retries and restoration of the original scope.

For isolated current-application verification, reuse
`node scripts/issue357-runtime.mjs start --current-application` from a clean fixed
candidate. Record its new run ID, SHA, source path, ports and owned resources.
Use only that run's local Login V2 users, PostgreSQL and browser tabs. Stop and
destroy only by that run ID using the existing supervisor commands. Exact
candidate receipts, screenshots, negative cases, cleanup, CI and independent
complete-slice review belong in the PR; local fixture results are not shared IAM,
real 1688, production or merge acceptance.
