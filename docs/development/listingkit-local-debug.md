# ListingKit Local Debug

ListingKit local scripts require the same ZITADEL authentication flow as every
other environment. There is no local identity, trusted identity header, or
authentication-disable mode.

## Start the local stack

Run the local stack normally:

```powershell
.\scripts\start-listingkit-local-dev.ps1
```

Before starting, provide a valid local ZITADEL issuer/client configuration in
`.env` (or inherited process environment). The API returns `503` for protected
routes until `ZITADEL_ISSUER_URL` and `ZITADEL_CLIENT_ID` are configured, and
then requires a verified bearer token. The UI redirects unauthenticated users
through the configured ZITADEL login flow.

Start each side separately when needed:

```powershell
.\scripts\start-listingkit-local-api.ps1
.\scripts\start-listingkit-local-ui.ps1
```

## Direct API Calls

For authenticated API calls, prefer the local machine-token helpers instead of
repeatedly copying browser tokens:

```powershell
.\scripts\listingkit-fetch-machine-token.ps1 -ApiBaseUrl http://localhost:8085
.\scripts\listingkit-auth-check.ps1
```

The helper writes the token under `.local\listingkit-api-token.txt` and can export `LISTINGKIT_API_TOKEN` for scripted checks.

## Safety

Do not use caller-supplied `X-User-*` or tenant headers as identity. ListingKit
derives identity from the verified ZITADEL bearer token in every environment.

`listingkit.ownerScopeRequired` is controlled by config or `TASK_PROCESSOR_LISTINGKIT_OWNER_SCOPE_REQUIRED`. The older `TASK_PROCESSOR_LISTINGKIT_ZITADEL_OWNER_SCOPE_REQUIRED` alias is still accepted for local `.env` compatibility.
