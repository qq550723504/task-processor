# 1688 browser capture

A Manifest V3 extension for an explicit capture of the current 1688 product
detail page. Chrome and Edge use the same build. This directory owns capture
and ephemeral transport; it cannot publish a Product or authorize an import.

Contract: [SRC-2B2 R1](https://github.com/qq550723504/task-processor/issues/399#issuecomment-5642868657),
[recovery clarification](https://github.com/qq550723504/task-processor/issues/399#issuecomment-5642876140),
and the [src2b-acquisition-v1 owner contract](https://github.com/qq550723504/task-processor/issues/398#issuecomment-5637525452).

## Build and local checks

Requires Node 24 and npm. Install with `npm ci`, then run `npm test`,
`npm run typecheck`, and `npm run lint`. Tests include the built manifest's
permission and destination boundary.

A release build requires an explicit application URL. There is no default
production destination. For example, in PowerShell, replace the example host
with the current application's approved HTTPS origin:

```powershell
$env:CAPTURE_APP_URL = 'https://app.example.com/capture/1688'
npm run build
```

Load the resulting `dist` directory as an unpacked extension. The application
at `/capture/1688` must implement the receiving contract below. A build alone
does not establish that the receiving application is available.

Task fixtures use a separate, visibly labelled artifact:

```powershell
$env:CAPTURE_APP_URL = 'http://127.0.0.1:4399/capture/1688'
npm run build -- --fixture
node scripts/browser-smoke.mjs --cdp
node scripts/browser-smoke.mjs --cdp --edge
```

The Windows harness starts an installed Chrome/Edge in its own new profile
under `artifacts`. It uses the official CDP `Extensions.loadUnpacked` and
`Extensions.triggerAction` testing methods, with the test-only extension
debugging flag. It opens the actual action popup, clicks capture and handoff,
checks the other-tab rejection, interrupts the task's workers, and reloads the
receiver with the same recovery key. The flag is never part of the extension
or a user's regular browser configuration. Ports 4398 and 4399 must be free;
the harness fails instead of taking over another process.

The canonical 1688 document response is substituted with `tests/fixtures/product.html`;
other page network requests are blocked except the task's loopback receiver.
The receiver is explicitly a **transport fixture**, with no account, BFF,
database or publication code. It never proves backend acceptance or real
1688 availability. Browser receipts/screenshots stay under ignored `artifacts`.
Browsers and the loopback server close in `finally`; the task owner can remove
only its closed disposable profiles after preserving the receipts.

## Capture boundary

- Permissions are exactly `activeTab` and `scripting`. No host, cookies,
  storage, tabs, history, webRequest, clipboard or downloads permissions.
- The popup's explicit capture action selects only the active tab. Injection
  targets its main frame in the isolated world. The same document ID and
  canonical source are checked again after extraction.
- Accepted pages have the exact `detail.1688.com/offer/<ID>.html` shape. IDs
  remain decimal strings, 1–20 digits without leading zero. Tracking query
  and fragment are discarded. The backend independently derives identity.
- Only the known static `context.result` product structure and the identified
  rendered description node are read. No page scripts execute on behalf of
  the extension; no global objects, forms, cookies, storage or other tabs are
  inspected. Unknown structures and challenge/login pages fail explicitly.
- Product fields are projected onto the existing v1 wire: no raw HTML,
  arbitrary metadata, actor, organization, role, Catalog identity or trusted
  asset status. Image URLs are candidates only, never downloaded by capture.
- Missing data stays null/empty with warnings. Amounts and SKU identifiers
  preserve source string precision. Images with query strings, credentials,
  non-HTTPS or private/local literal hosts are rejected.
- The static block and encoded payload are bounded to 2 MiB; a field to
  8 KiB; a collection to 256 items; aggregate fact items to 1024. Extraction
  also has bounded node/depth work and an 8-second deadline.

`tests/fixtures/capture-v1.json` is a deterministic lower-case wire golden
for the controlled product fixture at a fixed timestamp. Its content hash
covers the ordered public field projection only: sourceURL, offerID, title,
description, attributes, variants, priceFacts, images. This is a **capture
claim**, not an authenticity signature or the backend's canonical intent
digest. The server owns validation and the sole evidence-to-SourceEnvelope
mapper; SRC-1 and Catalog own durable publication and exact read-back.

## Application handoff contract

The user sees a summary, then explicitly chooses to open the current app.
The extension freezes one capture and UUID key, creates only its own app tab,
and includes `extensionId`, `handoffId`, `idempotencyKey` in the fragment.
No payload, credentials or identity claims are put in the URL.

The app uses external `chrome.runtime.sendMessage` with strict messages:

```typescript
// Request. All identifiers must match this handoff.
{ version: 1, type: "capture.read", handoffId, idempotencyKey }
// Response.
{ version: 1, type: "capture.payload", handoffId, idempotencyKey, payload }
// Optional status projection from the app after its real backend operation.
{ version: 1, type: "capture.status", handoffId, idempotencyKey,
  outcome: "processing" | "published" | "failed" | "outcome_unknown",
  operationId? }
```

Every message checks the browser-provided exact origin, URL scheme/host/port/
path, top-level frame and own-created tab ID. Other extensions are not
admitted. All fields are required except the optional operation ID; a
published status requires that ID. A late processing message cannot erase
unknown, and a different receipt ID cannot replace the original. Status is
only an application projection, never publication authority.

The app can retry an early `capture.read` at most three times, 250 ms apart,
while `tabs.create` finishes. Unbound tabs fail closed. There are no automatic
write retries. Before any POST, the app must replace the initial fragment
with `#operationKey=<original UUID>`, show the currently verified user and
Effective Organization, and obtain explicit confirmation using the existing
BFF context-drift protections. Login, organization change, revocation and
authorization checks belong to that app/backend owner, not this extension.

The planned browser ingress is
`/api/v1/workbench/sourcing/1688/browser-captures`, with `/verify`,
`/by-key/:key` and `/:operation_id`. **These endpoints and the Web receiver
are delivered by the #398 owner, not this directory.** Public `{source}`
acquisition is not a substitute for uploading BrowserCapturePayload.

One handoff lives only in worker memory, for at most five minutes. Chrome may
terminate an idle worker much earlier. There is no keepalive or storage;
after loss/expiry, capture reads fail. If a request may already have been
submitted, the app's original key recovery URL remains the recovery entry.
The backend must reauthorize that scope and read the original staging command
through SRC-1 Verify/Read, without creating, fetching, claiming or publishing.
A missing receipt does not authorize an automatic new POST/key. The extension
cannot recover a forgotten key after restart; it tells the user to use the
original app page. Beginning another capture requires an explicit new action.

## Verification still required for delivery

Extension unit tests and the browser transport fixture are separate from the
real application chain. Final acceptance requires the actual Web/BFF → browser
validator → existing mapper → SRC-1 → Catalog flow and exact read-back, including
scope changes, revocation, response loss and by-key read-only recovery. A fixture
success cannot satisfy these requirements. Real 1688 login/network acceptance
needs separate authorization; do not use real accounts for this harness.

## Legacy decision

Legacy decision: **EXTRACT**. Reusable behavior is the known 1688
productTitle/gallery/Root SKU/price field interpretation. The current owner
here is the untrusted browser capture adapter. No old services, DTOs, profile,
queues, fallback or handoff are imported or kept alive. Existing legacy
consumers are outside this slice and follow their own retirement owner.
