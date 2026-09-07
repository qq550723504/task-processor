# #350 Console plans and entitlements

## Design authority and admission

The user supplied these current final nodes on 2026-09-07:

- [套餐方案 / 431:3455](https://www.figma.com/proto/tg48P46SSXl6TBy9lZwg63/硕米智能引擎?node-id=431-3455&page-id=31%3A463)
- [我的权益 / 431:3959](https://www.figma.com/proto/tg48P46SSXl6TBy9lZwg63/硕米智能引擎?node-id=431-3959&page-id=31%3A463)

Both were read using get_design_context with skillNames=figma-design-to-code, then separate 1440×900 screenshots. Reference PNGs are in evidence/issue350. Code Connect was enabled; no mapped component snippets were returned. Existing ConsolePage, ConsoleState, Card, Button, Next Link and Console semantic tokens are reused. Shell/navigation graphics remain owned by the existing Console; the three resource bullet SVGs are exact downloaded Figma assets, with explicit dimensions. No temporary asset URLs enter production source.

Independent narrow admission by entitlements_admission: IMPLEMENTATION_READY, no BLOCKER. This is a presentation slice within the existing subject/organization/role boundary, not a new authorization or commercial state machine. Two pages share the existing #347 contract. Final code and browser review remain separate gates.

## Frame → component → contract

| Frame region | Local component responsibility | #347 fields and limits |
| --- | --- | --- |
| Both frame headers and breadcrumbs | CommercialPage, ConsolePage | Current context organization name/id; observed_at from authorized response; no response means observation unavailable |
| Options 1844:368 overview | PlanOptions | plans.name/code/source/availability; approved descriptions, never actual subscription or sellable price; price/currency null explicitly unavailable |
| Options 1844:375,382,389 three cards | ResourceCards | Store pricing, AI point and data acquisition are not provided; do not copy design price or conversion rules |
| Options 1844:396 rules | PlanOptions explanation | Descriptions do not grant subscription/entitlements; no commercial formula; actual subscription appears on entitlements page |
| Options 1844:407 related actions | PlanOptions links and unavailable text | Only link to implemented entitlements page; wallet, billing and usage detail remain unavailable |
| Entitlements 1844:496 overview | EntitlementsOverview resource summary | resource_balance unsupported; store service count/expiry not supplied by this reader; never substitute store_count limit or operation usage |
| Entitlements 1844:512,522,532 cards | ResourceCards | Preserve three-column structure and semantic accents; resource quantities unsupported; AI points are not model tokens or cash |
| Entitlements 1844:542 capabilities | GrantedEntitlements | entitlements actual rows, effective_status, validity, explicit_grant_only limits, uninterpreted_limit_count; no invented AI/Chat capability labels |
| Entitlements 1844:556 boundary | EntitlementsOverview explanation | Member allocation, resource acquisition and cash deferred; no links suggesting they are operational |
| Additional subscription section | SubscriptionDetails | subscription null means no subscription; preserve custom plan name/code; status and effective_status separate; null validity is open bound |
| Additional usage section | UsageObservations | Exact decimal committed/reserved, backend unit and known/unknown; UTC calendar month start/end exclusive for operations; current retained-byte gauge has no period; separate updated_at |
| Error/loading/context state | ScopedCommercialRequest + ConsoleState | Only CommercialReadError safe code mapping; no old success data while reading, denied, revoked, switching or context unavailable |

## Approved semantic differences

- Finding: Figma says binding starts service time. Product requirement: binding does not charge; explicit activation starts the period. Classification: NOT_APPLICABLE. Action: use approved wording, no store action in these pages.
- Finding: Figma shows ¥168, balances and charging conversion rules. Requirement: only approved source facts. Classification: NOT_APPLICABLE. Action: no design values/prices/conversions; prices/currency explicitly not provided, resources unsupported.
- Finding: Figma uses token积分 and hypothetical enabled capability chips. Requirement: approved AI点数 terminology and actual grants. Classification: NOT_APPLICABLE. Action: normalize terminology; display persisted module labels/status, no inferred feature availability.
- Missing resource/cash customer reader is approved scope, not a request for a new ledger/API. Unknown, unsupported, unlimited and finite zero remain distinct.
- Actual subscription/grants/usage and scope/observation labels require additional space below the original composition. 390px is an engineering responsive adaptation; the final Figma reference is desktop. Body text is 13px and supporting text at least 12px per frozen Console accessibility decisions.

## Invariants and evidence plan

The sole wire module is `src/lib/api/commercial.ts` from #347. Pages call getCommercialOverview with the expected organization and AbortSignal. No schema/client/BFF, persistence or business arithmetic is duplicated. Parent scope checks and keyed children discard previous subject/org/role state; query data has zero retention, no persisted cache, no placeholder data or automatic retry. Refresh hides old data before new authorization; cancelled old requests cannot surface their normalized 504 in a new scope. Server authorization remains authoritative.

TDD covers both pages, no subscription, effective statuses, explicit grants, zero/unlimited/unknown/unsupported, precise int64/signed byte delta, UTC month versus gauge, unsafe text, safe typed errors, scope changes, late success/error, refresh and unmount. Browser tests must use the actual page→#347 client/BFF→Go→isolated PostgreSQL chain. Provider session/token/grants are explicit substitutes; OpenMeter is not queried. Real IAM, payment, customer data and production are NOT_RUN.

Shared workspace-app-shell.tsx belongs to #348; console-navigation.ts requires serial owner handoff before enabling the two existing plans entries. No provider lifecycle, shell architecture or other navigation entries are changed by this slice.

Rolling implementation HEAD, CI, exact-SHA browser/screenshots, independent final review, fixture startup/stop and remaining gates belong in the PR. Design/reference images alone are not implementation evidence.

## Reproduce isolated browser acceptance

1. Follow the unique [#347 contract runbook](issue347-commercial-read-contract.md). From this checkout run `node web/listingkit-ui/scripts/commercial-read-fixture.mjs --serve --web-dir <absolute-checkout>/web/listingkit-ui`. It creates an isolated PostgreSQL container and an actual Next/Auth.js/BFF/Go chain, using only synthetic identity issuance and grants. Wait for its smoke PASS and ready message. Do not print or copy its private manifest.
2. From `web/listingkit-ui` run `node scripts/commercial-ui-acceptance.mjs <private-fixture.json-path> <absolute-evidence-directory>`. The existing Playwright Chromium and axe dependencies run the two real pages at 1440×900 and 390×844, dark/light themes, keyboard interaction, actual organization switching, status/error/lifecycle cases and logout. No route mocking or commercial response replacement occurs. The long-name probe changes rendered text only and is explicitly separate from real contract acceptance. Finite store zero and trialing are covered by component tests; the backend fixture does not seed those cases.
3. The sanitized report records SHA/source hashes, screenshots, checks and request method/path/status only. It never records cookies, auth headers, manifest values or response payloads. Normal restores only fixture provider behavior; recovery uses actual organization selection, because revoked access clears the effective-org cookie and ordinary context grants have the existing conservative cache. No cache or authorization changes are introduced here.
4. Create an empty `stop-fixture` file in the launcher-reported control directory. Wait for launcher exit 0. Verify `cleanup.json` has `passed: true` and `goExit: 0`, and `go.log` records `ZERO_WRITE` with equal all-saas-table row-value/xmin snapshots. The owner launcher checks Next process/port closure and removes only its unique fixture container, without a persistent volume. Do not terminate unrelated processes or clean worktrees.

The in-app Browser rejected the local fixture port; the repository's existing Playwright launched a separate disposable test browser. Screenshot evidence is therefore actual Chromium acceptance, not a claim about an in-app Browser session. The visible Next development indicator is part of the test runtime. Real OIDC re-login after logout is NOT_RUN: the fixture issuer deliberately has no OIDC server, so the redirect reaches an Auth.js configuration error. Actual logout/session removal, BFF 401 and absence of prior commercial UI are independently checked.

The shared navigation window was authorized for this slice's two entries, committed separately and handed back to #348. The latest main consumed for combination verification is recorded in the PR. Unmerged #345/#348 page and navigation combinations remain a separate merge gate until their owners deliver them; current-tree Store/context/navigation/provider tests do not stand in for that future combination.
