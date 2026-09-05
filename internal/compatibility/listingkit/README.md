# ListingKit Legacy Drain

`internal/compatibility/listingkit` contains remaining historical ListingKit bridges that have not yet completed cutover. It is a **retirement zone**, not a target layer or extension point.

Repository policy is `EXTRACT | RETIRE`; there is no long-term Compatibility class.

## Current active debt

The 1688 handoff under `sourcehandoff/a1688` is still wired into production and currently bridges source input into legacy ListingKit task/request objects. GitHub #30 owns its `EXTRACT -> RETIRE` cutover:

- preserve valid SourceEnvelope / source identity behavior;
- preserve verified request identity and source/store access semantics;
- preserve required publication/idempotency behavior;
- move those behaviors to their current Product / Store / Organization / Application owners;
- switch the production path;
- remove the old ListingKit handoff route, wiring, and legacy-only tests.

Other code in this tree is handled by #29/#300. For example, an adapter with no production caller should be deleted rather than retained as a precautionary compatibility surface.

## Rules

Do not add here:

- new service entrypoints;
- new DTO bridges for new features;
- new marketplace or product rules;
- new crawler/sourcing ownership;
- new Agent/Tool/BusinessTask dependencies;
- fallback to root ListingKit;
- permanent dual-path behavior.

When useful behavior is found here, extract it directly into the current owner and make the new caller depend on that owner. Do not wrap this package to keep it alive.

See `docs/refactoring/legacy-hard-cut-policy.md`, `docs/refactoring/legacy-register.md`, #29, #30, and #300.
