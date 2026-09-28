# Optional ZITADEL SMS delivery (#539)

The account settings phone action uses the signed-in user's ZITADEL Auth API.
ZITADEL owns the phone, code generation, expiry and verification. The current
application can now deliver its signed notifications through the existing
Tencent Cloud SDK adapter at `POST /api/v1/identity/notifications/sms`.
It has no application retry, OTP store, new identity directory or legacy route
alias. The four accepted provider events and template mapping are unchanged.

## Private configuration

Leave `ZITADEL_SMS_CONFIG_FILE` unset to keep delivery unavailable (503).
For an authorized installation, write a UTF-8 JSON file without BOM, at most
8 KiB, outside the checkout. Use the signing key of **this instance's** HTTP
SMS provider; never copy an old instance's provider ID or signing key.

```json
{
  "SigningKey": "<this-provider-signing-key>",
  "TencentSecretID": "<secret-id>",
  "TencentSecretKey": "<secret-key>",
  "TencentAppID": "<sms-app-id>",
  "TencentSignName": "<approved-sign>",
  "TencentTemplateID": "<approved-template-id>",
  "PhoneVerificationExpiryMinutes": 5
}
```

The last field is optional: omit it or use 0 for a code-only template. A value
from 1 to 60 adds template text only when `user.human.phone.code.added` omits
expiry. It **must match the actual ZITADEL phone-code expiry**; the example does
not configure expiry in ZITADEL. Other supported events use provider-supplied
expiry. Check template parameters before activating the provider.

Use an absolute path and a private regular file: mode 0600 on Linux, or a
Windows ACL allowing only the owner, SYSTEM and Administrators. Symlinks,
relative paths, public files, missing fields, unknown fields and trailing JSON
are rejected at startup, with no credentials or path in the error.

For a directly started app, set `ZITADEL_SMS_CONFIG_FILE` to that file and use
the existing command:

```powershell
go run ./cmd/current-application -config <absolute-private-runtime-manifest>
```

For Account Compose, set `ZITADEL_SMS_CONFIG_HOST_FILE` to the host file and
add `-f deployments/docker/account-compose/docker-compose.sms.yml` after the
normal Compose file, retaining the same authorized project and ports. This
overlay only mounts configuration; it neither creates nor activates a provider.
The mounted file must retain private permissions inside the Linux container.
If Docker Desktop does not preserve them, use the concrete named-volume workflow
below instead of the bind overlay; do not weaken permission validation.

### Docker Desktop private-volume workflow

These commands copy the already-private host file into a **fresh**, project-specific
volume and set Linux permissions. They do not start the app, configure ZITADEL,
or send SMS. Use the same explicitly authorized isolated `COMPOSE_PROJECT_NAME`
and host file as above. The runtime image currently runs as root, so the file is
owned by root and mode 0600; the application mounts the volume read-only.

```powershell
$smsProject = $env:COMPOSE_PROJECT_NAME
if ($smsProject -notmatch '^[a-z0-9][a-z0-9_-]*$') {
  throw 'Set the authorized isolated COMPOSE_PROJECT_NAME first'
}
$smsSource = (Get-Item -LiteralPath $env:ZITADEL_SMS_CONFIG_HOST_FILE -ErrorAction Stop)
if ($smsSource.PSIsContainer -or ($smsSource.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
  throw 'SMS source must be a private regular file'
}
$smsVolume = "${smsProject}-sms-secrets"
$smsHelper = "${smsProject}-sms-copy"
$smsVolumes = docker volume ls --format '{{.Name}}'
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Docker volumes' }
$smsContainers = docker ps -a --format '{{.Names}}'
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Docker containers' }
if ($smsVolumes -contains $smsVolume -or $smsContainers -contains $smsHelper) {
  throw 'Name already exists; preserve it and inspect ownership, do not overwrite'
}
docker volume create $smsVolume | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot create private volume' }
docker create --name $smsHelper --network none `
  --mount "type=volume,src=$smsVolume,dst=/private/notifications" `
  alpine:3.22 sh -c 'sleep 300' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot create copy container; volume retained' }
try {
  docker start $smsHelper | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Cannot start copy container' }
  docker cp $smsSource.FullName "${smsHelper}:/private/notifications/zitadel-sms.json"
  if ($LASTEXITCODE -ne 0) { throw 'Cannot copy private configuration' }
  docker exec $smsHelper sh -ec 'chmod 700 /private/notifications; chmod 600 /private/notifications/zitadel-sms.json; test "$(stat -c %a /private/notifications/zitadel-sms.json)" = 600'
  if ($LASTEXITCODE -ne 0) { throw 'Cannot establish private Linux permissions' }
} finally {
  # Remove only the container just created here. Retain the configuration volume.
  docker rm -f $smsHelper | Out-Null
}
```

Then use `docker-compose.sms-volume.yml` **instead of** `docker-compose.sms.yml`:

```powershell
docker compose -f deployments/docker/account-compose/docker-compose.yml `
  -f deployments/docker/account-compose/docker-compose.sms-volume.yml config --quiet
# Only after runtime-update authorization, retain the same project/ports and start:
docker compose -f deployments/docker/account-compose/docker-compose.yml `
  -f deployments/docker/account-compose/docker-compose.sms-volume.yml up -d current-application
```

The external volume must already exist; Compose does not silently create or
replace it. Do not run both SMS overlays, print the file, commit credentials,
delete retained volumes, or automatically overwrite a prior configuration after
a failed preparation. This procedure changes file storage only; the callback
connectivity and provider-activation prerequisites below still apply.

## Provider connection and local limitation

Use ZITADEL's [HTTP notification provider API](https://zitadel.com/docs/guides/manage/customize/notification-providers)
to create an inactive SMS provider with a reachable endpoint ending in
`/api/v1/identity/notifications/sms`. Creation returns its ID and signing key;
keep both private and place the key in the file above. Enable the provider only
after the runtime and intended recipient have been approved for real SMS.
Do not log raw callbacks, OTPs, authorization headers or full phone numbers.

**The existing isolated Account Compose cannot deliver to its loopback webhook
with the default ZITADEL egress policy.** Its Go listener is
`http://127.0.0.1:8085`, inside the shared container network namespace, but
[ZITADEL v4.17.1 defaults](https://github.com/zitadel/zitadel/blob/v4.17.1/cmd/defaults.yaml)
deny loopback and private-network destinations. The normal application HTTPS
port serves the UI and is not a direct Go webhook route. This patch does not
relax the denylist, publish the Go port or add an external tunnel. A separately
approved reachable endpoint or narrowly reviewed local egress configuration is
still required before real local phone verification. Keep that step NOT_RUN;
mounting this file alone is not proof of delivery.

Once that connection exists, an unsigned callback must return 401 and send
nothing. A valid signed callback returns 204 on SDK success; invalid payloads
return 400, oversized payloads 413, and delivery failures 502. Provider error
categories may be logged, but response bodies contain no provider details.

### Repeated delivery and production activation

This extraction preserves the existing stateless delivery contract. Two valid
callbacks carrying the same code can cause two Tencent sends and charges; the
five-minute HMAC freshness check is **not** a deduplication guarantee. This patch
does not claim at-most-once delivery. ZITADEL owns notification retry; the app
adds no retry or automatic resend. After a lost response, do not manually replay
an uncertain send.

**Production SMS activation is not authorized by this PR.** Surface this duplicate
send/cost risk in any separate rollout decision, including lost responses and
Tencent's result being unknown; no production acceptance of that risk is implied.
A local claim/cache alone cannot establish durable at-most-once delivery across
restart or reconcile provider acceptance with a lost response. If the rollout
decision requires such persistence or a recovery protocol, that change requires
its own Design Basis and is outside this wiring-only extraction. The code may be
reviewed and merged while the provider remains inactive; this is not production
SMS acceptance. Any real local trial still needs explicit scope and recipient
authorization.

After activation, use Account settings with an E.164 phone number (for example,
`+86` followed by the mainland number), request one fresh official verification
code, and enter the received code in the existing verification action. Confirm
the official phone state becomes verified. A timeout or error is not permission
to replay an earlier uncertain phone-change/send request. Code tests use fake
senders; receiving a real SMS, completing phone verification and subsequent KYC
remain separate user acceptance, not inferred from CI.
