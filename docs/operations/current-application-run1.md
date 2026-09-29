# RUN-1 current application loopback runbook

> **Scope note (2026-09-29)**: this runbook describes RUN-1, the identity/route acceptance
> environment only. The drift that broke it from 2026-09-20 is analysed in #541, and the
> split into RUN-1 plus a production-following FULL environment is designed in
> [`acceptance-environment-split.md`](acceptance-environment-split.md) - the split decision is
> **admitted (IMPLEMENTATION_READY)**; the FULL environment is **deferred by decision**, and
> section 3.1 there remains an **unadjudicated proposal** pending the **account-center and
> verification** domain owners. The **commercial domain was removed from that scope** on
> 2026-09-29: it has no startup permission check at all (section 3.1.1 of that document), and
> the question left on the commercial side is a different one, owned by `listingsubscription`.
> Do not widen RUN-1's schema or grant scope to follow production.

> **Current decision (2026-09-29, #541)**: fix the obsolete manifest field within
> #541. RUN-1 omits both `commercialDatabase` and `commercialOwnerDatabase`; it
> does not rename the field or upgrade a role. Keep the existing source-account,
> account-profile and verification ACL checks and commercial grants unchanged.
> A (account/verification composition) and B (commercial test-binding contract)
> remain separate owner questions, not prerequisites for this manifest repair.
> FULL, egress proxy configuration and real acquisition success are outside this
> startup acceptance.


This runbook owns an isolated, disposable acceptance environment for Issue #390.
It is not a deployment path and never targets shared or production resources.
The launcher allocates fresh loopback ports, private files, Docker resources and
an empty PostgreSQL database under one generated run ID.

## Lifecycle

Initialize the empty database explicitly and start the normal application plus
the current Console:

```powershell
node scripts/issue357-runtime.mjs start --current-application
```

The command prints the run ID and private manifest path. Subsequent commands
must use that exact run ID:

```powershell
node scripts/issue357-runtime.mjs check --run <run-id>
node scripts/issue357-runtime.mjs stop --run <run-id>
node scripts/issue357-runtime.mjs start --run <run-id>
node scripts/issue357-runtime.mjs restart --run <run-id>
node scripts/issue357-runtime.mjs destroy --run <run-id>
```

`stop` drains only the Go and Next processes. It retains the owned provider,
database, volumes and current facts. `start --run` and `restart` reuse those
facts and do not run schema initialization, bootstrap or seed again. `destroy`
is the separate destructive boundary: it verifies recorded ownership, stops
applications if needed, removes only resources carrying the run's exact ID and
label, deletes private credentials, and retains bounded evidence files.

Current-application restart requires an explicit successful stop report before
starting anything again. Failed or unconfirmed shutdown records `restart-failed`
at `stopping-applications`, exits nonzero and never prints `READY`. If cleanup
after a failed start cannot confirm successful shutdown, the retained state is
`stop-failed`, with both start and stop failures recorded; another start is denied.
These failures do not automatically destroy retained resources or facts.

A rejected current-application startup still exits nonzero and never publishes
READY. Its supervisor confirms child shutdown before writing the stop receipt.
If every spawned child exited without forced termination, the receipt records
`startupFailed: true` and successful cleanup; after the controller independently
checks process identities and released ports, the existing `start-failed` retry
path remains available. An unconfirmed or forced shutdown remains a failed stop.

## Runtime boundary

The serving process is built by `go build ./cmd/current-application`. It accepts
only an absolute private JSON manifest and listens on `127.0.0.1`. Its default
composition includes identity/effective-organization/account routes, SA1,
account profile and subject verification using the source-account pool, plus
the unified commercial overview. The authoritative route inventory is
`internal/app/httpapi/current_application.go`; optional owners are not enabled
by this launcher. It does not call the legacy default HTTP composition.

Schema installation runs separately before the first start from a run-private
working directory with an allowlisted environment; the launcher rejects every
`.env` location the existing schema tool could inspect. Serving opens only
the existing database through `sourceAccountDatabase`, using only
`source_account_runtime`. The harness retains these existing roles/grants:

- `source_account_runtime`: `CONNECT`, schema `USAGE`, resource
  `SELECT/INSERT/UPDATE`, and operation `SELECT/INSERT`, **plus** — in
  current-application mode — the account-center and verification tables that
  `sourceaccountregistry` verifies for the same pool: `SELECT/INSERT/UPDATE` on
  `account_business_profiles`, `personal_verification_applications`,
  `subject_verification_applications` and `subject_verification_messages`,
  `SELECT/INSERT` on `account_business_profile_audit_events`, plus
  `USAGE/SELECT` on `account_business_profile_audit_events_id_seq`. Those tables
  are installed by the SA1 schema initializer; it is the grant that was missing.
  A launch **without** `--current-application` does not receive them, because that
  composition is not verified against them.
- `commercial_runtime`: existing commercial SELECT and usage/audit INSERT/UPDATE
  grants are retained. RUN-1's normal application does not connect this role.
  `VerifyCommercialReadSchema` checks a wider runtime contract only through the
  test-binding helper `buildCommercialReadModuleFromDatabase`; it is not a
  preflight on the normal `buildUnifiedCommercialRead` composition. The role's
  read-only session setting does not replace ACL verification in that helper.
- `commercial_reader`: the legacy/default composition's read-only role, retained
  so that a launch without `--current-application` has the role it names.

The bootstrap token, database owner credentials and Login V2 service credentials
stay in run-private control files and are not present in the serving manifest.
The manifest rejects unknown fields (including the removed `commercialDatabase`)
and DSN-delimiter characters. `sourceAccountDatabase` must name
`source_account_runtime`. An explicitly supplied `commercialOwnerDatabase`
requires `commercial_owner_runtime` and enables separate owner capabilities;
RUN-1 supplies neither that block nor `productAcquisitionDatabase`. Startup
verifies official same-origin OIDC discovery and the source-account pool's exact
required/forbidden privileges and schemas before binding the listener. A provider
outage or under/over-privileged source-account role fails startup and leaves the
facts untouched. There is no commercial startup ACL preflight in this composition.

The permission preflight inventories every user table in the admitted `public`
schema for the source-account pool, including migration tables and additional
tables. It compares PostgreSQL
effective table and column privileges (including inherited and PUBLIC grants) with the exact
table/privilege grants above. Other tables may exist, but the source-account
runtime role may not have
unadmitted table or column privileges on them. For SELECT/INSERT/UPDATE/REFERENCES,
PostgreSQL's effective any-column check includes whole-table and column-only grants;
the required whole-table grants remain mandatory and cannot be replaced by column grants.
The server's privilege vocabulary is used,
including MAINTAIN on PostgreSQL 17; ordinary system catalog access is excluded
from this business-table inventory. Preflight only reads catalogs and schema: it
does not grant, revoke, alter or repair permissions.

The authenticated commercial overview returns `unified-base-prepaid-v1`. With
commercial and store owners absent, `resources` and `store_services` each have
`state: "unavailable"` and `value: null`. A successful authenticated overview
request proves route/authentication behavior; it does not prove balance, quota,
store facts or any commercial success path. The retained subscription fixture
tables are not used as a fallback. B's wider test-only runtime contract remains
unchanged pending its owner decision.

## End-to-end acceptance

After committing the candidate so source fingerprints are stable, run:

```powershell
node web/listingkit-ui/scripts/current-application-final-acceptance.mjs <40-char-candidate-sha>
```

This exercises official ZITADEL Login V2 and Auth.js authorization-code/PKCE
sessions, account reads and the authenticated commercial `unavailable` response,
the actual SA2 TypeScript client and
Workbench BFF, SA1 writes/reads/isolation/role denial/idempotency, application
stop with retained resources, and two fact-preserving starts. The runner always
attempts the separate owned-resource destroy step, deletes the temporary Auth.js
cookie handoff, and never logs credentials or tokens.

The source-account required/forbidden permission checks remain exercised. The
runner no longer expects grants on the unconnected `commercial_runtime` role to
block startup; this removes an obsolete RUN-1 assertion, not the commercial
permission matrix or its dedicated safety tests. Implementer checks and CI are
development evidence; independent validation/user trial is still required.
