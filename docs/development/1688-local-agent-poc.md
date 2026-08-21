# 1688 Local Agent POC

This POC lets an authenticated Windows CLI run the existing public 1688
crawler in the user's local browser environment. The API owns tenant identity,
job leases, result validation, and `SourceEnvelope` reconstruction.

## Run

Start the API locally with the normal development configuration. Then run the
confirmation-gated script from the repository root:

```powershell
pwsh -NoProfile -File .\scripts\1688-local-agent-acceptance.ps1 `
  -ApiBaseUrl http://127.0.0.1:18086 `
  -IssuerURL $env:LISTINGKIT_ZITADEL_ISSUER_URL `
  -ClientID $env:LISTINGKIT_ZITADEL_CLIENT_ID `
  -ProjectID $env:TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID `
  -Url https://detail.1688.com/offer/1052008074197.html `
  -Confirm CREATE-LOCAL-AGENT-JOB
```

The CLI performs device authorization, claims one tenant-scoped job, runs the
public processor in a short-lived browser context, and submits either a
narrow product snapshot or a sanitized failure category. The access token is
kept in process memory only. No cookies, source-account IDs, browser profiles,
proxy credentials, target stores, ListingKit drafts, or publish calls are part
of this flow.

Omit `-Url` only when a pending job was created separately through the API.
The POC stores jobs in API process memory; restarting the API drops them.
