# Image Agent Acceptance Command Ownership Design

**Date:** 2026-08-30

## Context

The local Image Agent acceptance seed is a developer-only executable used by
`scripts/image-agent-local-acceptance.ps1`. Its current executable lives below
`internal/listingkit/imageagentacceptance/cmd` and directly assembles GORM,
ZITADEL verification, and ListingKit repositories.

The repository command boundary requires every directory named `cmd` to depend
only on application-layer assembly packages, not business-domain or
infrastructure packages. Moving the executable below the ListingKit domain did
not satisfy that ownership rule. Making that executable import an application
runtime would also invert the dependency direction from ListingKit back to the
application assembly layer.

## Decision

The acceptance command and its assembly runtime will both be owned by the
application runtime tree:

- `internal/app/runtime/imageagentacceptance/runtime.go` owns flag processing,
  environment verification, GORM handle reuse, ZITADEL verification, ListingKit
  repository construction, seed execution, and JSON output.
- `internal/app/runtime/imageagentacceptance/cmd/main.go` is a thin executable
  that passes process arguments and standard streams to the runtime.
- `internal/listingkit/imageagentacceptance` retains only the domain-facing
  acceptance configuration, environment guard, and deterministic seed logic.
- `scripts/image-agent-local-acceptance.ps1` invokes the new command path.

The dependency direction is therefore:

```text
PowerShell orchestrator
  -> application-owned cmd
    -> application acceptance runtime
      -> ListingKit acceptance/domain packages and adapters
```

No depguard exception or allowlist will be added.

## Runtime Behavior

The command-line contract remains unchanged:

- required flags: `-runtime-file`, `-token-file`, and `-source-url`;
- optional flag: `-style-url`;
- success output: the existing JSON object containing task, tenant, user, and
  workspace identities;
- failure behavior: return the existing safe error and exit non-zero without
  printing secrets.

The orchestration script remains the only supported operator entrypoint. The
move changes package ownership and the internal `go run` path, not the manual
acceptance workflow.

## Error and Security Boundaries

- The application runtime must retain exact Compose project and loopback
  PostgreSQL binding validation before seeding.
- Token and runtime files remain local inputs; their contents must not be added
  to command errors or logs.
- The verified database handle continues to be reused so the environment guard
  and seed do not open inconsistent connections.
- ZITADEL subject and tenant equality checks remain in the existing acceptance
  seed path.

## Tests and Guardrails

- Add a semantic architecture test covering every production Go file beneath
  an internal `cmd` directory, matching the repository depguard policy.
- Move command behavior tests to the application runtime package, where the
  behavior is owned.
- Keep the executable thin enough that it requires no duplicate forwarding
  test.
- Run the focused architecture test and application runtime tests in the TDD
  cycle.
- Run the CI-equivalent depguard command from the repository root.
- Run serial full Go tests and classify any unrelated failures separately.
- Run the PowerShell acceptance contract tests because the orchestrator command
  path changes.

## Migration

1. Introduce the application acceptance runtime and move existing behavioral
   tests with it.
2. Move the thin executable from the ListingKit domain tree to the application
   runtime tree.
3. Update the PowerShell command path and acceptance documentation.
4. Remove the retired ListingKit-owned command files.
5. Verify, commit, and push the fix to Draft PR #267 without merging or
   deploying.
