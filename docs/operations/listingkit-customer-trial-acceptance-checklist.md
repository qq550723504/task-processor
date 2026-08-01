# ListingKit Customer Trial Acceptance Checklist

Use this checklist for each controlled customer-trial release. It records
evidence and a human Go/No-Go decision; passing local or CI checks alone does
not authorize customer publishing.

The release operator owns the record. Do not store passwords, bearer tokens,
cookies, store secrets, or unredacted customer product data in this document or
its evidence attachments.

## Release identity

| Field | Record |
| --- | --- |
| Release candidate source SHA |  |
| API image tag |  |
| UI image tag |  |
| Environment / tenant |  |
| Operator |  |
| Business approver |  |
| Start / end time |  |

## Preflight

- [ ] The [Release Candidate Runbook](./listingkit-release-candidate-runbook.md)
  is completed through its pre-release gate.
- [ ] CI evidence is retained for the recorded immutable SHA, including Go
  tests, frontend lint/typecheck/tests/build, image builds, and Kustomize
  rendering.
- [ ] The controlled tenant, operator identity, and test store are recorded
  outside this repository and are isolated from customer production data.
- [ ] The operator can authenticate through the configured HTTPS OIDC flow and
  receives the intended tenant and ListingKit role.
- [ ] A restore point, recovery owner, and previous immutable image tags are
  recorded before a data-changing migration or a remote publish attempt.

## Customer workflow evidence

Record a task ID and the corresponding evidence location for each completed
row. A failed or blocked row is not a pass: record the failure phase,
recoverability, and closure criterion in a validation run.

| Capability | Required evidence | Pass criteria | Result / evidence link |
| --- | --- | --- | --- |
| Tenant and access isolation | Operator and non-member access attempts | Operator reaches only the controlled tenant; unauthorized or cross-tenant access is denied |  |
| Product import | Source request and created task ID | Source identity, title, variants, and assets are visible in the task |  |
| AI processing and review | Generated result and reviewer action | AI output is reviewed or edited before it becomes the approved listing data |  |
| Readiness | Preview response and blocker summary | Structured blocking items and repair actions are visible; a ready case is explicitly recorded |  |
| Controlled save draft | Task ID, attempt ID, remote draft ID | The expected SHEIN draft is created once; duplicate clicks do not create another remote attempt |  |
| Publish decision | Approval and final submit record | Customer publishing is enabled only after all blocking readiness items are closed and the approver authorizes it |  |
| Result and task center | Final task status and operator view | The operator can find the result, remote identifier, error details, and next action without application logs |  |
| Recoverable failure | Failed task/attempt and retry evidence | Failure phase and user action are clear; retry or escalation follows the documented recovery path |  |

## Operational verification

- [ ] API and UI workloads use the recorded immutable images and are available.
- [ ] `/health` and in-cluster `/readyz` evidence is attached according to the
  Release Candidate Runbook.
- [ ] Authenticated ListingKit settings health and controlled-tenant preflight
  have been checked without recording credentials.
- [ ] Backup/restore, rollback, and alert ownership have been verified for this
  release or explicitly marked as a release blocker.
- [ ] At least one observation window after the controlled workflow is recorded;
  unresolved errors or unknown states block customer expansion.

## Decision

| Decision | Name | Time | Notes |
| --- | --- | --- | --- |
| Go / No-Go |  |  |  |
| Engineering sign-off |  |  |  |
| Business sign-off |  |  |  |

Mark **No-Go** if a P0/P1 customer-flow defect is open, remote submission is
not auditable, a required recovery path is untested, or any prerequisite
evidence is missing. Preserve the validation records and record the next owner
and closure criterion before retrying the release.
