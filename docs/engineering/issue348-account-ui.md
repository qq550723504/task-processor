# Account UI: scope, design mapping and isolated verification

Issue #348 delivers the two read-only pages in the existing Console Shell. The page consumes `web/listingkit-ui/src/lib/api/account.ts`, owned by #346. Profile, enterprise grants and commercial facts remain separate owners. No identity mutation, member management, resource allocation or financial projection is implemented.

## Design mapping

Both exact nodes in Figma file `tg48P46SSXl6TBy9lZwg63` were read with `get_design_context` and their screenshots visually inspected before implementation. Source screenshots are in `evidence/issue348/`.

| Authority | Current component / data | Deliberate data boundary |
| --- | --- | --- |
| Profile `432:534`, light 1440 × 900; nameplate `453:425` | `ProfileView`, displayName, userId, homeOrganizationId | No example username, certification or last-login fact |
| Account settings `453:440` | Real display name / phone / email; nullable values remain unknown | Region and registration date: 未提供 |
| Business profile `453:465` | 经营画像 panel and original field groups | Business roles, shops, factory, platforms, sites, shop type and services: 未提供; project grants are not business roles |
| Account status `453:494` | Phone/email verification: true 已验证, false 未验证, null 未提供 | Password state not sourced and never inferred |
| Enterprise `432:4483`, dark 1440 × 900; card `1631:472` | `OrganizationView`, name, Effective ID, Home ID, current account project roles | Certification, administrator and totals not provided |
| Management `1631:490`, `1631:497`, `1631:504` | Three static cards | No fake management CTA |
| Resources `1634:359`, allocation `1636:371` | Explicit unavailable regions | No fabricated zero, balance, member rows or derived commercial data |

`source` and `readAt` are displayed in an expandable provenance section. Enterprise authorization is CachedRead with a maximum 60-second cache; projection read time is not grant refresh time. The exported enterprise avatar background at `public/console/account/organization-avatar.svg` preserves the original Figma asset bytes; it is decorative, not a company logo. No expiring asset URL is used at runtime.

Current Console semantic variables and shared Card/Page/State/Button are reused. Profile dark theme, enterprise light theme and both 390px layouts are engineering adaptations, not additional approved Figma frames. The real Shell retains enterprise selection and Home/Effective delegation information, so vertical placement differs from the example Shell. The layout uses normal document flow and expands for real text.

## Identity and lifecycle

`AccountServerPage` uses the existing server session, identity-version, session-error and access-token helpers and passes only `expectedUserId` to the client. The legacy allowlist session endpoint is not used. The actual profile reader still authorizes the request server-side; a Shell exception is not an authorization grant.

Only exact `/workbench/account/profile` bypasses enterprise loading/selection/grant errors. Both Shell authentication error sources remain blocking. Organization, descendants, plans, tasks and Store keep enterprise gates. When context is uncertain, old user/org/delegation metadata is hidden in the same Shell.

Personal and enterprise reads use separate React Query keys. User/org/role/context transitions remount the request subtree, discard its cache and abort its consumed signal. Switching and refetching hide old data; unmounted late success and failure cannot populate the replacement query. Logout removes the request subtree immediately. There is no persistent profile cache, optimistic fact, automatic write or retry loop.

## Repeatable verification

From `web/listingkit-ui`, after the #346 dependency is integrated:

```powershell
pnpm.cmd install --frozen-lockfile
pnpm.cmd exec vitest run src/components/workbench/account src/components/workbench/workspace-app-shell.test.tsx src/lib/workbench/console-navigation.test.ts
pnpm.cmd lint
pnpm.cmd typecheck
pnpm.cmd test
pnpm.cmd build
node scripts/account-fixture.mjs --serve
```

The fixture prints a private temporary `fixture.json` path and localhost origin. It issues synthetic Auth.js sessions and substitutes only the external OIDC/UserInfo/Authorization services; the ordinary browser scenarios use the actual client → Next BFF → Go current identity/grant/readers. The fixture uses no database or real IAM credentials.

In a second terminal, supply the printed private path:

```powershell
node scripts/account-browser-verification.mjs C:/path/printed/by/fixture/fixture.json
```

The browser script uses isolated Chromium contexts. It records eight desktop/mobile/theme screenshots with axe, overflow and keyboard checks, including Tab/Shift+Tab order for provenance/refresh and the account menu; no-org/selection/grant outage profile reads; provider and identity errors; actual enterprise switching, failed switching, late cancellation, logout and cache-expired revocation; the unauthenticated proxy redirect and sibling enterprise gates. Two separately named supplemental scenarios use synthetic responses: long-name layout overrides its account response, and a role-context change overrides its context response while exercising the actual provider and profile client. These are not evidence of real provider mutations. Screenshot-only styling hides the Next development-tools portal so it cannot cover product text; it does not change application CSS or hide product elements.

To inspect manually, open the printed origin in an isolated browser context and load a chosen synthetic cookie set from the private manifest locally. Do not paste cookies into Issue/PR, commit the manifest, export authenticated traces, or use a normal personal browser profile. The command and output screenshots are safe to share; the manifest and process logs stay temporary.

Stop normally by writing `stop-fixture` in the manifest's `controlDirectory`:

```powershell
Set-Content -LiteralPath C:/path/printed/by/fixture/stop-fixture -Value stop
```

The launcher shuts down Next, signals the Go fixture, checks Go PASS and verifies its Next port can be rebound. `cleanup.json` remains in the private control directory. Browser contexts close in `finally` even on test failure.

The browser report binds results to the checked-out source HEAD, fixture HEAD and normalized source-tree SHA256. Its source hash covers sorted unique tracked/untracked nonignored `src` and `public/console/account` paths, each path plus NUL plus UTF-8 LF-normalized contents. Evidence-only commits do not change that digest. Rolling final HEAD, exact dependency revision, CI, independent review and any remaining acceptance restrictions belong in the PR.

Real login issuance, real IAM, production deployment and real identity/business/financial operations are NOT_RUN and not authorized by this slice.
