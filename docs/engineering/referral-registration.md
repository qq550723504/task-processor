# Personal referral registration: Slice A

## Authority and ownership

This slice implements the domain, registration application, PostgreSQL adapter,
create-only ZITADEL adapter and explicit schema command from the
[frozen #413 contract](https://github.com/qq550723504/task-processor/issues/413#issuecomment-5644467491).
The [independent architecture admission](https://github.com/qq550723504/task-processor/issues/413#issuecomment-5647168600)
does not authorize serving routes, deployment or real identity-provider mutations.
The [three-file operational registration window](https://github.com/qq550723504/task-processor/issues/413#issuecomment-5650271025)
adds the maintained command registration and script without changing repository guards.

`internal/referral` owns immutable personal attribution and its projection.
`internal/app/referralregistration` coordinates registration through local ports.
`internal/integration/persistence/referral` owns the five-table transaction boundary.
`internal/integration/zitadelregistration` implements only the pinned user create and
fixed-ID read APIs. Existing `authidentity` supplies verified current identity;
organization grants are not personal referral ownership.

This is a greenfield schema. There is no legacy migration, account adoption by email,
organization impersonation, historical binding import, compatibility adapter or second
attribution source. Existing accounts cannot acquire a referral by ordinary login.
Earnings remain `unavailable`, with no amount field.

## Application calls and recovery

- `Start` validates input and trusted client IP, applies the shared PostgreSQL IP
  admission limit, and commits an immutable Intent before any provider mutation.
  The client key is a stable 256-bit hexadecimal value. The Intent ID and recovery
  secret each contain 256 random bits; the server fixes one UUIDv7 subject.
- The same client key and canonical payload recover the original encrypted admission
  receipt, including its original recovery secret, after a lost response or restart.
  A changed payload conflicts. An email fingerprint without the original key cannot
  claim a pending request. Identity and email uniqueness survive expiry and UNKNOWN.
- `Resume` requires the original Intent ID and recovery secret. It reads only the
  fixed subject. A confirmed 404 within the original create window permits one
  create-only attempt using the same subject and proof. Every dispatched attempt is
  followed by fixed-ID readback. Missing/changed proof, failed reads and ambiguous
  mutation responses remain UNKNOWN; they never release identity or adopt an account.
- `Complete` derives the subject from the verified current identity and checks its
  token expiry. It first returns an existing successful receipt. For an unconsumed
  Intent it checks the original subject, signup organization, original email, live
  verification and HMAC proof. It never creates a provider user.
- Relation insertion, successful receipt, CONSUMED state and encrypted payload erasure
  share one PostgreSQL transaction. Same attribution replays; different attribution
  conflicts. After locking, the database clock participates in the expiry decision.
  A lost COMMIT acknowledgement returns UNKNOWN; the next call reads the durable result.
- `ReadSelf` and `CreateSelfCode` derive the current person without requiring an
  organization. Reading never creates a code or relation. Code creation is explicit.

The two deadlines are fixed at initial admission: create expires after 15 minutes and
completion after 24 hours. Retries do not slide either deadline. After the create
deadline, recovery may read back but cannot create. An expired unconsumed Intent
cannot bind. A successful receipt remains replayable after expiry or later provider,
email or proof changes, provided current authentication remains valid.

## Secrets and resource bounds

Application configuration supplies separate 32-byte encryption, proof and lookup
keys. AES-GCM protects the minimum recovery payload and original admission receipt;
the Intent issuer/ID is authenticated as associated data. The recovery secret is
checked against a keyed digest. HMAC proof binds the key ID, issuer, instance,
signup organization, Intent, subject, referrer and canonical fingerprint. Provider
metadata is only its carrier. Retain the encryption/proof keys for unfinished
Intents during rotation; a missing key fails closed.

Registration commands have a 15-second total deadline, provider calls at most five
seconds, database operations at most three seconds and eight in-flight application
commands per service instance. A full gate rejects instead of queuing. The fixed
subject and PostgreSQL lease coordinate instances; provider create-only uniqueness
is still authoritative if an earlier response is delayed.

The IP limit is five attempts per UTC minute, atomically counted in PostgreSQL across
instances. `Request.ClientIP` is excluded from JSON and must be supplied by the trusted
proxy boundary in Slice B; it is never accepted from an arbitrary browser header.
The immutable email fingerprint uniqueness allows at most one Intent for that email,
including throughout its initial 15-minute window. A finished or ambiguous Intent
does not automatically become a new identity.

Code is at most 200 bytes. Profile names are required valid UTF-8, at most 120 bytes
each. Email is further limited to the pinned provider's 200-byte ceiling, validated
before admission, and lowercased without provider-specific alias rewriting. Provider
requests are bounded to 4 KiB and responses to 64 KiB. The adapter requires an explicit
HTTPS origin, a server-side credential callback and no redirects; it does not forward
a current user's token. It never includes raw provider responses or credentials in errors.

Start, authenticated recovery and unconsumed completion invoke bounded expiry cleanup:
at most 20 expired encrypted payloads, within 100 ms per invocation. Admission-bucket
cleanup has its own same-size/time budget. No scheduler runs when there are no commands,
so this does not promise physical deletion exactly at expiry. Deduplication and durable
attribution keys remain; relations and receipts cannot be updated or deleted by runtime.

## Explicit installation and runtime privileges

The maintained entrypoint is `scripts/referral-schema-init.ps1`, which invokes
`cmd/referral-schema-init` through the application composition entrypoint. It takes a
mandatory DSN file and an optional 1–60 second timeout. It resolves the file before
changing directory and passes the path as one argument, including paths with spaces.
The Go command reads at most 64 KiB and does not print the DSN or raw connection errors.
No DSN environment fallback or second target is attempted. No serving process starts.

The file must contain one explicit PostgreSQL URL with username, password, host,
port, database and `sslmode` (`disable`, `require` or `verify-full`). The installation
transaction creates the current five tables and fails if they already exist. It
does not migrate or repair an existing referral schema. Protect the file using the
operator's normal secret-file controls, and remove task-owned secret files after use.

Example syntax, only after the concrete target and operation have been authorized:

```powershell
./scripts/referral-schema-init.ps1 -DsnFile '<private DSN file>' -TimeoutSeconds 30
```

An explicit DSN, localhost address or test-like database name is not authorization
to operate on a shared or real database. Slice A verification creates its own
PostgreSQL testcontainer, records that allocation, connects only to its returned
target and terminates that container in test cleanup. It does not start the official
application, IAM, mail, Dub or any shared Compose environment.

Use a distinct non-owner runtime role. Provision its grants through the authorized
database owner, separately from serving. The effective privilege contract is:

| Object | Runtime grants |
| --- | --- |
| The five tables | SELECT, INSERT |
| `registration_intents` | UPDATE only `state`, `ciphertext`, `lease_until` |
| `registration_admission_buckets` | UPDATE, DELETE |
| Database / public schema | CONNECT / USAGE; no CREATE or TEMP |

No superuser, CREATEDB, CREATEROLE, BYPASSRLS, ownership membership, other table/column
UPDATE, DELETE, TRUNCATE, REFERENCES or TRIGGER privileges are permitted.
`persistence.New` verifies effective grants, including inherited and column grants,
and refuses an excessive or incomplete role. The schema installer does not silently
elevate the runtime role. Slice B must supply a dedicated bounded pool and close it on
failed assembly/shutdown, preserving the other identity pools and factories.

## Pinned provider boundary

Wire behavior is pinned to ZITADEL v4.17.1 commit
`a9311b8c702531832575351a663e98a2242778e5`:

- [User service](https://github.com/zitadel/zitadel/blob/a9311b8c702531832575351a663e98a2242778e5/proto/zitadel/user/v2/user_service.proto):
  POST `/v2/users/human`, GET `/v2/users/{id}` and POST `/v2/users/{id}/metadata/search`.
- [User/profile and metadata schema](https://github.com/zitadel/zitadel/blob/a9311b8c702531832575351a663e98a2242778e5/proto/zitadel/user/v2/user.proto):
  required given/family names, fixed user ID and base64 metadata values.
- [Email schema](https://github.com/zitadel/zitadel/blob/a9311b8c702531832575351a663e98a2242778e5/proto/zitadel/user/v2/email.proto):
  a maximum of 200 characters and `sendCode` using the configured official default
  verification URL. The application does not request verification codes or set verified.

There is no password, OTP, session or authenticator implementation here. Slice B/C
must verify the actual official verification URL, sender/template, service-account
permissions, available primary authenticator and generic OIDC/Auth.js return chain.
Controlled HTTP fixtures cannot establish that real-provider happy path.

## Verification commands and evidence boundaries

```text
go test ./internal/referral ./internal/app/referralregistration ./internal/integration/zitadelregistration ./cmd/referral-schema-init -count=1
go test -tags integration ./internal/integration/persistence/referral ./internal/integration/zitadelregistration -count=1
golangci-lint run ./internal/referral/... ./internal/app/referralregistration/... ./internal/integration/persistence/referral/... ./internal/integration/zitadelregistration/... ./cmd/referral-schema-init/...
go test ./tests -count=1
```

The PostgreSQL suite exercises cross-instance quotas, concurrent fixed identity and
single consumption, forbidden runtime statements and privilege drift, transaction
rollback, lock-delayed expiry, bounded cleanup, and loss of the actual server COMMIT
acknowledgement after durable admission/consumption. The combined owned-PG/controlled-
HTTPS fixture separately exercises lost application receipts, provider create-response
loss, delayed metadata, restart and successful replay during provider outage.

Record final SHA, CI and independent review in the PR. Real official verification →
authenticator setup → generic OIDC/Auth.js → same-subject completion remains NOT_RUN
until separately authorized B/C verification. This slice exposes no HTTP route or UI,
and its isolated tests are not production acceptance, deployment or main integration.
