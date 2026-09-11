# RUN-1 current application loopback runbook

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

## Runtime boundary

The serving process is built by `go build ./cmd/current-application`. It accepts
only an absolute private JSON manifest, listens on `127.0.0.1`, and assembles the
four current identity/effective-organization/account routes, the commercial
overview read, and the five admitted SA1 routes. It does not call the legacy
default HTTP composition.

Schema installation runs separately before the first start from a run-private
working directory with an allowlisted environment; the launcher rejects every
`.env` location the existing schema tool could inspect. Serving opens only
the existing database through two roles:

- `source_account_runtime`: `CONNECT`, schema `USAGE`, resource
  `SELECT/INSERT/UPDATE`, and operation `SELECT/INSERT`.
- `commercial_reader`: read-only session and `SELECT` only on the four tables
  used by the commercial overview.

The bootstrap token, database owner credentials and Login V2 service credentials
stay in run-private control files and are not present in the serving manifest.
The manifest rejects DSN-delimiter characters and any role name other than the
two admitted roles. Startup verifies official same-origin OIDC discovery, exact
required/forbidden database privileges and current schemas before binding the
listener. A provider outage or an under/over-privileged role therefore fails
startup and leaves the facts untouched.

The permission preflight inventories every user table in the admitted `public`
schema, including migration tables and additional tables. It compares PostgreSQL
effective table and column privileges (including inherited and PUBLIC grants) with the exact
table/privilege grants above. Other tables may exist, but neither role may have
unadmitted table or column privileges on them. For SELECT/INSERT/UPDATE/REFERENCES,
PostgreSQL's effective any-column check includes whole-table and column-only grants;
the required whole-table grants remain mandatory and cannot be replaced by column grants.
The server's privilege vocabulary is used,
including MAINTAIN on PostgreSQL 17; ordinary system catalog access is excluded
from this business-table inventory. Preflight only reads catalogs and schema: it
does not grant, revoke, alter or repair permissions.

Commercial overview queries explicitly address these same four `public` tables,
regardless of a connection's `search_path` or same-named tables in another schema.
Missing public tables or SELECT privileges fail closed; shadow facts are never a fallback.

## End-to-end acceptance

After committing the candidate so source fingerprints are stable, run:

```powershell
node web/listingkit-ui/scripts/current-application-final-acceptance.mjs <40-char-candidate-sha>
```

This exercises official ZITADEL Login V2 and Auth.js authorization-code/PKCE
sessions, account and commercial reads, the actual SA2 TypeScript client and
Workbench BFF, SA1 writes/reads/isolation/role denial/idempotency, application
stop with retained resources, and two fact-preserving starts. The runner always
attempts the separate owned-resource destroy step, deletes the temporary Auth.js
cookie handoff, and never logs credentials or tokens.
