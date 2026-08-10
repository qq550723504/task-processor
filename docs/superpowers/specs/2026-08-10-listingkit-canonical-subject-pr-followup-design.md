# ListingKit Canonical Subject PR Follow-up Design

**Goal:** Close the remaining canonical-ZITADEL-subject review findings without weakening tenant-wide credential behavior or digest-pinned rollback safety.

## Scope

This design addresses three release blockers reported on PR #109:

1. User-scoped `ai_client_credentials` must be validated against the canonical ZITADEL subject before owner scope becomes mandatory.
2. The ListingKit UI must receive the same canonical tenant, subject, and role allowlists as the API.
3. A source-built release must run a preflight binary compiled from the same source as its candidate API image, while a digest rollback keeps a workflow-version runner independent of the old candidate image.

## Architecture

### AI credential preflight inventory

`ai_client_credentials` is not a normal owner-scoped table: an empty `user_id` is a valid tenant-wide fallback credential. Add the table to the fixed preflight inventory with an explicit blank-user policy of `ignore`. The repository will aggregate only non-blank user IDs for that entry; existing owner tables retain their blocking blank-user policy. This preserves fail-closed behavior for rows that will become inaccessible while avoiding false blockers for valid tenant credentials.

The inventory guard will recognize the infrastructure credential model as an intentional, literal-table preflight entry. Repository and service tests will prove that a legacy non-empty credential user ID blocks the gate, while an empty user ID is omitted before directory lookup.

### UI canonical allowlists

Replace the UI's required deprecated role key with required canonical keys:

- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES`

The manifest will not project deprecated aliases. The UI already reads canonical names, and migration compatibility is intentionally not reintroduced. The existing parsed-manifest boundary test will assert the exact key set.

### Source-aligned preflight runner

The runner build job selects its checkout ref from release mode:

- normal source release: `needs.prepare.outputs.source_ref`;
- digest rollback: `github.workflow_sha`.

The runner image tag follows that selected ref, while its deployed reference remains the Buildx-produced digest. The deploy job continues to use workflow-version scripts and receives the runner digest output. This keeps the gate compiler aligned with new source schemas but makes rollback independent of the rollback candidate image.

## Error Handling and Safety

- Every SQL identifier stays compile-time inventory data and is validated before querying.
- Empty AI credential user IDs are intentionally skipped only for that table; blanks elsewhere remain blocking findings.
- The UI explicitly maps only allowlist keys it consumes; it does not regain broad Secret access.
- Digest rollback input restrictions and step-environment input isolation remain unchanged.

## Verification

- TDD RED/GREEN for the credential blank-user policy and canonical UI key projection.
- Parsed workflow test for source-ref runner checkout and workflow-SHA rollback checkout selection.
- Run the focused identity preflight, repository/config/workflow tests, then the existing release-driver and Kubernetes rendering checks.
