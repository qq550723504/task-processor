# ListingKit production hardening split plan

This document records the recommended split for the current ListingKit hardening
work. It is intended for follow-up PRs or for rewriting the local branch into
smaller commits before sharing.

## 1. ZITADEL auth and tenant identity

Scope:
- `web/listingkit-ui/src/app/api/zitadel-auth/**`
- `web/listingkit-ui/src/components/providers/zitadel-auth-gate.tsx`
- removal of `yudao-auth` UI bridge files
- ZITADEL Kubernetes examples under `deployments/kubernetes/zitadel/**`
- tenant identity forwarding in ListingKit proxy auth

Validation:
- `npm run typecheck`
- `npm test -- src/app/api/listing-kits/route.test.ts src/app/listing-kits/listingkit-smoke.test.tsx`

## 2. ListingKit proxy and local storage boundaries

Scope:
- `web/listingkit-ui/src/app/api/listing-kits/proxy-*.ts`
- `web/listingkit-ui/src/lib/server/local-storage-path.ts`
- `web/listingkit-ui/src/lib/server/local-json-file.ts`
- local storage usage in async jobs, SHEIN Studio storage, style gallery

Validation:
- `npm test -- src/app/api/listing-kits/proxy-*.test.ts src/lib/server/*local*.test.ts`
- `npm run lint`

## 3. API response schemas

Scope:
- `web/listingkit-ui/src/lib/api/*-schema.ts`
- `web/listingkit-ui/src/lib/api/response-schema.ts`
- parser wiring in task list, queue, preview, review, dispatch, SDS product APIs

Validation:
- `npm test -- src/lib/api`
- `npm run typecheck`

## 4. Studio async jobs

Scope:
- Go async job API under `internal/listingkit/api/studio_async_jobs_handler.go`
- route registration in `internal/app/httpapi/server.go`
- UI `apiAsyncRequest` backend-first flow and Next fallback
- database-backed Studio async job persistence wiring

Validation:
- `go test ./internal/listingkit/api -run StudioAsyncJob -count=1`
- `go test ./internal/app/httpapi -run ListingKitEndpoints -count=1`
- `npm test -- src/lib/api/client.test.ts src/lib/api/async-job-resume.test.ts`

## 5. SHEIN Studio reducer and smoke coverage

Scope:
- `shein-studio-workbench-state.ts`
- `shein-studio-workbench.tsx`
- `shein-studio-workbench-actions.ts`
- `shein-studio-workbench-workspace.ts`
- `web/listingkit-ui/src/app/listing-kits/listingkit-smoke.test.tsx`

Validation:
- `npm test -- src/components/listingkit/shein-studio/shein-studio-workbench*.test.ts*`
- `npm test -- src/app/listing-kits/listingkit-smoke.test.tsx`
- `npm test`

## Current recommendation

Keep the already-created local commit as a checkpoint unless the branch needs
code review. Before opening a PR, rewrite or cherry-pick into the five groups
above so reviewers can inspect auth, proxy/schema, async job, and Workbench
changes independently.
