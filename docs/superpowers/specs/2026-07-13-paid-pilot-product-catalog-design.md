# Paid Pilot Product Catalog and Usage Policy Design

**Status:** approved design; policy-document implementation pending review

## Purpose and scope

PAY-040 defines one customer-facing ListingKit paid-pilot package and the usage policy that later PAY-041 and PAY-042 work must implement. It resolves the current ambiguity between subscription-plan names and the actual product surface without changing any runtime entitlement, billing, or publication behavior.

This design covers only the invite-only paid pilot. It does not add self-service checkout, automatically charge a tenant, enable 1688, or make formal publication generally available.

## Options considered

1. **One `paid_pilot` plan with explicit capabilities — selected.** A stable customer-facing package is paired with independently auditable entitlements for the few actions that need extra safety review.
2. **One all-inclusive plan.** Simpler to describe, but would expose 1688 or formal publication before their safety gates are complete.
3. **Basic and Professional plans.** Rejected because current product entrances do not have a durable, tested mapping to those labels.

## Product model

### Plan identity and admission

- The sole launch plan code is `paid_pilot`.
- It is available only to a specifically invited tenant after an operator records a contract/order reference, plan effective period, quotas, and an approval actor.
- There is no public price page, self-service purchase path, or automatic plan upgrade. A tenant without the recorded commercial approval is not billable.
- Numeric price, currency, and quota values belong to the signed order and platform-admin record for that tenant. They are not inferred from internal usage, and they must not be exposed to other tenants.

### Capability matrix

| Capability | `paid_pilot` default | Additional gate | Notes |
| --- | --- | --- |
| Create ListingKit task | Enabled after tenant admission | Existing runtime quota and authorization controls | Admission records a task-capacity allowance; task creation is not a standalone billable completion in the first pilot. |
| SDS source | Enabled after tenant admission | Store/source configuration and preflight | Included source path for the launch package. |
| Studio design generation | Enabled after tenant admission | Successful job completion for usage commitment | Internal AI cost is recorded but is not a customer-facing per-call charge. |
| Product-image generation | Enabled after tenant admission | Successful job completion for usage commitment | Failed or cancelled work creates no committed usage. |
| SHEIN save draft | Enabled after tenant admission | Store preflight and existing safety controls | A successful remote draft is a billable outcome. |
| SHEIN formal publish | Disabled | Separate `shein_publish` entitlement plus tenant preflight, successful draft validation, and explicit business approval | Never enabled by plan assignment alone. |
| 1688 source | Disabled | M2 completion and a later explicit product-policy update | PAY-040 must not expand this source while its traceability and safety work is incomplete. |

## Usage and billing policy

### Metrics

The policy vocabulary for the future idempotent ledger is:

- `listing_tasks_created` — operational quota/admission metric only in the first pilot; not directly invoiced.
- `studio_design_jobs_succeeded` — committed only after a successful design job.
- `product_image_jobs_succeeded` — committed only after a successful image job.
- `shein_drafts_succeeded` — committed only after a successful remote draft.
- `shein_publishes_succeeded` — committed only after a successful remote publication and only where `shein_publish` is separately enabled.
- `storage_bytes_current` — measured as the tenant's current retained object size, not upload volume or byte-hours.

### Event outcomes

| Outcome | Usage treatment |
| --- | --- |
| Eligible action succeeds | Commit the applicable usage metric once. |
| User cancels before successful completion | Do not commit usage; release any prior reservation. |
| Platform rejects or action fails | Do not commit usage; release or reverse any prior reservation. |
| Engineering retry/recovery replays the same business action | Reuse the existing business event; do not create additional customer usage. |
| Remote result is unknown | Hold the event out of invoicing until reconciliation determines a final outcome. |

Storage remains chargeable only as a current-occupancy measure governed by the tenant's manually approved allowance. Deleting an object reduces future current-occupancy usage; it does not create a refund event for an already completed generation or remote submission.

## Operational and security boundaries

- Plan assignment, price, contract/order reference, entitlement changes, quota overrides, suspension, and expiry actions are platform-admin operations with actor, before/after values, reason, and timestamp audit fields.
- Customer-facing responses may disclose the plan, enabled feature, metric, used amount, limit, and support path. They must not disclose contract price, another tenant's limits, or internal AI cost.
- A suspension or expiry disables new eligible actions according to the recorded policy; it does not erase history or permit an internal retry to bypass entitlement checks.
- No workflow in PAY-040 activates a charge. Charging remains prohibited until the M4 ledger, enforcement, and reconciliation work are complete.

## Implementation boundary and verification

The PAY-040 implementation is a policy document and execution-plan update only. PAY-041 will provide idempotent events and reservation/commit/release semantics; PAY-042 will enforce the catalog consistently at public, batch, retry, and recovery entrances. Subsequent tests must prove that a plan assignment alone cannot publish, a rejected/cancelled/replayed action is not charged, and 1688 remains unavailable before its separate gate is completed.
