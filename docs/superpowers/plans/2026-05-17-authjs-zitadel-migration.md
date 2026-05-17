# Auth.js ZITADEL Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the custom ListingKit ZITADEL OIDC/session plumbing with Auth.js while preserving the existing ListingKit identity and authorization contract.

**Architecture:** Introduce a single Auth.js authority for OIDC state, PKCE, callback handling, cookie/session lifecycle, and refresh token rotation. Keep ListingKit-specific tenant/user/role extraction, allowlist authorization, and upstream header mapping in a thin adapter layer so the Go backend contract does not change.

**Tech Stack:** Next.js App Router, Auth.js (`next-auth@beta`), ZITADEL OIDC, React Query, Vitest.

---

## File Map

- Create: `D:\code\task-processor\web\listingkit-ui\src\auth.ts`
- Create: `D:\code\task-processor\web\listingkit-ui\src\auth.config.ts`
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\api\auth\[...nextauth]\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\package.json`
- Modify: `D:\code\task-processor\web\listingkit-ui\package-lock.json`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\proxy.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\lib\server\zitadel-auth.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\login\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\callback\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\logout\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\session\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\listing-kits\proxy-auth.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\providers\zitadel-auth-gate.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\lib\server\zitadel-auth.test.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\listing-kits\route.test.ts`

### Task 1: Add Auth.js Core

**Files:**
- Create: `D:\code\task-processor\web\listingkit-ui\src\auth.config.ts`
- Create: `D:\code\task-processor\web\listingkit-ui\src\auth.ts`
- Create: `D:\code\task-processor\web\listingkit-ui\src\app\api\auth\[...nextauth]\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\package.json`
- Modify: `D:\code\task-processor\web\listingkit-ui\package-lock.json`

- [ ] Install Auth.js beta compatible with App Router and Next 16.
- [ ] Define a ZITADEL OIDC provider config using issuer/client/scopes from env.
- [ ] Configure JWT session strategy and store `access_token`, `refresh_token`, `expires_at`, `id_token` in the token.
- [ ] Implement refresh-token rotation in the Auth.js `jwt` callback.
- [ ] Map session output to ListingKit’s stable identity shape: `tenantId`, `userId`, `username`, `roles`, `userType`.
- [ ] Expose Auth.js handlers under `/api/auth/[...nextauth]`.

### Task 2: Keep ListingKit Contract, Remove Custom Transaction Logic

**Files:**
- Modify: `D:\code\task-processor\web\listingkit-ui\src\lib\server\zitadel-auth.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\login\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\callback\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\logout\route.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\zitadel-auth\session\route.ts`

- [ ] Delete custom OIDC transaction state/callback exchange logic from `zitadel-auth.ts`.
- [ ] Keep only ListingKit-specific helpers: env parsing, identity authorization, session-to-identity adaptation.
- [ ] Make `/api/zitadel-auth/login` redirect into Auth.js sign-in with preserved `returnTo`.
- [ ] Make `/api/zitadel-auth/callback` a compatibility redirect to the Auth.js callback endpoint or remove its external dependency if no callers remain.
- [ ] Make `/api/zitadel-auth/logout` delegate to Auth.js sign-out while preserving public post-logout redirect.
- [ ] Keep `/api/zitadel-auth/session` response shape stable for the existing frontend and proxy code.

### Task 3: Rewire Middleware and Proxy Auth

**Files:**
- Modify: `D:\code\task-processor\web\listingkit-ui\src\proxy.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\listing-kits\proxy-auth.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\components\providers\zitadel-auth-gate.tsx`

- [ ] Switch middleware/session reads from custom cookies to Auth.js `auth()`-backed session checks.
- [ ] Preserve current behavior: unauthenticated -> login redirect, unauthorized -> `/unauthorized`.
- [ ] Keep API proxy auth extracting bearer token / session identity without changing backend headers.
- [ ] Remove assumptions about `listingkit_zitadel_session` cookie presence from middleware and proxy helpers.
- [ ] Keep the client auth gate behavior stable, but source session state from the Auth.js-backed session route.

### Task 4: Verify and Deploy

**Files:**
- Modify: `D:\code\task-processor\web\listingkit-ui\src\lib\server\zitadel-auth.test.ts`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\api\listing-kits\route.test.ts`

- [ ] Update unit tests for the new session/authorization adapter behavior.
- [ ] Add at least one test covering Auth.js-backed session identity extraction.
- [ ] Run `npm run typecheck`.
- [ ] Run targeted Vitest suites for auth/session/proxy.
- [ ] Build the UI image, deploy it, and production-test repeated visits to `/listing-kits/new`, `/listing-kits/style-gallery`, `/listing-kits/sds`, and `/listing-kits/admin/import-tasks`.
