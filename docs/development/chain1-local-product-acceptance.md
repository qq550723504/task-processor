# CHAIN-1 local product acceptance

This is the isolated, opt-in acceptance assembly for Issue #388. It proves the
current product path against real PostgreSQL in a task-owned empty database and
a real Temporal server. It does not mount a default runtime, deploy anything, call a
paid provider, or access shared/real business data.

Run from the repository root on Windows:

```powershell
.\scripts\chain1-local-product-acceptance.ps1
```

The runner first proves that its script root is the Git repository root, resolves
`HEAD`, and requires an empty index and worktree, including no untracked files.
If a Git query fails or the tree is dirty, it exits non-zero before any Docker or
Go command, does not clean resources it never touched, and does not emit a
`CHAIN1_HEAD` attribution. Commit the intended acceptance changes before running
the real chain; the runner never stashes, resets, deletes, or auto-commits them.

The script owns only the Compose project `task-processor-chain1-acceptance-388`,
localhost ports 17443/17333/18333, and its two named volumes. It creates a fresh
database/Temporal history for each ordinary run and removes those resources in
`finally`. `-KeepResources` retains them solely for local diagnosis; rerunning
without that switch still starts from empty resources.

Internal behavior is real: SRC-1 publication, REV-1 HTTP review/apply, the
Organization ImageAgent PostgreSQL repository, Temporal workflow/activities,
current embedded ImagePolicy resolver, worker authorization, durable
staging/publication, Product Asset,
Store Center subscription/quota/audit/create, and DRAFT-S1 HTTP persistence.
The controlled external fixtures are limited to token verification, the
loopback ZITADEL Authorization API, image capabilities, and immutable object
storage. Material outputs plus authorization and image-capability invocation
counts are asserted; no external network or provider call is made.

The recovery cases lose committed Review Apply, ImageAgent approval, and DRAFT
responses. ImageAgent HTTP/worker and DRAFT are rebuilt after their respective
response losses before durable projection/receipt verification. Compose native
command failures produce a non-zero process result and cannot emit final PASS.
A Windows execution-level matrix covers a clean run, staged/unstaged/untracked
changes, Git lookup failure, Compose startup failure, business failure before the
contract command, and final cleanup failure. Those command-shim cases verify the
runner protocol and command counts only; they are not PostgreSQL/Temporal PASS.

Output uses `CHAIN1_STAGE <name> PASS|FAIL|SKIP|NOT_RUN`. A PASS is emitted only
after the named assertion has run. A skipped environment-gated Go test is SKIP,
never PASS. The runner initializes resources, business-chain, contract, cleanup,
and acceptance as NOT_RUN before any fallible preflight. It then emits the actual
terminal state for every reached stage. Thus an early resource failure leaves the
business and contract stages explicitly NOT_RUN, and a business failure leaves
the contract explicitly NOT_RUN. Cleanup reports only what actually happened;
any required step or cleanup failure forces acceptance FAIL. The final receipt
binds the exact clean git HEAD, org, actor, product, applied Catalog version/publication,
ImageAgent run/plan/result/action, Product Asset inventory, Store, Listing
record, and diagnostic status. Local PASS is not CI, merge, deployment, runtime
health, or business acceptance.
