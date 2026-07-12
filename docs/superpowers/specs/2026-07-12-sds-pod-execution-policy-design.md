# SDS POD Execution Policy Design

## Goal

Move deterministic SDS POD execution-state policy out of root ListingKit while correcting stale failure details after a non-failure status transition.

## Problem

Root `internal/listingkit` currently owns POD status normalization, SDS and child-task status mapping, submission blocking, and readiness copy. These decisions do not require task persistence, runtime clients, or SHEIN DTOs.

The current derivation can set POD status to `succeeded` from an SDS summary while retaining a non-empty historical `FailureReason`. This leaves a contradictory state for API consumers and audit readers.

## Ownership

`internal/product/sourcing/sdspod` will own pure SDS POD execution policy. It will accept and return neutral value types only; it must not import ListingKit, SHEIN, persistence, runtime, HTTP, Temporal, or external SDK packages.

Root `internal/listingkit` will retain:

- `GenerateRequest` policy selection for whether SDS POD is disabled, required, or optional;
- mapping between `PodExecutionSummary`, `SDSSyncSummary`, `ChildTaskState`, and neutral policy values;
- timestamps, audit history, task/result mutation, standard-product snapshot synchronization, and SHEIN readiness DTO construction.

## Domain API

The domain package exposes neutral string-value structs for current execution state, SDS result facts, and child task facts. It provides:

- status normalization and defaulting;
- derivation from an active child task, an SDS result, or a terminal child task;
- conversion of SDS result states into required/optional failure states;
- submission block decisions; and
- a readiness decision with severity and message.

Root adapters convert these values to existing ListingKit strings and DTOs. Existing public JSON fields and reason text remain unchanged.

## State Semantics

- Active SDS child tasks (`pending` and `processing`) take precedence over a stale SDS result.
- Otherwise, SDS result status retains the existing precedence over terminal child-task compatibility state.
- `required` turns render failure into blocking failure; `optional` turns it into degraded failure; `disabled` remains not applicable.
- Only `failed_blocking` and `failed_degraded` retain `FailureReason` and `FallbackType`.
- `succeeded`, `pending`, `processing`, `bypassed`, and `not_applicable` clear historical failure details. This corrects the stale-success contradiction without changing failure classification.
- Submission decisions preserve current behavior: required blocks until success; optional allows success, degraded failure, and bypass; disabled never blocks.

## Tests

Domain tests cover status mapping, active-child precedence, required versus optional failure classification, submission blocking, readiness decision text, and clearing stale failure details after success.

Root tests verify the adapters preserve current POD audit history, timestamps, standard-product snapshot synchronization, and SHEIN readiness blocker/warning shape. Add a regression test for a successful SDS summary carrying an old error string.

## Non-goals

- Do not alter the request-level choice of required, optional, or disabled POD mode.
- Do not add a general-purpose state-machine dependency; the policy is SDS POD-specific and has no suitable mature open-source replacement.
- Do not change remote SDS execution, retry scheduling, persistence, audit schema, or API contracts.
