# Legacy Retirement Zone

`internal/compatibility` is **not** a target architecture layer and is not a long-term home for backward-compatible facades.

The repository uses `Hard-Cut + Selective Extraction`:

- `EXTRACT`: move still-correct behavior into the current Domain/Capability owner, switch callers, then remove the legacy dependency.
- `RETIRE`: do not extend obsolete design; remove it after cutover.

There is currently no Legacy Compatibility category.

Rules for this tree:

- do not add new consumers of `internal/compatibility/*`;
- do not add a new compatibility package for a new feature;
- do not add fallback, dual-read, dual-write, or bidirectional synchronization to preserve an old internal design;
- existing production paths are migration debt and must have an `EXTRACT -> RETIRE` destination in `docs/refactoring/legacy-register.md`;
- historical code may remain temporarily only until its current owner has completed cutover.

See `docs/refactoring/legacy-hard-cut-policy.md`, `docs/refactoring/legacy-register.md`, and GitHub #300.
