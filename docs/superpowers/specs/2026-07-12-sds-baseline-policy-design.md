# SDS Baseline Policy Design

## Goal

Move deterministic SDS baseline cache reusability and overall-status policy into `sdspod`, and make Studio batch admission agree with the existing baseline-readiness API for ready cache entries with unknown validation.

## Problem

ListingKit currently contains both the baseline-readiness API policy and the reusable-cache policy used by Studio batch task admission. They classify one cache state differently:

- `Status: ready` and `ValidationStatus: unknown` is reported as ready by the baseline-readiness API.
- The same entry is rejected as not reusable by Studio batch admission.

This makes operators see a ready baseline while task creation still fails.

## Ownership

`internal/product/sourcing/sdspod` will own deterministic classification of a neutral baseline cache snapshot: normalized cache/validation statuses, overall status, reason code/message selection, and cache reusability.

Root `internal/listingkit` will retain cache-key construction, tenant resolution, repository access, canonical payload decoding, payload health detection, remote SDS validation, login reconciliation, cache persistence, and public DTO adaptation.

## Domain API

The domain policy uses value-only types:

- `BaselineSnapshot`: cache status, cache version, payload state, validation status, validation reason code, and validation reason;
- `BaselineDecision`: overall status, cache status, validation status, reason code, reason, and reusable flag.

The root adapter converts `SDSBaselineCacheEntry` to `BaselineSnapshot` only after it has decoded the payload and classified the payload as present, invalid, or empty. The domain package never receives ListingKit cache entries or repository types.

## Semantics

- Missing, unusable, unsupported-version, missing-payload, invalid-payload, and empty-payload cache snapshots are not reusable and retain the current reason codes/messages.
- `baseline_cached` requires validation status `ready` to be reusable.
- `ready` cache status is reusable when validation is either `ready` or `unknown`; `ready` records represent a previously completed usable baseline and match the API's existing readiness classification.
- Blocked and failed validation statuses remain non-reusable and preserve their current reason details.
- The policy makes no remote calls and does not mutate cache entries.

## Tests

Domain tests cover cache status, payload state, version, validation status, reason propagation, and the `ready + unknown` compatibility rule.

Root tests prove the baseline-readiness API and `evaluateSDSBaselineReusableReadiness` produce consistent decisions for the same ready/unknown cache entry. The existing Studio batch gate test is extended to prove it accepts the matching reusable baseline.

## Non-goals

- Do not move remote SDS validation, login checks, cache storage, payload serialization, or tenant behavior.
- Do not alter `baseline_cached + unknown` behavior.
- Do not change cache schema, supported version, public baseline-readiness fields, or Studio batch task orchestration.
