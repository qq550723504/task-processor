# ListingKit deployment Secret preflight

## Goal

Fail the ListingKit API deployment before it mutates the Deployment when the
API-only member-invitation Secret is not ready.

## Design

The deploy workflow will run a versioned preflight script before applying
`product-listing-api-deployment.yaml`. The script reads only the metadata and
key presence of `listingkit-member-invitation-secret` in the target namespace.

The step fails when the Secret is absent or either required key is absent or
empty:

- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID`

It must never print Secret data. The existing shared-Secret legacy-key guard
remains unchanged and still runs before any Deployment mutation.

## Validation

Add a Go test that executes the script with fake `kubectl` and `jq` commands
for missing-Secret, missing-key, and ready-Secret cases. Keep a small
workflow-contract assertion that the preflight step appears before the
Deployment apply step. Run the focused Go test and workflow YAML
rendering/checks before delivery.
