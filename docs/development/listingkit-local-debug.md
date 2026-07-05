# ListingKit Local Debug

ListingKit local scripts default to a Codex-friendly auth setup so local API/UI debugging is not blocked by stale or missing ZITADEL tokens.

## Default Local Mode

Run the local stack normally:

```powershell
.\scripts\start-listingkit-local-dev.ps1
```

By default this does two things:

- Starts the API with `-ZitadelAuthMode Disabled`.
- Starts the UI with `-BypassAuthGate 1`.

The API starter clears ZITADEL issuer/client/allowlist values from the current process before launching Go, so `.env` values cannot accidentally re-enable middleware. The UI bypass makes the Next auth gate and API proxy use a local debug identity instead of requiring browser login tokens.

That local identity still goes through the normal ListingKit data-permission headers. By default it is:

```env
LISTINGKIT_UI_LOCAL_DEBUG_TENANT_ID=default
LISTINGKIT_UI_LOCAL_DEBUG_USER_ID=local-debug
LISTINGKIT_UI_LOCAL_DEBUG_USERNAME=local-debug
LISTINGKIT_UI_LOCAL_DEBUG_ROLES=platform_admin,listingkit_admin,listingkit_operator
```

Use these variables when a local replay needs to behave like a specific tenant or user. The default `platform_admin` role lets the backend's existing ListingKit authorizer skip owner-only filtering without adding a special local-debug bypass in the Go API.

The same defaults apply when starting each side separately:

```powershell
.\scripts\start-listingkit-local-api.ps1
.\scripts\start-listingkit-local-ui.ps1
```

## Real ZITADEL Mode

Use real auth only when validating auth behavior or reproducing a production-like token issue:

```powershell
.\scripts\start-listingkit-local-dev.ps1 -ZitadelAuthMode Required -BypassAuthGate 0
```

Or start each side separately:

```powershell
.\scripts\start-listingkit-local-api.ps1 -ZitadelAuthMode Required
.\scripts\start-listingkit-local-ui.ps1 -BypassAuthGate 0
```

## Direct API Calls

When real API auth is required, prefer the local machine-token helpers instead of repeatedly copying browser tokens:

```powershell
.\scripts\listingkit-fetch-machine-token.ps1 -ApiBaseUrl http://localhost:8085
.\scripts\listingkit-auth-check.ps1
```

The helper writes the token under `.local\listingkit-api-token.txt` and can export `LISTINGKIT_API_TOKEN` for scripted checks.

## Safety

The bypass is for local development only. Do not use it for production checks, deployment validation, or tests whose purpose is to verify ZITADEL authorization behavior.

`listingkit.ownerScopeRequired` is controlled by config or `TASK_PROCESSOR_LISTINGKIT_OWNER_SCOPE_REQUIRED`. The older `TASK_PROCESSOR_LISTINGKIT_ZITADEL_OWNER_SCOPE_REQUIRED` alias is still accepted for local `.env` compatibility.
