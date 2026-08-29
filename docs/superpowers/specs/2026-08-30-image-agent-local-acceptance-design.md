# Image Agent Local Acceptance Identity and Seed Design

## Status

- Date: 2026-08-30
- Status: proposed for document review; implementation has not started
- Scope: a self-contained local acceptance environment for the ListingKit manual Image Agent path
- Decision: authenticate a real local ZITADEL user, derive the task tenant and owner from that user's token, and seed only a guarded local acceptance database

## Goal

Make the existing browser-facing manual Image Agent flow verifiable without asking an operator to discover or supply a tenant ID or user ID. The acceptance path must exercise real token discovery and introspection, existing ListingKit ownership checks, task-backed source asset selection, image-agent run creation, and Temporal dispatch.

The environment must be disposable, local-only, repeatable, and unable to silently write a forwarded, shared, or production database. It must not add an authentication bypass, a mock identity header, a fake provider result, or a source-image fallback.

## Context and root causes

The Image Agent workspace endpoint already verifies the caller identity and requires the ListingKit task's `TenantID` and resolved `UserID` to match the verified identity. An empty environment therefore cannot be accepted by inserting arbitrary IDs into a task; that would exercise neither ownership nor the authentication boundary.

The local ZITADEL Compose definition is not currently self-contained: its local PostgreSQL service is profile-gated while the documented start command does not enable the profile, and ZITADEL always requires an external `docker_yudao-network`. A seed script on top of that configuration would hide, rather than solve, the environment-boundary defect.

The standalone ListingKit migration path must also be able to prepare a blank dedicated database through the real schema owner. It is not acceptable for seed tooling to create legacy table shells or issue ad-hoc SQL merely to make a task repository work.

Finally, Image Agent source assets are validated as public HTTPS URLs. A `localhost`, Docker-network, private-IP, or opaque local object URL is intentionally rejected before provider execution. A real acceptance seed therefore needs a caller-provided public HTTPS source image.

## Decision and non-goals

This slice introduces a local acceptance control plane, not a new product runtime.

It will:

1. make local ZITADEL runnable without Yudao or another pre-existing Docker network;
2. extend the repository's existing `internal/zitadelprovision` ownership boundary to create/reuse the required project, roles, API application, browser OIDC application, and local operator authorization;
3. reuse the existing ZITADEL verification semantics to derive canonical identity from a bearer token;
4. create one owned ListingKit task with a minimal SHEIN asset bundle through the repository API;
5. run the existing API, Image Agent Temporal worker, and manual workspace routes against the local configuration;
6. prove a real user can reach the owned preflight and create a run.

It will not:

- seed an AI credential, provider result, COS object, or generated image;
- change the generic Image Agent HTTP API or add a browser-accessible seed endpoint;
- accept tenant, user, run, plan, or budget identifiers from a seed caller;
- make a local MinIO instance appear to be real COS acceptance;
- weaken safe-image URL validation;
- alter production compose files, remote databases, or remote ZITADEL tenants.

## Architecture

```text
one-time local management token (entered locally, never committed)
                         |
                         v
local acceptance bootstrap
  - ListingKit project and roles
  - API introspection application
  - browser OIDC application
  - grant current human user listingkit_operator
                         |
                         v
real local browser login ----> bearer token file (untracked)
                         |
                         v
shared ZITADEL identity verifier
                         |
              +----------+-----------+
              |                      |
              v                      v
     API route middleware      local seed command
     identity + roles          derives resource-owner + subject
              |                      |
              +----------+-----------+
                         v
guarded image_agent_acceptance database
                         |
                         v
owned ListingKit task -> manual preflight -> run creation -> Temporal worker
```

### Local ZITADEL Compose

`deployments/docker/zitadel/docker-compose.yml` becomes the self-contained local default: its PostgreSQL dependency is enabled by default and all default services attach only to the `local-zitadel` network. The documented `up`, `down`, and reset commands must all work with that file alone.

An explicit `docker-compose.yudao-db.yml` overlay retains the current opt-in mode for environments that deliberately use an existing Yudao PostgreSQL network or DSN. The overlay, rather than the local baseline, owns the external network declaration. The README documents both modes and states that the acceptance path uses only the self-contained mode.

This preserves the external integration without making it an invisible prerequisite for local identity acceptance.

### Acceptance configuration and lifecycle

Add a PowerShell entry point, provisionally named `scripts/image-agent-local-acceptance.ps1`, with explicit subcommands such as `start`, `bootstrap`, `seed`, `status`, and `stop`. It is an orchestrator around existing application commands and Compose files, not a second application runtime.

`start` must:

- create an ignored `.local/image-agent-acceptance/` directory and generate local-only secrets there without printing their values;
- render and validate Compose before startup;
- start a dedicated Compose project with local ZITADEL, PostgreSQL, Redis, Temporal, the API, and the Image Agent worker using non-production names and labels;
- run the canonical schema migrations in their declared owner order against only the dedicated `image_agent_acceptance` database;
- record an acceptance environment marker generated for this local stack.

`stop` stops only the labeled acceptance project. A destructive data reset requires an explicit `-Reset` switch and displays the exact Compose project and named volumes before removal.

The script does not accept an arbitrary database DSN for the seed operation. Its runtime file contains the known local DSN and marker produced by `start`.

### Database guard

The seed command and acceptance health checks reject the environment unless all of the following hold:

- the current database name is exactly `image_agent_acceptance`;
- a local acceptance marker exists and matches the untracked runtime marker;
- the database endpoint belongs to the currently labeled acceptance Compose project;
- the canonical migration health check has completed successfully.

The marker is created by acceptance environment initialization, not by the task seed. It exists solely in the dedicated local database namespace. These independent checks prevent a loopback port-forward, a copied DSN, or a similarly named shared database from being treated as a writable local target.

If the existing canonical migration chain cannot create the required ListingKit tables on a blank database, the acceptance contract test fails. The fix belongs in the actual schema owner and migration ordering; the seed command must not compensate with raw SQL or fixture-only base tables.

### ZITADEL provisioning and identity

Add a small CLI adapter for `internal/zitadelprovision`; the package remains the only repository-owned client for these Management API operations. Its typed, idempotent operations will:

1. find or create the local ListingKit project;
2. ensure the existing ListingKit roles;
3. find or create an API application used by the Go API's token introspection client;
4. find or create a browser OIDC application with only local redirect URIs;
5. after a real browser token is available, grant that verified human user `listingkit_operator` (and only add `listingkit_admin` when explicitly requested);
6. emit non-secret identifiers and the exact audience/role scopes required by the local UI and API.

The first provisioning phase requires an operator to enter a local ZITADEL management token in the terminal or place it in the ignored local runtime file. It never takes that value from source control, command history, or chat. This phase must create the OIDC application before a browser user token can exist. Generated application secrets are written once to the ignored runtime configuration and are redacted from console and test output.

The browser login is intentionally a real local human login. After the provisioning phase, the user logs in through the generated OIDC application and the existing secure token-save workflow stores that bearer token in the acceptance directory. The authorization phase verifies this token, derives the current user, and grants the operator role. The seed tool then invokes the same ZITADEL discovery and introspection behavior as the API, validates an active token and the required ListingKit role, and derives:

- task tenant from ZITADEL resource owner;
- task user from ZITADEL subject.

To avoid a second token-parsing implementation, `internal/authruntime/zitadel` exposes a small verifier abstraction used by both its Gin middleware and the local seed command. It preserves discovery caching, audience-aware client authentication, canonical identity validation, and fail-closed errors. ListingKit route authorization remains owned by `internal/listingkit/httpapi`.

### Seeded task contract

Add a Go local seed command that receives only:

- the accepted local bearer-token file;
- the generated local runtime configuration;
- a required `-source-url` public HTTPS URL;
- optionally, a public HTTPS style image URL.

It does not accept tenant IDs, user IDs, task IDs, provider credentials, or image-agent run fields. After the database and token guards pass, it constructs the smallest task that satisfies the existing Image Agent workspace catalog:

- the derived tenant and user are stored on the ListingKit task;
- `StandardProductSnapshot` is present;
- `AssetBundlesByTarget["shein"]` contains one `KindSourceImage` with the caller-supplied safe URL;
- the optional style candidate is a safe non-source asset;
- no generated assets or false completion state are inserted.

The command uses the existing ListingKit task repository's create/read path. It never writes `listing_kit_tasks` directly. It derives a stable task identifier from a versioned seed namespace plus the verified tenant and subject. A repeated execution reads and verifies the existing task's owner, target, and asset URL; it succeeds only when the record is equivalent and otherwise refuses to overwrite it.

The seed output contains task ID, tenant ID, and user ID only after they have been derived locally, together with the safe workspace URL. It does not echo bearer tokens, passwords, application secrets, or full DSNs.

### Manual acceptance sequence

1. Run `start` to create the isolated environment and migrate the dedicated database.
2. Enter a management token only for the local `provision` operation, which creates the project, roles, API application, and browser OIDC application.
3. Log in through the generated local UI and save the real bearer token in the ignored acceptance directory.
4. Run `authorize`; it verifies that token and grants its derived subject the local operator role.
5. Run `seed -source-url <public-https-url>`; the command derives ownership from the now-authorized token.
6. Open the printed workspace URL, fetch the owned Image Agent preflight catalog, select the source, and create a manual run.
7. Verify the API projection and Temporal worker state for the exact run ID.

The first five steps validate local identity, schema, task ownership, and asset eligibility. Step six validates the browser route and run creation. Step seven validates real Temporal dispatch.

## Provider and storage boundary

The currently observed local image-provider configuration is not a valid governed `image_gpt_image_2` tenant route. That is deliberately not hidden by this work. With no supported tenant credential, the run must reach the existing runtime preflight and block with the real credential/provider-policy error before an external call.

A fully successful remote image-generation and COS publication acceptance requires two additional, explicitly authorized inputs:

1. a supported OpenAI-compatible tenant credential registered through the governed capability path; and
2. a dedicated acceptance COS bucket/prefix with write authorization.

Neither is created by the local seed. A local object store can support a later storage adapter test, but is not evidence of real COS publication. A public HTTPS source URL is required now because the existing safe-image policy must remain enforced.

## Test strategy

Implementation follows test-driven development. New behavior begins with failing focused tests.

### Compose and orchestration tests

- the base ZITADEL file renders without an external Docker network and includes its local PostgreSQL dependency;
- the Yudao overlay is the only configuration that declares the external network;
- `start` refuses an existing conflicting acceptance project and verifies rendered config before `up`;
- destructive reset requires the explicit switch and exact local project/volume identity;
- generated local files are ignored and secrets are not written to standard output.

### Provisioning and verification tests

- `internal/zitadelprovision` sends idempotent Management API requests for project, roles, API application, OIDC application, and authorization grant;
- retries/re-runs read existing resources instead of generating duplicates;
- error output is bounded and never includes management tokens or generated application secrets;
- the shared verifier maps active token subject/resource-owner/roles identically for middleware and seed callers;
- inactive token, wrong audience, missing resource owner, missing subject, and missing operator role fail closed.

### Schema and seed tests

- a fresh dedicated database reaches the canonical task-schema migration path without fixture table shells;
- unknown database name, absent marker, marker mismatch, and unlabeled endpoint are rejected before any task mutation;
- a valid token seeds an owned SHEIN task through the repository and creates no unrelated records;
- an identical rerun is a no-op; a changed owner, target, or source URL is rejected rather than overwritten;
- localhost, HTTP, private, and malformed URLs are rejected by the same safe-image policy used at runtime.

### End-to-end acceptance

- a real local browser user receives a bearer token that the API introspects successfully;
- that user can read only the seeded task's image catalog and can create one manual Image Agent run;
- the API records the derived tenant/user on the run and the Temporal worker receives the run;
- removing the role, changing the tenant, using another user, or omitting the token is denied;
- the unsupported/missing governed provider credential is surfaced as its real terminal or blocked reason, not reported as image-generation success.

## Compatibility and rollout

- Existing ZITADEL production and Yudao-connected development paths remain opt-in and unchanged by default.
- Existing ListingKit APIs, Image Agent workspace endpoints, route permissions, provider policy, and safe-image policy keep their public contracts.
- New local files remain under ignored `.local/image-agent-acceptance/`; no local credential, token, or seed URL is committed.
- The change is delivered as one local-acceptance slice because its Compose, identity, schema, and seed boundaries are mutually necessary to prove a real owned request. It excludes unrelated provider and COS configuration.

## Acceptance criteria

1. A developer can start a self-contained local ZITADEL environment without an external Yudao network or database.
2. The developer never manually enters a tenant ID or user ID.
3. A real local token is introspected by the same verification semantics as the API and produces the task tenant/user.
4. A seed command cannot write a non-acceptance database and never uses raw SQL to insert task fixtures.
5. The seeded task appears only to its verified owner through the existing ListingKit Image Agent endpoints.
6. The owner can create a manual Image Agent run and the local Temporal worker receives it.
7. Provider/COS prerequisites that are not configured remain explicit real failures; no mock/fallback image output is accepted.
8. Focused unit/integration tests and the documented end-to-end local command sequence pass with no secrets in tracked files or logs.

## References

- Image Agent manual workspace boundary: `docs/superpowers/specs/2026-08-29-image-agent-workspace-entry-and-style-authorization-design.md`
- Image Agent workflow ownership: `docs/superpowers/specs/2026-08-26-image-agent-workflow-design.md`
- ZITADEL authentication runtime boundary: `docs/superpowers/specs/2026-08-25-zitadel-authruntime-design.md`
- ListingKit schema migration unification: `docs/superpowers/specs/2026-08-08-listingkit-schema-migration-unification-design.md`
- ZITADEL API application token introspection: https://zitadel.com/docs/guides/integrate/token-introspection/basic-auth
- ZITADEL Management API application creation: https://zitadel.com/docs/reference/api/management/zitadel.management.v1.ManagementService.AddAPIApp
