# Issue 30 sourcing cutover

Status: independent review round 1 complete; account-access cutover blocked.
Baseline: main 2fd42cc06.

Follow-up decision: the user accepted the BLOCKER, paused production cutover and
old-route removal, and authorized all independent preparation slices. Current
READY/BLOCKED/WAITING_FOR_PREREQUISITE evidence and the required source-account
interface are in [the prepared-slices record](../../refactoring/2026-09-05-issue-30-prepared-slices.md).

Authority: Issue #30, Legacy Hard-Cut Policy, approved Phase 3 Product design.
Must: verified organization ownership, deterministic publication, durable lineage,
observable missing facts, explicit ImageAgent approval, no legacy task handoff.
Out of scope: crawling orchestration, marketplace submission, new image workflow,
new IAM, general tenantbridge migration, UI delivery and physical table deletion.

## Slices and owners

1. Product sourcing owns bounded product keys and publication identity. Preserve
   source-run identity; otherwise hash the canonical publication payload, including
   lineage. Same publication key with different payload is a Catalog conflict.
2. Application sourcing accepts the existing integration-owned 1688 snapshot,
   verified effective Organization ID and actor, source account and Store Center
   ID. Validate identity and access before publication. SourceEnvelope maps via
   the existing sourcing Publisher to the existing durable Catalog repository.
3. Store Center validates its own active organization-scoped store. Source-account
   access must retain existing ownership and disabled/deleted checks. Existing
   source_account rows use legacy numeric tenant IDs: this is a concrete unresolved
   ownership boundary, not permission for a new tenantbridge consumer or numeric
   fallback. Review must determine a bounded current-owner extraction or identify
   the required migration before this slice can be implemented.
4. Read canonical snapshot and approved inventory via existing Product ports.
   Source images never enter approval automatically. Controlled acceptance uses
   the existing ImageAgent approval owner and verifies missing-assets readiness
   before approval, scoped inventory and downstream readiness afterwards.
5. Switch app route and remove the entire old a1688 compatibility handoff and its
   obsolete task tests. Add a #30-specific architecture guard and dated evidence.

## Invariants and failure model

Catalog is the only Product fact authority; its atomic version/publication commit
is the only import write. No task, billing, crawl or asset write occurs on import.
Publication failure returns failure; response loss/restart/retry uses the same key
and payload. Concurrent replay and changed-payload conflict use existing Catalog
transaction behavior. Cancellation uses request context. There is no automatic
recovery loop: explicit retry of the import is the sole recovery entry.

Asset approval remains a separate existing ImageAgent transaction; import success
does not imply approval or marketplace readiness. No Saga or new retry owner.
Access is checked on every call without an application cache. Store and source
identity mismatches fail closed. The effective organization, never a body-supplied
tenant or home organization, scopes publication and reads. Existing authorization
middleware must resolve and authorize this organization for the new route.

HTTP input is bounded to 2 MiB and uses a bounded request deadline. Unknown legacy
fields are rejected. Missing facts and source errors are explicit. Tests cover
identity spoofing, unavailable access dependencies, wrong/disabled source/store,
publication replay/conflict, durable reread, no implicit approval, and removal of
the old route/imports. Focused domain, persistence, HTTP and architecture tests
are required before PR completion.

Legacy decision: EXTRACT -> RETIRE.
Reusable behavior: source identity, publication identity, access validation.
Current owner: Product sourcing, Store Center, source-account, Application.
Cutover/deletion condition: controlled current-domain chain and guards pass.

## Independent review, 2026-09-05

Finding: source-account has no current Organization ownership contract.
Product requirement affected: source-account access and tenant isolation.
Classification: BLOCKER.
Reason: sourceaccount/gorm_repository.go stores and queries numeric tenant_id;
sourceaccount/repository.go validates that numeric owner. A new effective
Organization cannot prove account ownership using that contract alone. Skipping
the mapping or interpreting the Organization number as the old tenant risks
incorrect authorization; denying every private account breaks that happy path.
Introducing a resolver wrapper would add a forbidden legacy consumer.
Action: keep production handoff unchanged until a source-account ownership
cutover is designed and reviewed. Do not add a compatibility Exception by default.

Evidence also includes crawler/alibaba1688/account_profile.go: the profile path
is derived from numeric tenant/account; worker_processor.go uses the task's
numeric tenant to resolve access. crawler/shared/task.go and shared result/service
contracts carry numeric tenant scope. Therefore migrating only the import HTTP
command would leave old account readers authoritative alongside the new one.

The reviewer classified Store Center access and asset/readiness as
IMPLEMENTATION_TEST: reuse current Store repository and ImageAgent assetpublication
publisher, with identity, lifecycle and approval-before-readiness tests. No new
approval workflow or generic authorization mechanism is required.

## Concrete prerequisite slice to resolve scope

1. Establish OrganizationID as the sole source-account ownership field and port.
   Inventory all existing rows through a read-only preflight and resolve ownership
   from authoritative Organization metadata; missing or ambiguous mappings block
   cutover. Do not infer ownership from numeric equality.
2. Design an explicit one-time backfill with a resumable receipt and validation
   before changing readers. This is a migration operation, never a request-time
   fallback or permanent dual read/write. Keep profile locations as validated
   opaque runtime references; changing authorization ownership must not silently
   move or discard authenticated browser profiles.
3. Drain the 1688 crawler's source-account readers and its persisted task/result
   ownership references to current Organization scope. Specify disposition of
   in-flight old jobs and responses before rollout; do not invalidate them by
   guessing that production has no outstanding jobs. Isolate shared crawler
   changes from other platforms.
4. Verify wrong-Organization/disabled/deleted access, missing and ambiguous mapping,
   migration restart, old-job cutover and profile reuse. Then review the import
   composition against this current port and complete #30's EXTRACT -> RETIRE.

This prerequisite changes database ownership, crawler execution/persistence and
browser-profile references. It exceeds a mechanical #30 handoff extraction and
requires resolving whether this larger slice is included or tracked separately.
No live migration or profile operation has been executed.

## Initial review evidence (superseded by the prepared-slices record)

- New PublicationIdentity tests failed first with the missing implementation,
  then passed after the Product-owned implementation was added.
- Focused sourcing, catalog and integration/crawler/a1688 tests passed.
- Five focused guards passed: Product domain outer-adapter boundary, Phase 3
  depguard scope, and the three 1688 ListingKit root/HTTPAPI dependency guards.
- New helper has not been wired into production. Old route has not been removed.
- No controlled import, approval/readiness acceptance, push or PR yet.
