# Platform Boundary Strategy

## Goal

This document defines the near-term convergence strategy for platform-related
packages. The goal is not to migrate every package immediately. The goal is to
make future ownership decisions predictable so new features do not spread
platform rules across historical packages, ListingKit facade code, publishing
packages, and runtime registration packages at the same time.

Use `docs/architecture/project-boundaries.md` as the repository-wide entrypoint
for default package ownership and dependency direction. This strategy document
narrows that baseline for platform-related convergence work; it should not grow
into a second repository-wide policy source.

For migration-cost triage of historical platform packages, use
`docs/architecture/historical-platform-migration-inventory.md`.

## Current Package Families

The repository currently has four platform-adjacent families:

- Historical platform packages
  - `internal/shein`
  - `internal/temu`
  - `internal/amazon`
- Publishing packages
  - `internal/publishing/*`
  - `internal/marketplace/*/publishing`
- Product and listing facade packages
  - `internal/listingkit`
  - `internal/amazonlisting`
- Platform registration packages
  - `internal/platforms/*`

These families are allowed to coexist during migration, but new code should
choose one owner intentionally instead of copying behavior into several places.

## Stable Roles

### Historical platform packages

Historical platform packages continue to own existing API clients, payload
builders, validation rules, and runtime details that have not yet been split
into a clearer marketplace or publishing module.

They should not become the preferred home for new cross-platform abstractions.
When adding new platform behavior, first decide whether the behavior is:

- publishing workflow behavior
- workspace/editor behavior
- product facts or reusable asset behavior
- low-level external integration behavior
- temporary compatibility with a legacy platform package

### `internal/publishing/*`

`internal/publishing/*` is the convergence target for marketplace publishing
semantics that are not tied to HTTP handlers or process startup.

It should own:

- submission preparation and validation
- marketplace publishing policies
- publish payload shaping
- publishing result interpretation
- shared publishing vocabulary

It should not own:

- HTTP request parsing
- worker or scheduler startup
- raw external client construction when a local interface is enough
- product facts that belong in catalog or asset modules

Publishing convergence is guarded by:

- `TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit`
- `TestPublishingSheinNonAPISheinImportsStayAllowlisted`
- `TestPublishingCommonUsesCanonicalPackage`
- `TestPublishingCommonDoesNotImportPlatformImplementations`

Treat direct dependencies on legacy SHEIN runtime packages, ListingKit facade
code, or platform implementation packages as migration seams, not precedent
for new publishing behavior.

### `internal/listingkit`

`internal/listingkit` remains the product facade and listing orchestration
surface.

It may coordinate tasks, workflows, workbench state, review models, and
cross-platform listing concepts. It should not be the default place for new
SHEIN, TEMU, Amazon, or Walmart platform rules.

New platform-specific rules should move toward marketplace or publishing
packages unless the behavior is truly a ListingKit product concept.

### `internal/platforms/*`

`internal/platforms/*` is a thin registration and platform selection layer. It
can expose platform descriptors, registry behavior, or adapter lookup points.

It is not the future home for marketplace business rules. If a change requires
substantial validation, payload shaping, pricing, category, workspace, or
publishing behavior, the owning package should be a marketplace or publishing
module, with `internal/platforms/*` only delegating to it.

This thin-layer rule is guarded by
`TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages` and
`TestPlatformRegistrationPackagesStayThin`.

Platform registration packages must also stay free of checked-in local
artifacts. This is guarded by
`TestPlatformRegistrationPackagesContainNoLocalArtifacts`.

Current historical implementation imports from `internal/platforms/*` are
guarded by `TestPlatformModulesHistoricalImplementationImportsStayAllowlisted`.
Treat the Amazon, SHEIN, and TEMU registration delegates as explicit migration
seams, not as precedent for adding new business-rule imports to platform
registration packages.

## Migration Rules

When adding or moving platform behavior:

1. Keep existing historical package behavior in place unless there is a clear
   owner and a narrow migration slice.
2. Put new publishing semantics in `internal/publishing/*` or
   `internal/marketplace/*/publishing`.
3. Keep ListingKit focused on product orchestration and facade behavior.
4. Keep `internal/platforms/*` thin: registration, selection, descriptors, and
   delegation only.
5. Hide external clients behind local interfaces before moving behavior across
   package families.
6. If a legacy dependency remains, document the exception and add a cleanup path
   instead of treating it as a precedent.

If a platform-specific note starts reading like a repository-wide default rule,
promote the stable part back into `project-boundaries.md` and leave this
document focused on convergence choices and migration tradeoffs.

## Review Questions

When reviewing platform-related changes, ask:

1. Is this a marketplace rule, a product concept, a publishing workflow, or a
   runtime registration concern?
2. Did the change add new platform rules to the ListingKit root facade?
3. Did `internal/platforms/*` stay thin, or did it begin owning business rules?
4. Could the change live behind a local interface instead of importing a
   concrete historical platform package?
5. Does the change reduce future migration cost, or does it create another
   place that will need to be unwound later?

## Working Rule

Prefer narrow ownership over convenient reuse. A small adapter is better than
making ListingKit, publishing, historical platform packages, and platform
registration packages all know the same rule.
