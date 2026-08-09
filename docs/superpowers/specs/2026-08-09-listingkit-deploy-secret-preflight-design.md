# ListingKit deployment Secret preflight

## Goal

Fail the ListingKit API deployment before it mutates the Deployment when the
API-only member-invitation Secret is not ready.

## Design

The deploy workflow will add one shell step before applying
`product-listing-api-deployment.yaml`. It reads only the metadata and key
presence of `listingkit-member-invitation-secret` in the target namespace.

The step fails when the Secret is absent or either required key is absent or
empty:

- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID`

It must never print Secret data. The existing shared-Secret legacy-key guard
remains unchanged and still runs before any Deployment mutation.

## Validation

Extend the existing Go workflow-contract test to assert the dedicated-Secret
guard exists, checks both keys, and appears before the Deployment apply step.
Run the focused Go test and workflow YAML rendering/checks before delivery.
