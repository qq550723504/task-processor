# ListingKit deployment Secret preflight

## Goal

Fail the ListingKit API deployment before it mutates the Deployment when the
API-only member-invitation Secret is not ready.

## Design

The deploy workflow will run a versioned preflight script before applying
`product-listing-api-deployment.yaml`. The script uses `kubectl` to read the
Secret metadata and key values into memory, and checks key presence without
printing a value.

The step fails when the Secret is absent or either required key is absent or
empty:

- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID`

It must never print Secret data. The existing shared-Secret legacy-key guard
remains unchanged and still runs before any Deployment mutation.

## Validation

Add a Go test that executes the script with a fake `kubectl` command for
missing-Secret, missing-key, and ready-Secret cases. Keep a small
workflow-contract assertion that the preflight step appears before the
Deployment apply step. Run the focused Go test and workflow YAML
rendering/checks before delivery.
