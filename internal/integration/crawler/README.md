# Crawler Integrations

Owns source-specific crawling adapters that feed product sourcing pipelines.

Current approved children:

- `internal/integration/crawler/amazon`
- `internal/integration/crawler/a1688`

Use this area for:

- browser or fetch runtime concerns
- anti-bot and source-specific access adaptation
- raw extraction contracts

Do not use this area for:

- marketplace publishing rules
- normalized product facts
- listing-task orchestration

Normalized handoff belongs in `internal/product/sourcing`.

Guardrail:

- crawler integrations must not import `internal/listingkit`, marketplace/workspace/publishing packages, or `internal/product/sourcing`; source identity and normalized result alignment flow toward product sourcing from outside these adapters.
