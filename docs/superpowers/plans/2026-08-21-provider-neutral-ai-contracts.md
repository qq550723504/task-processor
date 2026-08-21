# Provider-Neutral AI Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract shared chat/image contracts from the OpenAI implementation package into `internal/ai`, then migrate Gemini, GrsAI, and ListingKit runtime boundaries to those contracts without changing routing, identity, governance, usage settlement, provider wire behavior, or security behavior.

**Architecture:** `internal/ai` owns provider-neutral request/response structs, capability interfaces, and shared errors. `internal/infra/clients/openai` continues to own OpenAI-compatible configuration, resolver, manager, pool, and HTTP implementation, while exposing one-cycle aliases for the moved contracts. Gemini and GrsAI adapters depend only on `internal/ai` for shared image types. `internal/listingkit/httpapi` uses `internal/ai` for capability interfaces and image data crossing provider boundaries, but keeps resolver/config types in the OpenAI package until a separate credential/config extraction effort.

**Tech Stack:** Go, standard `go test`, existing `testify` tests, package-level import-boundary tests, `git diff --check`.

**Spec:** `docs/superpowers/specs/2026-08-21-provider-neutral-ai-contracts-design.md`

## Global Constraints

- Preserve the existing exported method signatures semantically, JSON tags, error values, async unsupported behavior, retry behavior, route selection, tenant/user identity propagation, usage ledger behavior, and SSRF-safe secondary-image transport.
- Keep `internal/infra/clients/openai` aliases during this migration; do not mass-edit unrelated business packages that still consume the compatibility surface.
- Do not modify the currently uncommitted PR #166 review-fix files: `internal/infra/clients/geminiimage/client.go`, `internal/infra/clients/geminiimage/client_test.go`, `internal/listingkit/httpapi/studio_product_image_usage.go`, and `internal/listingkit/studio_batch_task_link_model.go`. Stage only files belonging to this plan.
- Do not introduce LangGraph/Temporal changes, provider-selection policy changes, credential-schema changes, new provider features, or Gemini async parity.
- Use TDD for every migration slice: add/adjust a focused compile or behavior test first, make the smallest implementation change, then run the focused package test before continuing.

---

## Task 1: Add the provider-neutral contract package

**Files:** `internal/ai/contracts.go`, `internal/ai/contracts_test.go`

- [ ] Add package `ai` with the exact shared definitions currently in `internal/infra/clients/openai/types.go`: chat messages/content parts, chat request/choice/usage/response, image generate/edit request, image data/response, async submit/query responses, `ChatCompleter`, `ImageGenerator`, and `ErrAsyncImageGenerationNotSupported`.
- [ ] Preserve every field name, JSON tag, pointer/value choice, and method signature; keep `context.Context` and `time` dependencies local to this package.
- [ ] Add contract tests that assert the JSON representation of representative chat/image/async values and that a compile-time stub satisfies both interfaces.
- [ ] Run `go test ./internal/ai -count=1`.

## Task 2: Make OpenAI the compatibility/provider implementation boundary

**Files:** `internal/infra/clients/openai/types.go`, `internal/infra/clients/openai/types_contract_test.go` (new if needed), existing OpenAI implementation files that reference moved types

- [ ] Replace the moved definitions in `types.go` with aliases/imported values from `task-processor/internal/ai`; retain OpenAI-owned `ClientConfig`, `PoolConfig`, `ClientConfigResolver`, `ResolvedClientConfig`, constructors, and `ImageRouteSelection` in the OpenAI package.
- [ ] Re-export the async unsupported sentinel as an alias so `errors.Is` and direct equality remain compatible for existing callers.
- [ ] Add compatibility tests proving assignments between `ai.ImageGenerator`/`ai.ChatCompleter` and the legacy `openai.ImageGenerator`/`openai.ChatCompleter` compile, and that the legacy sentinel is the same error value.
- [ ] Run `go test ./internal/infra/clients/openai -count=1` and `go test ./internal/ai ./internal/infra/clients/openai -count=1`.

## Task 3: Migrate Gemini without an OpenAI contract dependency

**Files:** `internal/infra/clients/geminiimage/client.go`, `internal/infra/clients/geminiimage/client_test.go`, any Gemini-only helper/test files found by `rg 'clients/openai' internal/infra/clients/geminiimage`

- [ ] Change Gemini method signatures and internal response construction to `internal/ai` types only; keep Gemini configuration and `generateContent` wire structs local to `geminiimage`.
- [ ] Update tests to construct `ai.ImageGenerateRequest`/`ai.ImageEditRequest` and assert `ai.ImageResponse` values.
- [ ] Preserve the existing safe secondary URL validation/client behavior, including the explicit trusted HTTP client test override already present in the worktree.
- [ ] Add an import-boundary test or source assertion that Gemini has no import of `internal/infra/clients/openai`.
- [ ] Run `go test ./internal/infra/clients/geminiimage -count=1`.

## Task 4: Migrate GrsAI without an OpenAI contract dependency

**Files:** `internal/infra/clients/grsai/client.go`, `internal/infra/clients/grsai/client_test.go`, `internal/infra/clients/grsai/client_integration_test.go`, any additional GrsAI files found by `rg 'clients/openai' internal/infra/clients/grsai`

- [ ] Change GrsAI image request/response and async submit/query signatures to `internal/ai` types only; keep GrsAI request/poll/result payloads local.
- [ ] Update unit and integration tests to use `ai` contracts while preserving request paths, polling, download, and error assertions.
- [ ] Add an import-boundary test or source assertion that GrsAI has no import of `internal/infra/clients/openai`.
- [ ] Run `go test ./internal/infra/clients/grsai -count=1`; run integration tests only under their existing opt-in conditions.

## Task 5: Migrate ListingKit runtime capability interfaces and adapters

**Files:**

- `internal/listingkit/httpapi/ai_clients.go`
- `internal/listingkit/httpapi/ai_client_builders.go`
- `internal/listingkit/httpapi/ai_client_strict_chat.go`
- `internal/listingkit/httpapi/ai_client_strict_image.go`
- `internal/listingkit/httpapi/ai_client_image_routing.go`
- `internal/listingkit/httpapi/ai_image_generator_adapter.go`
- `internal/listingkit/httpapi/ai_client_fallback_helpers.go`
- `internal/listingkit/httpapi/runtime_support_hooks.go`
- `internal/listingkit/httpapi/runtime_support_shein.go`
- `internal/listingkit/httpapi/runtime_support_shein_adapter_helpers.go`
- `internal/listingkit/httpapi/ai_clients_test.go` and focused boundary tests

- [ ] Replace only shared contract references (`ChatCompleter`, `ImageGenerator`, image request/response/async types, `Usage`) with `internal/ai`; leave `ClientConfig`, resolver, resolved-config, and OpenAI client construction in `internal/infra/clients/openai`.
- [ ] Update adapter conversions and route-aware async assertions to use `ai.ImageAsyncQueryResponse`/`ai.ImageResponse` while retaining existing route keys and provider selection.
- [ ] Preserve all existing strict-client cache/fallback behavior and timeout enforcement.
- [ ] Update stubs and tests in the same package; add a compile-time assertion that the OpenAI, Gemini, and GrsAI implementations satisfy `ai.ImageGenerator`.
- [ ] Run `go test ./internal/listingkit/httpapi -count=1`.

## Task 6: Migrate product-image governance boundary and prevent regression

**Files:**

- `internal/productimage/openai_scene_generator.go`
- `internal/productimage/openai_image_edit_adapter.go`
- `internal/productimage/httpapi/model_provider_builder.go`
- `internal/productimage/httpapi/scene_governance_builder.go`
- `internal/productimage/httpapi/ai_capability_scene_catalog.go`
- related product-image tests currently importing OpenAI contract aliases

- [ ] Replace shared image generator/request/response references with `internal/ai`; keep OpenAI-specific resolver/config types where required by the existing manager wiring.
- [ ] Preserve the already-fixed fail-closed route validation, provider-style allowlist, timeout classification, recorder warning callback, and identity-context behavior; this task is type-boundary-only.
- [ ] Add/update governance tests proving the selected route still reaches the same provider and ledger metadata after the contract move.
- [ ] Run `go test ./internal/productimage ./internal/productimage/httpapi ./internal/app/httpapi -count=1`.

## Task 7: Full dependency and behavior verification

**Files:** `internal/ai/import_boundary_test.go` (new), existing package boundary tests only when assertions need updating

- [ ] Add a focused source/import guard that fails if `internal/infra/clients/geminiimage` or `internal/infra/clients/grsai` imports `internal/infra/clients/openai` for shared contracts.
- [ ] Use `rg` to verify no provider adapter imports another provider implementation package; document intentional OpenAI config/resolver imports in the test or plan comments.
- [ ] Run the relevant nested-module suites: `go test ./internal/aicapability ./internal/core/config ./internal/ai ./internal/infra/clients/openai ./internal/infra/clients/geminiimage ./internal/infra/clients/grsai ./internal/listingkit/httpapi ./internal/productimage ./internal/productimage/httpapi ./internal/app/httpapi ./tests -count=1`.
- [ ] Run `go test ./... -count=1` from the repository root if the nested suites are green; capture any pre-existing unrelated failures separately.
- [ ] Run `git diff --check` and inspect `git status --short`; ensure the four pre-existing PR comment-fix files remain un-staged and behavior is unchanged.
- [ ] Perform a final self-review against the approved spec and record the exact test commands/results before creating a PR.

## Completion Criteria

- [ ] `internal/ai` is the source of truth for shared chat/image contracts.
- [ ] OpenAI compatibility aliases compile existing callers.
- [ ] Gemini and GrsAI no longer import the OpenAI package for shared contracts.
- [ ] Routing, identity, governance, usage settlement, retries, async behavior, and SSRF-safe image handling are unchanged.
- [ ] Focused and full verification pass, with no unrelated work staged.
