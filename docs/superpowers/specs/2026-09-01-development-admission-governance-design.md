# Development Admission Governance Design

## Context

PR #270 showed that a capable model and high reasoning effort do not replace
engineering admission controls. The pull request combined identity,
authorization, persistence, quota accounting, audit recovery, HTTP policy,
BFF behavior, frontend state, and CI changes in one review surface. The design
described the intended properties, but it did not make every durable failure
boundary, recovery owner, or retry outcome explicit before implementation.

The repository already has strong import-boundary guards and a detailed
architecture review checklist. It does not currently have repository-wide
rules that stop an oversized or consistency-sensitive change before coding,
require an independent design challenge, or make pull-request scope visible to
CI and reviewers.

## Goal

Add a repository-owned development admission mechanism that:

1. stops broad or consistency-sensitive work before implementation;
2. requires falsifiable designs for stateful and distributed behavior;
3. separates design authorship from design approval;
4. keeps implementation and review surfaces small;
5. enforces measurable pull-request scope limits with an explicit maintainer
   override;
6. preserves the existing architecture guard inventory as the authority for
   structural boundaries.

The mechanism must guide Codex and human contributors consistently. It must not
claim that line-count limits prove correctness or allow an override label to
replace design evidence.

## Approaches considered

### Documentation-only guidance

Add advice to the pull-request template and architecture checklist. This is
low cost, but it repeats the failure mode from PR #270: broad guidance can be
acknowledged without stopping implementation. Rejected as insufficient.

### Hard size limit without an exception path

Fail every pull request above a fixed file or line threshold. This is simple,
but generated changes, mechanical migrations, and intentionally approved
cross-cutting changes sometimes need a larger review surface. Rejected because
contributors would either bypass the check or distort commits to satisfy it.

### Layered admission controls with a governed override

Combine durable agent instructions, a falsifiable architecture checklist, a
pull-request declaration, and a CI scope guard. Oversized work fails closed
unless the pull request has an `architecture-approved` label, a maintainer or
administrator `APPROVED` review for the current head SHA, and a record of why
splitting is unsafe or counterproductive. The label alone is not authorization.
This is the selected approach.

## Admission model

Development passes through the following gates:

```text
scope classification
  -> design completeness
  -> independent design challenge
  -> implementation slice plan
  -> fault-oriented verification
  -> stable pull request
  -> code review
```

A failed gate returns work to the preceding design step. It does not authorize
a local patch that weakens an invariant.

## Scope classification gate

Work is architecture-sensitive when any of the following is true:

- it crosses three or more independently owned subsystems;
- it crosses more than one consistency boundary, such as a database plus an
  external service, queue, cache, filesystem, or browser-held state;
- it introduces or changes a state machine, compensation path, reconciliation
  path, authorization boundary, tenant boundary, or destructive operation;
- it is expected to change more than 30 scope-relevant files;
- it is expected to add more than 1,500 production lines or change more than
  2,500 production lines;
- it mixes a foundational refactor with user-visible feature delivery.

When a threshold is reached during development, work stops and is reclassified.
The design must either split the work or document why a larger atomic change is
necessary.

The numeric thresholds are review-surface tripwires, not quality targets.
Contributors must not pad, compress, or reorganize code merely to evade them.

## Design completeness gate

Architecture-sensitive work must identify:

- business and security invariants;
- authoritative data owners and transaction boundaries;
- states, events, preconditions, transitions, and durable effects;
- idempotency identities and the explicit same-key/different-payload result
  (reject, or return the original result only when the payload matches);
- recovery ownership and the trigger that makes recovery reachable;
- behavior before and after every durable write;
- lost-response, retry, restart, cancellation, and concurrency outcomes;
- tenant, authorization-revocation, and cross-context behavior;
- bounded request, timeout, and resource-exhaustion policy;
- verification evidence for every invariant and failure boundary.

When all durable effects share one database, the design must evaluate a shared
transaction or Unit of Work before introducing compensation. When effects cross
durability boundaries, the design must evaluate established transactional
outbox, Saga, or the repository's existing Temporal facilities before creating
a custom recovery protocol.

Stateful designs must contain a failure matrix. At minimum, each row records:

| Boundary | Durable state after failure | Retry identity and result | Recovery owner | Verification |
| --- | --- | --- | --- | --- |
| Before effect | What is absent | Whether retry is safe | Caller or service | Test name |
| After effect, before response | What is committed | How the original operation is found | Service or reconciler | Lost-response test |
| During compensation | Partial state | Conflict/replay behavior | Reconciler | Fault-injection test |

Vague phrases such as "support idempotency", "roll back on failure", or
"eventually reconcile" do not satisfy this gate without the corresponding
identity, durable state, owner, and verification.

For an idempotency-key operation, the design must also specify the key
namespace across tenant, actor, endpoint, and business object; the canonical
payload fingerprint; the result for same-key/same-payload retries; a fixed
conflict response with no side effects for same-key/different-payload retries;
behavior while the first request is in flight, failed, timed out, or recovering;
key expiry and reuse; the single owner for same-key concurrency; and behavior
when the authorization context changes.

## Independent design challenge

The authoring task cannot approve its own architecture-sensitive design. A
fresh human or Codex review context must try to falsify it before implementation.
The review must look for:

- ambiguous or unreachable recovery states;
- partial persistence and lost responses;
- same-key and different-key races;
- stale snapshots and version drift;
- multi-tab, cache, cookie, and tenant-context drift;
- authorization revocation during an operation;
- slow requests, missing deadlines, and resource retention;
- sibling routes or consumers that do not share a cross-cutting policy.

The pull request records the design-review evidence or an explicit maintainer
decision accepting a residual risk. A model name or reasoning-effort setting is
not review evidence by itself.

## Implementation slicing

Architecture-sensitive work is implemented as independently verifiable slices.
Foundational consistency mechanisms precede API and UI consumers. A typical
sequence is:

1. transaction, operation journal, or recovery foundation;
2. domain state machine and repository contract;
3. HTTP/API contract;
4. BFF or client adapter;
5. UI behavior;
6. rollout and observability.

Each pull request should remain mergeable with incomplete user-facing behavior
disabled behind an existing feature or routing boundary. A pull request must
not combine unrelated cleanup with the feature slice.

## Pull-request scope guard

The authoritative guard will inspect changed files through the GitHub
pull-request API using the maintained open-source `actions/github-script` action
instead of implementing a custom GitHub API client.

The guard computes:

- scope-relevant changed file count;
- production additions;
- production churn (`additions + deletions`).

Documentation, lockfiles, generated files, snapshots, and test-only files do
not count as production lines. Test files still remain visible in the pull
request declaration and review; they are excluded only from the production-line
threshold. Documentation-only pull requests are outside the size guard.

The default limits are:

- at most 30 scope-relevant changed files;
- at most 1,500 production additions;
- at most 2,500 lines of production churn.

Exceeding any limit fails the job unless the pull request has the
`architecture-approved` label and a maintainer or administrator `APPROVED`
review whose `commit_id` equals the current head SHA. When overridden, the
pull-request template must name the approving maintainer, link the approved
design, and explain why the change cannot be split safely. CI will report the
measured values and the override status in its log. A later head change makes
the approval ineligible until a new current-head approval is submitted.

The authoritative guard runs in the dedicated
`.github/workflows/development-admission.yml` workflow on an unfiltered
`pull_request_target` trigger for opened, synchronize, reopened, edited,
labeled, and unlabeled events. It checks out only the trusted default-branch policy revision;
it never checks out or executes pull-request code. It reads the PR metadata before
and after pagination, fails closed if the head SHA, base SHA/ref,
`merge_commit_sha`, update time, or `changed_files` count moves, if labels,
reviews, or base-reference-change events move, and fails closed if the paginated
file list does not equal `changed_files`. It publishes a commit status with the
fixed context `Development Admission` to the PR's current test merge commit
(`merge_commit_sha`): `pending` before evaluation and `success`, `failure`, or
`error` after evaluation. This keeps evaluations for the same head against
different base branches from overwriting one another. Evaluations are serialized
per PR so an older run cannot overwrite a newer result. When the override label
is present, the workflow reads pull-request reviews and the reviewer's repository
`role_name`; only a non-author collaborator with `maintain` or `admin` role plus
an `APPROVED` review for the current head submitted after the latest base
retarget can authorize the override. The trusted workflow has `contents: read`,
`issues: read`, `pull-requests: read`, and `statuses: write`; it never creates
labels or edits the pull request. The existing `ci.yml`
workflow runs the proposed classifier's unit tests on `pull_request`/push events,
but is not the authoritative admission decision. The first PR adding the
trusted workflow is a bootstrap exception that requires maintainer review; after
merge, branch protection must require the `Development Admission` commit-status
context for the guard to block merges.

The admission evaluator's failure matrix is:

| Boundary | Durable state after failure | Retry identity and result | Recovery owner | Verification |
| --- | --- | --- | --- | --- |
| PR metadata, review, or event read fails | No new status is trusted; any prior status is not refreshed | The PR number and current test-merge SHA identify the next run; retry on the next PR/review event or manual rerun | GitHub Actions and maintainer | API-error and timeout path |
| Any PR input changes between snapshots | The old target receives `error` when possible; no success is published for the stale snapshot | Same PR event is retried against the newly fetched head/base/merge/review state | Per-PR serialized evaluator | Moving-snapshot tests |
| Status publish fails | Evaluation result is not considered authoritative | Retry the same PR event; no local write can substitute for the missing repository status | GitHub Actions/GitHub status service | Status-write failure path |
| Review approval, label, or maintainer role is revoked | The next review event evaluates as blocked and publishes `failure` when the token permits it | Current head plus latest review state is re-read; stale approval is never reused | Trusted evaluator, with branch protection/ruleset as final owner | Dismissed-review, label, and role-change tests |

The current repository has no branch-protection required status check or ruleset;
the rollout must enable the `Development Admission` status after this workflow
is merged. Merge-queue support is not claimed by this PR: if a `merge_group`
required workflow is introduced later, it needs a separate adapter for its
merge-group SHA and constituent PRs before the gate is required there.

## Repository surfaces

### Root `AGENTS.md`

Create repository-wide instructions that preserve the existing user rules:

- report invalid instructions instead of executing them;
- solve root causes rather than symptoms;
- report architectural problems discovered during development;
- reuse established open-source or existing repository implementations.

Add the admission gates, stop conditions, independent-review rule, and
fault-matrix requirement. Nested `AGENTS.md` files may add local rules but may
not weaken repository-wide safety and architecture gates.

### Pull-request template

Add fields for:

- scope classification and affected subsystems;
- consistency and authorization boundaries;
- design and independent-review evidence;
- invariants and failure-matrix evidence;
- measured or expected size;
- `architecture-approved` override justification;
- focused validation and fault-injection results.

Keep the existing architecture boundary checklist and reviewer notes.

### Architecture review checklist

Add a development-admission section before the existing structural checks. It
will remain a review guide, while the current `Guard Baseline` remains the
authoritative import-boundary inventory.

### CI workflows

Add the authoritative `development-admission` job to the dedicated
`.github/workflows/development-admission.yml` workflow. Keep it unfiltered so a
PR changing any repository path is evaluated. Use `pull_request_target` so the
workflow and checked-out policy come from the trusted default branch, and include
label and `edited` activity types so adding or removing `architecture-approved`
or retargeting immediately re-evaluates the same PR. Also trigger on
`pull_request_review` `submitted`, `edited`, and `dismissed` events so approval
and revocation cannot leave a stale result. Use `concurrency` per PR to serialize
evaluations and give the job a bounded deadline. Publish the decision as the fixed `Development
Admission` commit-status context on the PR's `merge_commit_sha`, because the
workflow job's automatic check is attached to the default-branch SHA and a
head-only status can be shared by PRs with different base branches. Require the
current-head maintainer/admin review described above before accepting the
override label.

Keep only the classifier unit-test job in `.github/workflows/ci.yml`; its
pull-request path filters are for ordinary CI and must not be used as the
admission boundary. Include the trusted workflow path in those filters so
proposed workflow changes still run the tests. The action must be pinned
consistently with the repository's existing GitHub Actions policy. Classification
and snapshot validation remain in a dedicated checked-in JavaScript module with
unit tests; workflow YAML is only orchestration.

## Verification

Implementation verification must include:

- tests for below-limit, each over-limit, exempt-file, pagination completeness,
  rename paths, repository test suffixes, snapshot changes, merge-target
  isolation, label-only rejection, current-head maintainer/admin approval,
  stale-review rejection, and override cases;
- a spec-to-document self-review of the PR template, root `AGENTS.md`, and
  architecture checklist; these human and agent instructions must not gain
  brittle tests that merely lock exact prose;
- the open-source `actionlint` validator for workflow structure and expression
  correctness instead of a repository-specific YAML keyword scanner;
- manual inspection that the trusted workflow has no path filter, uses only the
  default-branch policy revision, uses only `contents: read`, `issues: read`,
  `pull-requests: read`, and `statuses: write`, does not execute pull-request code, publishes
  status only to the current `merge_commit_sha`, verifies current-head review
  authorization before applying an override, and does not mutate labels or pull
  requests;
- `git diff --check` and the focused Go or JavaScript tests owning the new
  guard behavior.

## Rollout

The first release enables the CI limits immediately because the override path
prevents legitimate work from being blocked without recourse. The PR that first
adds the trusted workflow cannot trigger that new default-branch workflow until
it is merged, so it is a documented bootstrap exception requiring maintainer
review. Maintainers must create the `architecture-approved` repository label and
require the `Development Admission` commit-status context in branch protection
before relying on the gate. The label is only a declaration; the workflow also
requires the current-head `APPROVED` review from a collaborator with `maintain`
or `admin` permission. If the label does not exist, ordinary pull requests
continue to work; only oversized pull requests lack an override until the label
and protected approval are present.

Threshold changes require an architecture-checklist update and tests in the
same pull request. Repeated override use is treated as evidence that either the
thresholds or repository boundaries need architectural review.

## Non-goals

- Proving correctness from pull-request size.
- Replacing code review, security review, or branch protection.
- Forcing every small change to produce a design document.
- Requiring a new orchestration dependency when a shared database transaction
  already solves the consistency problem.
- Automatically applying approval labels or accepting residual risk on behalf
  of a maintainer.
