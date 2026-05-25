# ListingKit Task Lifecycle Design

## Goal

Extract ListingKit task creation and read/query behavior out of the root `service` implementation into a focused collaborator while preserving the existing `listingkit.Service` API.

## Scope

This change only covers the task lifecycle slice inside `internal/listingkit`:

- `CreateGenerateTask`
- `GetTaskResult`
- `ListTasks`
- task dispatch helpers that only exist to support task creation

It does not move package boundaries across directories yet, and it does not change submit, revision, generation review, studio, or workflow APIs.

## Design

The root `service` stays as the public facade for `listingkit.Service`. Internally, it will delegate task-lifecycle responsibilities to a focused package-local collaborator. That collaborator will own the task-specific orchestration dependencies it actually needs instead of reaching through the entire `service` surface.

The first extraction remains inside package `listingkit` to avoid a Go import cycle. This is intentional: the current root package still owns the public models and interfaces, so a new subdirectory package would force a circular dependency. The immediate goal is to split responsibilities before changing package topology.

## Boundary Rules

- New task lifecycle behavior should be added to the focused collaborator, not directly onto the root `service`.
- The root `service` methods for task lifecycle become facade wrappers only.
- Shared task helpers may remain in package `listingkit`, but should migrate toward the focused collaborator when they are task-lifecycle-specific.

## Validation

- Add a test that proves the focused task lifecycle collaborator can handle task list queries with the existing semantics.
- Keep current `listingkit` service tests green for inline execution, SHEIN snapshot persistence, temporal dispatch, and task result behavior.
