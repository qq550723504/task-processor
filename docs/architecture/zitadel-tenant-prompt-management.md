# Prompt Management Module

## Goal

`promptmgmt` is now a reusable prompt-template management module rather than a
ListingKit settings subtype.

It owns three things:

1. prompt catalog metadata
2. tenant-scoped prompt overrides
3. HTTP management endpoints for catalog, schema, and tenant edits

It does **not** own prompt rendering. Runtime rendering still lives in
`internal/prompt`.

## Module Boundary

Backend module layout:

```text
internal/promptmgmt/
  api/
  catalog.go
  catalog_manifest.go
  service.go
  types.go
```

Responsibilities:

- `catalog_manifest.go`
  - explicit prompt metadata manifest
  - stable `label`, `description`, `scopes`, and `variables`
- `catalog.go`
  - schema assembly logic
  - derived `group`, `category`, and fallback metadata behavior
- `service.go`
  - tenant override management
  - catalog/schema lookup
  - key validation against the catalog
- `api/handler.go`
  - HTTP transport only

Frontend module layout:

```text
web/listingkit-ui/src/lib/api/prompt-management.ts
web/listingkit-ui/src/lib/query/use-prompt-management.ts
web/listingkit-ui/src/lib/types/prompt-management.ts
web/listingkit-ui/src/components/listingkit/prompts/
```

The current UI still lives inside ListingKit pages, but the prompt domain and
API contract are independent.

## Scope Model

Prompt templates are tenant scoped by the ZITADEL resource owner id. The web
proxy verifies the ZITADEL access token and forwards the resource owner id as
`tenant-id` and `X-Tenant-ID`; backend request contexts then carry that value
through `listingkit.WithTenantID`.

At the schema layer, supported scopes now come from the prompt catalog
manifest. Current prompts are all:

```text
tenant
```

`supports_tenant_override` is derived from `supported_scopes`, not hardcoded.

## Runtime Storage

Runtime tenant overrides are stored in:

```text
tenant_prompt_templates
```

Fields:

```text
tenant_id
key
content
version
enabled
created_at
updated_at
```

`tenant_id + key` is unique.

Only rows with `enabled = true` are used by runtime tenant prompt resolution.
Disabled rows are treated as unavailable.

## Catalog Contract

The catalog is explicit. A managed prompt key must exist in the prompt catalog.

The catalog schema currently exposes:

```text
key
label
description
group
group_label
category
category_label
supported_scopes
variables
has_default_content
supports_tenant_override
```

Important design rules:

- `label` and `description` come from the manifest, not only from key
  humanization
- `variables` come from the manifest first, then fall back to prompt-content
  extraction only when the manifest does not define them
- `supported_scopes` come from the manifest
- tenant overrides are only valid for catalog keys

## API Contract

Prompt management routes are independent from settings routes.

Catalog and schema:

```text
GET /api/v1/listing-kits/prompts/catalog
GET /api/v1/listing-kits/prompts/schema/:key
```

Tenant override management:

```text
GET   /api/v1/listing-kits/prompts
PUT   /api/v1/listing-kits/prompts
PATCH /api/v1/listing-kits/prompts/:key/status
```

Behavior:

- `GET /prompts/catalog`
  - returns all managed prompt schemas
- `GET /prompts/schema/:key`
  - returns one schema
  - `404` when the key is not in the catalog
- `GET /prompts`
  - returns current-tenant override rows
- `PUT /prompts`
  - saves a tenant override
  - `404` when `key` is not in the catalog
- `PATCH /prompts/:key/status`
  - toggles the tenant override enabled state
  - `404` when the tenant row does not exist

## Validation Rules

Current service-level rules:

1. prompt store must be configured
2. tenant ids are normalized
3. keys are trimmed
4. writes are allowed only for catalog keys
5. unknown keys are rejected before touching the store

This closes the old gap where UI restrictions could be bypassed by writing
arbitrary prompt keys through the API.

## Runtime Resolution Rule

Tenant-aware prompt resolution does not fall back to global prompt files or
code literals once tenant lookup is chosen.

Runtime call sites should use:

```go
prompt.GetTenantFromContext(ctx, key)
prompt.RenderTenantFromContext(ctx, key, vars)
```

When the database store is attached, database lookup is authoritative for
tenant prompts. Missing or disabled tenant rows are treated as unavailable.

## UI Contract

The current prompt management page is catalog-driven:

- left panel shows catalog entries
- right panel edits a selected catalog template only
- free-form prompt keys are not allowed
- overrides are filtered by:
  - group
  - coverage state
  - default-content presence
  - variable presence

Coverage state is three-valued:

```text
default
overridden_enabled
overridden_disabled
```

This is important because “has override” and “override is active” are not the
same state.

## Manifest Rule

When adding or changing a managed prompt, update these layers together:

1. `internal/prompt/keys.go`
2. `prompts/...` default template file if runtime default content exists
3. `internal/promptmgmt/catalog_manifest.go`

The manifest should be treated as the source of truth for management metadata.

## Review Checklist

Before extending prompt management, check:

1. Is the prompt supposed to be managed at all, or is it internal-only?
2. Does the key exist in `internal/prompt/keys.go`?
3. Does the catalog manifest define:
   - label
   - description
   - scopes
   - variables
4. Should the prompt appear under the right `group` and `category`?
5. Does the UI need new filtering or glossary support?

## Current Limitation

The UI shell is still ListingKit-specific. The prompt domain, service, API, and
types are reusable now, but the page container itself has not been extracted
into a standalone shared shell.
