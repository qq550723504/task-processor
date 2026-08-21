# Provider-Neutral AI Contracts Design

## Context

The ListingKit AI runtime already routes image and chat capabilities through
common interfaces, but those contracts currently live in
`internal/infra/clients/openai/types.go`. Gemini and GrsAI therefore depend on
an OpenAI-named package even when they do not use OpenAI wire semantics. This
couples provider adapters to one implementation package and makes future
providers harder to add or test independently.

## Goal

Create a provider-neutral AI contract package without changing model routing,
tenant identity propagation, capability governance, usage settlement, or
provider request behavior.

## Design

### Contract package

Add `internal/ai` with the shared request/response models and interfaces:

- chat messages, requests, choices, usage, and responses;
- image generate/edit requests and image responses;
- async image submit/query responses;
- `ChatCompleter` and `ImageGenerator` interfaces;
- provider-neutral async unsupported error.

`ImageRouteSelection` remains in the routing layer because it describes the
selected credential/configuration rather than a provider capability.

### Compatibility boundary

`internal/infra/clients/openai/types.go` keeps type aliases to `internal/ai`
for one migration cycle. Existing imports and public constructors continue to
compile while new provider code imports `internal/ai` directly. OpenAI keeps
OpenAI-specific configuration, credential, and HTTP client types in its own
package.

### Provider adapters

- `internal/infra/clients/openai` implements the shared contracts for
  OpenAI-compatible APIs.
- `internal/infra/clients/geminiimage` imports only `internal/ai` for shared
  image types and keeps Gemini `generateContent` translation local.
- `internal/infra/clients/grsai` imports only `internal/ai` for shared image
  types and keeps GrsAI async translation local.

### Runtime wiring

`internal/listingkit/httpapi` and capability routing consume
`internal/ai.ChatCompleter`/`internal/ai.ImageGenerator`. Provider selection
remains in the existing builder and resolver; no new provider-selection logic
is introduced in business services.

### Error and security behavior

The migration must preserve existing errors and async capability behavior.
Secondary image URL downloads continue to use the shared SSRF-safe transport;
moving contracts must not introduce a provider-specific bypass.

## Data flow

```text
tenant/user identity + capability route
              |
              v
      provider-neutral contract
              |
       provider adapter
              |
        model API request
```

## Verification

The migration is complete when:

1. OpenAI, Gemini, and GrsAI packages compile without importing each other's
   provider implementation types.
2. Existing ListingKit routing, identity, governance, and image tests pass.
3. Provider-specific request-shape tests remain green.
4. `git diff --check` is clean and no behavior change is observed in the
   relevant full Go test suites.

## Non-goals

- no LangGraph/Temporal workflow change;
- no model-selection policy change;
- no credential schema change;
- no deletion of the OpenAI compatibility aliases in this change;
- no provider feature parity work such as adding Gemini async generation.
