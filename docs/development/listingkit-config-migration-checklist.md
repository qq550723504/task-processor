# ListingKit Config Migration Checklist

This checklist is for moving a deployment from scattered `LISTINGKIT_*` /
`ZITADEL_*` runtime assumptions to the unified core config path.

## Target State

- The Go API builds ListingKit auth/runtime settings from one config object.
- `config/*.yaml` carries the structural shape for `listingkit.*` and
  `listingkit.zitadel.*`.
- Runtime env only supplies secrets or deployment-specific overrides.
- The Next.js UI still reads browser-side OIDC env such as `ZITADEL_ISSUER_URL`
  directly.

## Keys To Review

Core ListingKit keys now live under:

```yaml
listingkit:
  sheinSubmitDebugDumpDir: ""
  platformAdminUsers: []
  platformAdminRoles: []
  ownerScopeRequired: false
  zitadel:
    issuerURL: ""
    clientID: ""
    clientSecret: ""
    authRequired: false
    authorizationRequired: false
    allowedTenantIDs: []
    allowedUserIDs: []
    allowedUsernames: []
    allowedRoles: []
```

Relevant env bindings:

- `LISTINGKIT_DEBUG_SUBMIT_DUMP_DIR`
- `LISTINGKIT_PLATFORM_ADMIN_USERS`
- `LISTINGKIT_PLATFORM_ADMIN_ROLES`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_OWNER_SCOPE_REQUIRED`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED`
- `LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS`
- `LISTINGKIT_ZITADEL_ALLOWED_USER_IDS`
- `LISTINGKIT_ZITADEL_ALLOWED_USERNAMES`
- `LISTINGKIT_ZITADEL_ALLOWED_ROLES`

Compatibility aliases remain bound for some old `LISTINGKIT_ZITADEL_*` names,
but new deployment manifests should move to the `TASK_PROCESSOR_*` forms where
they exist.

## Migration Steps

1. Update `config/config-*.yaml` or your generated runtime config so
   `listingkit` and `listingkit.zitadel` keys exist explicitly.
2. Keep `ZITADEL_ISSUER_URL`, `ZITADEL_CLIENT_ID`, `ZITADEL_CLIENT_SECRET`,
   `ZITADEL_REDIRECT_URI`, and `ZITADEL_POST_LOGOUT_REDIRECT_URI` in the
   runtime secret because the Next.js UI still reads them directly.
3. Move Go API auth toggles and allowlists to the bound envs or YAML:
   `TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED`,
   `TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED`,
   `LISTINGKIT_ZITADEL_ALLOWED_*`.
4. If you want user-level data isolation in addition to tenant-level isolation,
   enable `TASK_PROCESSOR_LISTINGKIT_ZITADEL_OWNER_SCOPE_REQUIRED` only after
   confirming historical rows have `user_id` populated.
5. Remove any operator runbooks that tell engineers to patch
   `internal/app/httpapi/zitadel_auth.go` behavior with ad hoc env changes; the
   middleware now consumes injected runtime config only.

## Smoke Checks After Deploy

1. Sign in through the UI and confirm `/api/zitadel-auth/me` returns the
   expected tenant and role claims.
2. Confirm a user without an allowed role gets `listingkit_role_denied` when
   authorization is enabled.
3. Confirm a permitted user can open the main ListingKit pages and any intended
   operator/admin sections.
4. Restart the API pod and verify Studio async jobs remain queryable from the
   database-backed API.
5. Check API startup logs for config validation errors instead of silent auth
   fallback behavior.
