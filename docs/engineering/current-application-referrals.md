# Current application: personal referrals server slice

Refs #413, frozen proposal comment 5644467491. This slice mounts the existing
referral registration application in RUN-1. It does not deliver Browser/BFF,
official mail/authenticator/OIDC acceptance, deployment, or production enablement.

The manifest omits `referrals` or sets `{"referrals":{"enabled":false}}` by
default. Disabled means no additional routes, provider client, credential reads,
or referral database pool. Existing source, commercial and account audit owners
retain their independent composition and permissions. Optional composition uses
`NewCurrentApplicationWithOptions(..., WithReferrals(pool))`; the existing typed
`NewCurrentApplication` entry point remains available without the option.

## Five exact method/path contracts

| Method | Go path | Authority and effect |
| --- | --- | --- |
| POST | `/api/v1/referral-registration/intents` | Private BFF service credential; durable admission only |
| POST | `/api/v1/referral-registration/resume` | Same private scope; original Intent ID and resume secret |
| GET | `/api/v1/account/referrals` | CurrentIdentity, OrganizationAccessPolicyNone; read only |
| POST | `/api/v1/account/referrals` | CurrentIdentity; explicitly create own public code |
| POST | `/api/v1/account/referrals/complete` | CurrentIdentity; complete own durable Intent |

Pre-auth commands require exactly one `X-Referral-Service-Credential` containing
the configured 256-bit hexadecimal value. Authorization/Bearer is rejected on
these commands. SHA-256 plus constant-time comparison avoids secret-dependent
comparison timing. The credential confers neither user identity nor self access.
The current listener is IPv4 loopback; these commands also require a loopback peer.

`X-Referral-Client-IP` is an explicit assertion from that authenticated, local BFF.
It must be one IP address. Public `Forwarded` and `X-Forwarded-For` are rejected;
Gin ClientIP is not used. This proves only the controlled Go caller boundary.
The real proxy → BFF → Go extraction, same-origin/CSRF checks and fixed upstream
target are C acceptance work, not proven by this server fixture. IP is a resource
limit key, never an attribution or user-identity proof.

Admission takes `Idempotency-Key` in exactly one header and JSON string fields
`code`, `email`, `givenName`, `familyName`. Resume takes only `intentID` and
`resumeSecret`. Unknown, duplicate, case-variant, null and non-string fields are
rejected. The application validates lengths and durable request fingerprints.
Subject, issuer, organization, verified state, proof and redirect cannot be chosen
in these bodies. Resume secrets are response/body data, never query parameters.
Admission responses omit the internal subject and use UTC absolute deadlines.

Self routes reject query selectors and request bodies. They use the existing
verified current subject and require no enterprise membership or live grant.
An administrator has no impersonation route. GET creates no code, Intent,
relation, admission bucket or receipt. A missing code is `not_created`; a
successful count of zero is zero, while storage failure is an error. Earnings
remain `{"availability":"unavailable","amount":null}`. No synthetic earnings,
second count ledger, Dub, subscription mutation or funds operation exists.

HTTP errors preserve application outcomes: invalid 400, authentication 401,
missing 404, conflict/pending 409, expired 410, capacity 429, unknown/unavailable
503. Internal error details and provider bodies are not returned. Successful
completion returns only status, Intent reference and UTC bound time. A durable
successful receipt remains replayable when later provider reads fail.

## Private manifest and installation

Enabled `referrals` fields:

| Field | Requirement |
| --- | --- |
| `issuer` | Exact existing `identity.issuerURL` |
| `instanceID`, `signupOrganizationID` | Fixed bounded provider ownership coordinates |
| `providerOrigin`, `officialLoginOrigin`, `publicAppOrigin` | Fixed HTTPS origins without query, fragment or arbitrary path |
| `credentialFile` | Private file containing limited provider machine token |
| `serviceCredentialFile` | Separate private file containing random 32-byte hexadecimal credential |
| `lookupKeyFile` | Separate private file containing random 32-byte hexadecimal key |
| `keyID` | Active proof/encryption key ID |
| `proofKeyFiles`, `encryptionKeyFiles` | ID → private absolute filename maps; retain keys for pending Intents |
| `providerCAFile` | Optional private PEM trust addition for the fixed provider; never disables TLS verification |
| `referralDatabase` | Existing database coordinates, `referral_runtime`, explicit loopback port and bounded pool |

`referralDatabase` has the same `host`, `port`, `user`, `password`, `database`,
`maxConnections` shape as the existing database entries. The role and pool are
independent from source/commercial; no owner/superuser serving connection is
allowed. Configuration and secrets have no environment overrides. Never log or
publish the manifest, prepared configuration or private files.

The manifest and referenced private files must be regular absolute paths. Unix group/other permission bits
are rejected. LoadConfig also checks Windows manifest ACL before reading its
database passwords. Windows uses the native .NET ACL API through a fixed, noninteractive
Windows PowerShell command with JSON stdin paths; allow ACEs may name only the
current account, SYSTEM or Administrators. Inherited public/group access fails
closed. The ACL check has a three-second budget inside the startup deadline;
missing/failed ACL tooling is a startup error. No ACL is modified by serving.

Use the explicit greenfield installer documented in
[referral-registration.md](referral-registration.md) on a newly provisioned,
authorized database. All five tables, constraints and both expiry indexes belong
to that installer. The serving path only verifies existing permissions/schema;
it does not install, migrate, repair or grant anything. Runtime needs SELECT and
INSERT on the five referral tables, UPDATE only on Intent state/ciphertext/lease,
and UPDATE/DELETE on admission buckets. Relation/receipt UPDATE/DELETE and extra
privileges fail closed. Existing shared/production databases are not test inputs.

Key rotation must retain the original proof and encryption IDs for every live
Intent; UNKNOWN never means permission to issue a new subject or key identity.
The lookup key is stable for durable deduplication. Provider, BFF, proof, AEAD and
lookup material must differ. At most eight retained keys per map are admitted.

## Resource and lifecycle boundaries

Requests have a 15-second deadline, 4 KiB JSON bound and 16 KiB response bound.
Eight concurrent admitted handlers share one bounded gate; excess work receives
429 without a queue. A ResponseController socket read deadline interrupts
incomplete HTTP/1.1 bodies at the request budget; Body.Close alone can block
behind an active Read. Both pre-auth paths have real TCP coverage with eight
stalled bodies and a ninth capacity probe. B-local recovery contains command
panics (including broken-pipe errors), returns only a safe UNKNOWN result, and
does not log panic values or request dumps. The service header is consumed and
removed after reading it. The
existing application additionally bounds its own mutation execution, provider
calls to five seconds/64 KiB and DB operations to three seconds. Its shared PG
admission imposes five admissions per IP/minute and durable email deduplication.
No separate rate-limit database or scheduler is introduced.

The existing application owns fixed deadlines, DB-clock create permission,
leases, proof checks, transactions, UNKNOWN recovery and bounded cleanup.
The HTTP layer does not recreate those decisions. Cleanup remains at most 20 rows
and 100 ms per eligible command; idle time does not imply physical erasure.

Runtime owns the optional pool and HTTP transport. Invalid private configuration
fails before opening databases. Partial startup, constructor/listen failure and
cancellation close opened pools in reverse order. Shutdown drains serving before
pool close and releases idle provider connections. Restart reads the original
durable admission/receipt instead of reconstructing identity from browser state.

## Verification and limits

`go test ./internal/app/runtime/currentapplication ./internal/app/httpapi`
exercises configuration, routing and authority boundaries. The isolated test:

```text
go test -race -tags integration ./internal/app/httpapi -run TestReferralRegistrationNormalBinaryPostgres -count=1 -v
```

builds the normal binary, creates its own loopback PostgreSQL 17 container,
installs schema using the owner only in fixture setup, grants minimal runtime
roles and drives actual HTTP calls. It covers read-only GET, explicit code,
same-key admission replay/conflict, process restart, lost provider response,
pending/verified completion, immutable receipt replay and runtime rejection.
Every run checks listener/process exit and database-session release.

This controlled provider does not send email or provision real identities. The
official verification/authenticator handoff, actual provider grants, Auth.js,
real BFF/proxy, browser states and production acceptance remain NOT_RUN here.
