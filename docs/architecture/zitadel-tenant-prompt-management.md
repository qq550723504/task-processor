# ZITADEL Tenant Prompt Management

## Scope

Prompt templates are tenant scoped by the ZITADEL resource owner id. The web proxy verifies the ZITADEL access token and forwards the resource owner id as `tenant-id` and `X-Tenant-ID`; backend request contexts then carry that value through `listingkit.WithTenantID`.

## Database Storage

Runtime tenant prompts are stored in `tenant_prompt_templates`.

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

`tenant_id + key` is unique. Only rows with `enabled = true` are used by runtime prompt resolution. Disabled rows are treated as missing and do not fall back.

The HTTP API runtime auto-migrates the table when database config is available and attaches the GORM store to the global prompt registry.

## File Layout

Tenant prompts live under:

```text
prompts/tenants/<zitadel-resource-owner-id>/<domain>/<file>.yaml
```

Example:

```text
prompts/tenants/286/shein/content_optimizer.yaml
```

The YAML shape is the same as the existing prompt files. A file at the path above containing:

```yaml
content_optimizer:
  optimize_title_description_system:
    content: "..."
```

is resolved as:

```text
tenant=286
key=shein.content_optimizer.optimize_title_description_system
```

## No Fallback Rule

Tenant-aware prompt resolution does not fall back to global prompt files or code literals.

Missing tenant prompt:

```text
prompt "<key>" not configured for tenant "<tenant-id>"
```

Render error:

```text
returns the template render error and no prompt text
```

When the database store is attached, database lookup is authoritative for tenant prompts. A missing or disabled database row is an error; file prompts are not used as a fallback for that tenant lookup.

Legacy `Get` and `Render` also no longer return the fallback argument on misses. `Get` keeps its old signature, so it returns an empty string on misses; `Render` returns an error.

## Runtime Use

New tenant-aware call sites should use:

```go
prompt.GetTenantFromContext(ctx, key)
prompt.RenderTenantFromContext(ctx, key, vars)
```

These helpers read the tenant id from the request or task context, which is populated from ZITADEL by the API layer.

## UI Management Readiness

The backend store already supports the operations a management UI needs:

```go
GetEnabled(ctx, tenantID, key)
ListTenant(ctx, tenantID)
Upsert(ctx, TenantPromptTemplate{...})
SetEnabled(ctx, tenantID, key, enabled)
```

A UI should list templates for the current ZITADEL tenant only, save edits through `Upsert`, and disable templates through `SetEnabled(false)`. Runtime behavior will immediately treat disabled templates as unavailable.
