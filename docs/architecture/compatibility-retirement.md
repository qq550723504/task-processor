# Compatibility Retirement

## Goal

This document records compatibility paths that have already been retired and
the tests that keep them from becoming official entrypoints again.

Compatibility layers are useful during migration, but once in-repository usage
is gone they should not silently return as parallel APIs.

## Retired App Compatibility Paths

| Path | Status | Replacement | Guard |
| --- | --- | --- | --- |
| `internal/app/processor` | Retired | `internal/processor` | `TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer`, `TestAppProcessorCompatibilityLayerIsRetired` |
| `internal/app/state` | Retired for Go code | `internal/state` | `TestInternalPackagesDoNotImportAppStateCompatibilityLayer`, `TestAppStateCompatibilityLayerIsRetired` |

The retirement condition for both paths is zero in-repository imports. New Go
code must not import either app compatibility path.

`internal/app/state` may still appear locally as an ignored runtime-artifact
directory on developer machines, but it must not contain tracked Go
compatibility files. State owners should use `internal/state` directly.

## Retirement Conditions

Before retiring any future compatibility path, verify:

1. zero in-repository imports
2. external users have moved or the next removal window is documented
3. a replacement owner is named
4. an import or structure guard prevents recreation

## Review Questions

When reviewing compatibility-related changes, ask:

1. Is this path still needed for a migration, or is it recreating a retired API?
2. Does the replacement owner already exist and have tests?
3. Is the exception narrow and documented?
4. Will a test fail if someone reintroduces the old path?
