# Ecoservices qualification-only runtime

Status: DESIGN_REVIEW_PENDING (2026-10-11). Execution: #619; primary PR: #631.

## Product authority and outcome

The user explicitly chose “暂未准备商户配置，先接入非支付业务”. The existing full architecture in `ecosystem-services-v1.md` remains frozen. This supplement permits a separately selected qualification-only composition, without changing its payment, merchant, service eligibility, pricing or recovery rules.

An enterprise member with the existing join permission can upload private qualification documents, submit/correct the original application, read the actual platform decision and accept the existing agreement after approval. The existing platform administrator can read, approve or reject applications. Service market, provider listings and my-service queries show actual records, including an empty result. No sample providers, orders or success states are installed.

Product/Figma authority: `docs/product/final-ui-ia-authority.md` and the existing ecosystem frames in file `tg48P46SSXl6TBy9lZwg63`, page `31:463` (market `431:323`/`431:525`, mine `431:833`/`431:1035`, join `431:1343`/`431:1545`). Existing pages and permissions are reused. A runtime phase notice explains what can currently be completed. This is not a new page or alternative service workflow.

Scope: explicit nonpayment runtime selection, existing qualification domain/HTTP consumers and private files, native module mounting, current UI availability, and a dedicated empty Ecoservices owner/storage installation composed after the existing unified installers.

Out of scope: merchant submission/query/resume, public payment notifications, service drafts/create/update/publish, requests/quotes/delivery/acceptance, checkout/refunds/splitting/financial administration; changing ACTIVE/merchant eligibility; fake credentials or providers; offline payment; new state machines/schema/permissions/ledgers; legacy migration; real provider calls; production deployment; business acceptance. Existing listing create/update already requires ACTIVE plus MerchantID, so premerchant drafts are also closed. Approval and agreement do not imply ACTIVE or channel qualification.

## Design basis and owners

Design Basis: Independent Architecture, bounded runtime admission supplement. The new selection changes which existing business paths are admitted, while all existing fact owners and state transitions remain unchanged.

Contract → implementation → injection → consumer: manifest `ecoservices.nonPaymentOnly=true` → current-application validation and object preparation → explicit qualification dependencies → dedicated E repository/qualification Service and HTTP Handler → existing native route descriptors and UI pages. Full mode remains the default and still requires its original real WeChat profile, credentials, fixed HTTPS callback, canonical B/M pools and original protection ports. Missing credentials never silently select nonpayment mode.

E remains the sole application, agreement, private file and operation/CAS owner. Existing live ZITADEL authorization, actor/organization scope, platform-only review permissions, deadlines, body/file limits, private immutable S3 objects and idempotency fingerprints remain unchanged. Serving uses `ecoservices_runtime` with the existing installer allowlist and no DDL, DELETE, owner membership or access to installer/admin secrets. No Commercial/Money purchase service, channel, financial recovery loop or merchant protection is constructed in this composition.

## Admission and failure behavior

- Nonpayment mode rejects any nonzero payments configuration, preventing accidental downgrading of a retained full merchant profile. It still requires the dedicated E database and genuine private immutable object storage. Payment payload keys are not needed or generated for this phase.
- A qualification constructor explicitly allows only application_submit, application_review, application_reject and agreement_accept. It rejects every other mutation before repository Apply/replay and therefore cannot create financial intents, requests or listing effects even through a direct domain caller.
- The nonpayment HTTP constructor allows existing qualification reads/mutations and application file operations. Closed merchant, request-file, financial and notification consumers return unavailable; checkout must handle the absent port without panic. Existing route authentication/permission/deadline contracts remain in force. There is no fake successful endpoint or missing-configuration fallback.
- Before mounting, the E repository must verify that no requests, financial commands, merchant intents/bindings/progress, service listings, ACTIVE applications or stored merchant identifiers exist. Qualification applications, private application files and their operation/version history may be retained. A database error or incompatible retained facts fails startup rather than stranding a payment/recovery obligation. No facts are deleted or reinterpreted. Runtime qualification mutations cannot create the excluded facts; this proof relies on the existing one serving Writer/owner boundary, not an in-process lock.
- Original application validation, immutable file receipts, scoped replay, same-key/different-payload conflict and version/CAS transaction remain unchanged. Failed requests produce no new business success state. Agreement acceptance remains its original policy version and original persisted fact; it does not activate the provider.

## Local installation and retained runtime

A separate opt-in Compose overlay consumes the existing unified module manifest read-only, installs only a new empty `ecoservices` database through `ecoservices-schema-init`, and emits a separate serving manifest. It does not rerun or alter Product/native owner installers, migrate old ecosystem trials, or overwrite the original manifest. The cluster admin secret is available only to the one-shot installer. Separate installer state, runtime config, private object root secret and object data volumes are retained; serving/object containers receive only their necessary runtime credentials. The existing database-name admission reserves `ecoservices` before new commercial installation as well.

First install refuses an existing E database or interrupted installer marker. Retained restart reuses the original E role/storage credentials and exact completed configuration, with mode admission rechecked by the application. Base/native source changes that require reassembly are not silently reconciled; explicit operator inspection is required. New named volumes and all existing unified/older trial volumes are preserved on stop/start. No `down -v`, data re-seeding or initialization of business examples.

Legacy decision: N/A. Reuse current E/qualification owners, current S3 adapter and existing owner installer; no retired service wrappers, compatibility adapters or second fact source.

## Verification and handoff

TDD for explicit config admission, rejection of nonqualification domain effects, incompatible retained-state admission and absent checkout port. Check existing full-mode contracts remain strict. Reuse existing application/permission/CAS/private-file tests and bounded PostgreSQL runtime grant checks; no new verification framework. Check Compose/shell syntax, actual empty E installation, native application construction and normal browser access to qualification and platform-review pages. Preserve existing login/org/data and retained restart. UI must show the nonpayment phase and avoid merchant/financial controls and polling; backend guards remain authoritative.

Independent review covers only this new admission/composition boundary. After readiness, one Writer implements in the existing worktree/PR; final review checks the actual increment and running artifact. Development checks/CI/runtime reads remain separate from user acceptance. Real WeChat/provider execution and user business acceptance remain NOT_RUN. Merge/Issue closure/production actions remain unauthorized.
