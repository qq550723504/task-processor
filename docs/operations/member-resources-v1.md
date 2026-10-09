# Member resources v1: configuration and user handoff

This is the #561 delivery path governed by
[the admitted design](../architecture/member-resource-allocation-v1.md).
It does not extend RUN-1 or authorize replacing the retained localhost:22744
acceptance environment. Implementation checks and synthetic provider fixtures
are separate from real SHEIN authorization and user acceptance.

## Normal runtime

Use the current application entrypoint and an absolute private JSON manifest:

```powershell
go run ./cmd/current-application -config C:\private\current-application.json
```

Before starting a fresh installation, provision the current owner databases and
narrow runtime roles using their existing schema-owner entrypoints. Resource
positions and reservations belong to `commercialOwnerDatabase`; acquisition
operations/publication evidence belong to `productAcquisitionDatabase`; Store
records, member grants and encrypted connections belong to a separate
`storeCenter.database`. Serving never installs schema or seeds balances/prices.

The Store fresh initializer has the following command shape. Owner files carry
private database credentials and must not be used as serving credentials:

```powershell
go run ./cmd/store-center-schema-init -config C:\private\store-owner.json -quota-config C:\private\commercial-owner.json
```

Store quota uses `store_quota_runtime` on the canonical commercial target;
Store records use `store_center_runtime`. Both pools permit at most eight
connections. Membership requires its existing provider read/write configuration
and a live canonical member grant. Current assembly mounts member-resource APIs
when membership and the canonical commercial owner are configured. Store
assignment facts require the Store owner. Missing owners remain explicitly
unavailable; reads do not infer assignments or clear departed members' resources.

Start the existing Console against that application using its normal deployment
configuration (`LISTINGKIT_SERVICE_API_BASE` ends in `/api/v1`, public origin and
OIDC configuration match the actual HTTPS origin). From `web/listingkit-ui`:

```powershell
pnpm.cmd install --frozen-lockfile
pnpm.cmd build
pnpm.cmd start
```

The BFF binds operations to the signed-in user, selected enterprise cookie and
captured user/enterprise headers. Writes also require the configured public
same origin. Do not use old API composition to bypass unavailable current owners.

## Optional official SHEIN applications

The user confirmed no approved developer application exists yet. Omit
`storeCenter.officialApplications` until it exists. Store CRUD, resource reads,
allocation and grants remain available; real authorization and first service
activation remain blocked. No mock provider is mounted.

Once approved, add each application to the `storeCenter.officialApplications`
array inside the private manifest. Each entry is:

```json
{
	"type": "self_operated",
  "appId": "approved-application-id",
  "version": "configuration-version-1",
  "apiOrigin": "https://openapi.sheincorp.com",
  "callbackURL": "https://your-console-origin/workbench/stores/shein/callback",
  "appSecretFile": "C:\\private\\shein-app-secret",
  "credentialKeyFile": "C:\\private\\store-credential-key",
  "credentialKeyId": "store-key-1"
}
```

The approved regional host may instead be `https://openapi.sheincorp.cn`.
The type must match the official developer application's actual type:
`self_operated`, `semi_managed`, or `fully_managed`. Site `store_type` does not
identify it. Up to 16 entries may be configured; AppID, encryption-key identity
and private files must be unique across entries. The browser selects an admitted
application when connecting; callbacks and execution resolve the saved original
AppID and revision. The persisted revision includes the type. Removing or changing
the configuration makes that connection unavailable; it never falls back to
another application. Types without configuration are visibly unavailable.
Use the exact callback registered with SHEIN. The two files must be distinct
absolute private regular files: application secret and a Base64-encoded random
32-byte credential encryption key. Protect them with current-user/private
permissions; the runtime verifies permissions. Keep the encryption key while
connections use it. No secret values go in source control, browser storage,
provider error text or logs.

The browser stores only original user/enterprise/store/attempt routing metadata.
Callback `state` and `tempToken` stay in memory and are removed from the URL.
Completion is authenticated POST after the user checks the original context.
An uncertain one-time exchange is never repeated. When the encrypted credential
has been saved, “核验原连接” performs only the official store query, including
after a page reload. A new authorization fences the old attempt. Local disconnect
removes local usability; remote authorization must be revoked in SHEIN “我的授权”.

## User path

1. Sign in and choose the enterprise. Open
   `/workbench/account/organization/resources`.
2. An administrator sees actual assigned Store counts and member period/data
   balances. Search/filter members, assign concrete Stores, allocate resources,
   or reclaim only free resources. Departed/inactive holdings remain visible;
   allocation and new grants require an active current member.
3. Data allocation uses a currently configured Commercial data-row Offer.
   Enter an amount in yuan, obtain the immutable whole-row quote, inspect its
   pricing version/expiry/remainder and confirm. This transfers existing rows;
   it does not spend cash. Missing prices close this action. No example price
   or production entitlement is silently supplied.
4. Open an assigned Store at `/workbench/stores/<store-id>`. Once the approved
   application is configured, follow official authorization as the Store's
   main account, complete the callback and activate service with one period.
   One period is 30 days. A renewal permits at most 12 periods per operation.
   Administrators spend enterprise unallocated periods; members spend their
   original allocated periods. Renewal does not grant publishing permission.
5. A successful new acquisition that saves one product result spends one data
   row. Re-reading, replaying the same operation or subsequent AI processing
   does not charge another row; a new acquisition operation is charged anew.
   Failure releases only when the native publication path is definitively fenced.

Resource observations, positions, operation receipts, Store grants, service
dates and encrypted connection attempts persist in their current owners.
Timeout/response loss keeps unknown reservations. The application checks only
original persisted proofs; it never repeats acquisition/renewal or releases on
age alone. Before dispatch, the UI saves the original bounded operation key and
payload in session storage under the user, enterprise and member/Store identity.
Reloading or returning within the same browser tab restores only that command
for verification, even if balances or Store versions have advanced. A recovery
authorization failure retains the command. Unreadable or unwritable recovery
storage closes new commands; a confirmed original success clears only its own
record. This browser record is not the resource or operation fact owner.
Reclaim/release repays enterprise resource debt first and reports net credit.
Enterprise usable totals include unallocated and free member allocations;
administrator spendable balance remains the unallocated portion.

AI points continue to use the existing enterprise balance and member UTC monthly
consumption limits. Model Token entitlement/allocation remains a separate metric.
Real provider consent, first activation against SHEIN, production operation and
product acceptance are not established by the implementation checks.
