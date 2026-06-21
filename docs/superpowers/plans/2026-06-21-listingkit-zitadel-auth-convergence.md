# ListingKit ZITADEL Auth Convergence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Go API the only authoritative ListingKit API authentication and authorization layer while keeping the Next.js proxy as a token-forwarding boundary.

**Architecture:** Next.js keeps Auth.js session ownership and forwards either the incoming bearer token or the Auth.js access token. The Go API continues to perform ZITADEL introspection, tenant/user/role injection, allowlist checks, and route role authorization. Frontend proxy code must not call ZITADEL introspection for ListingKit API requests.

**Tech Stack:** Next.js App Router, Auth.js, Vitest, TypeScript, Go, Gin, ZITADEL introspection middleware.

---

## File Structure

- Modify: `web/listingkit-ui/src/app/api/listing-kits/proxy-auth.ts`
  - Remove proxy-path ZITADEL introspection and allowlist authorization.
  - Preserve session issuer/client mismatch protection.
  - Return token plus optional session-derived identity headers.
- Modify: `web/listingkit-ui/src/app/api/listing-kits/route.test.ts`
  - Update existing proxy auth tests that currently expect revalidation of legacy stored sessions.
  - Add regression coverage proving `verifyListingKitRequestIdentity` does not call `fetch`.
- Modify: `internal/listingkit/httpapi/zitadel_auth_test.go`
  - Add a backend authority regression test proving caller-supplied tenant/user headers are overwritten by the token identity.
- No production Go code is expected unless the backend authority regression exposes a gap.

## Task 1: Stop Next.js Proxy Introspection

**Files:**

- Modify: `web/listingkit-ui/src/app/api/listing-kits/route.test.ts`
- Modify: `web/listingkit-ui/src/app/api/listing-kits/proxy-auth.ts`

- [ ] **Step 1: Add failing test for bearer-token forwarding without introspection**

Add this test inside `describe("verifyListingKitRequestIdentity", ...)` in `web/listingkit-ui/src/app/api/listing-kits/route.test.ts`:

```ts
  it("accepts an incoming bearer token without ZITADEL introspection in the proxy", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks", {
        headers: { authorization: "Bearer caller-token-1" },
      }),
    );

    expect(result.response).toBeUndefined();
    expect(result.token).toBe("caller-token-1");
    expect(result.identity).toBeUndefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });
```

- [ ] **Step 2: Add failing test for Auth.js session-token forwarding without introspection**

Add this test in the same `describe` block:

```ts
  it("forwards an Auth.js session token without revalidating the token in the proxy", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    mockedAuthState.session = {
      accessToken: "session-token-1",
      issuerUrl: "https://issuer.example.com",
      clientId: "client-1",
      identity: {
        tenantId: "org-286",
        userId: "user-42",
        username: "admin",
        userType: "zitadel",
        roles: ["listingkit_admin"],
      },
    };
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks"),
    );

    expect(result.response).toBeUndefined();
    expect(result.token).toBe("session-token-1");
    expect(result.identity).toEqual({
      tenantId: "org-286",
      userId: "user-42",
      username: "admin",
      userType: "zitadel",
      roles: ["listingkit_admin"],
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });
```

- [ ] **Step 3: Update the legacy-session test expectation**

Replace the existing test named `revalidates legacy stored sessions that do not record issuer or client` with:

```ts
  it("forwards legacy stored sessions without proxy-side introspection", async () => {
    vi.stubEnv("ZITADEL_ISSUER_URL", "https://issuer.example.com");
    vi.stubEnv("ZITADEL_CLIENT_ID", "client-1");
    mockedAuthState.session = {
      accessToken: "session-token-1",
      identity: {
        tenantId: "org-286",
        userId: "user-42",
        username: "admin",
        userType: "zitadel",
        roles: ["listingkit_admin"],
      },
    };
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    const result = await verifyListingKitRequestIdentity(
      new NextRequest("http://localhost/api/listing-kits/tasks"),
    );

    expect(result.response).toBeUndefined();
    expect(result.token).toBe("session-token-1");
    expect(result.identity).toEqual({
      tenantId: "org-286",
      userId: "user-42",
      username: "admin",
      userType: "zitadel",
      roles: ["listingkit_admin"],
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });
```

- [ ] **Step 4: Run the targeted frontend test and verify it fails**

Run:

```powershell
cd web/listingkit-ui
npm test -- src/app/api/listing-kits/route.test.ts
```

Expected before implementation: FAIL because `verifyListingKitRequestIdentity` calls ZITADEL discovery/introspection for incoming bearer tokens and legacy stored sessions.

- [ ] **Step 5: Remove proxy-side introspection from `proxy-auth.ts`**

In `web/listingkit-ui/src/app/api/listing-kits/proxy-auth.ts`, remove these imports:

```ts
  authorizeZitadelIdentity,
  fetchZitadelDiscovery,
  verifyZitadelAccessToken,
```

Keep these imports:

```ts
  getZitadelAuthOptions,
  readZitadelAccessTokenFromSession,
  readZitadelSessionClientID,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
  readZitadelSessionIssuerURL,
  type ZitadelVerifiedIdentity,
```

Replace the body of `verifyListingKitRequestIdentity` with logic equivalent to:

```ts
  const zitadelOptions = getZitadelAuthOptions();
  if (!zitadelOptions) {
    return {
      response: NextResponse.json(
        {
          error: "zitadel_auth_not_configured",
          message: "ZITADEL authentication is not configured",
        },
        { status: 503 },
      ),
    };
  }

  try {
    const headerToken = extractBearerToken(request.headers.get("authorization"));
    if (headerToken) {
      logRequestInfo("listingkit proxy auth forwarding bearer token", {
        path: request.nextUrl.pathname,
      });
      return { token: headerToken };
    }

    const session = await auth();
    const sessionError = readZitadelSessionError(session);
    if (sessionError) {
      logRequestWarn("listingkit proxy auth session error", {
        path: request.nextUrl.pathname,
        error: sessionError,
      });
      throw new Error(sessionError);
    }

    const zitadelToken = readZitadelAccessTokenFromSession(session);
    const storedIssuerURL = readZitadelSessionIssuerURL(session);
    const storedClientID = readZitadelSessionClientID(session);
    const identity = readZitadelIdentityFromSession(session) ?? undefined;

    logRequestInfo("listingkit proxy auth forwarding stored session", {
      path: request.nextUrl.pathname,
      hasToken: Boolean(zitadelToken),
      hasIdentity: Boolean(identity),
      storedIssuerURL,
      storedClientID,
      configuredIssuerURL: zitadelOptions.issuerUrl,
      configuredClientID: zitadelOptions.clientId,
    });

    if (!zitadelToken) {
      throw new Error("Missing ZITADEL session");
    }

    if (
      (storedIssuerURL && storedIssuerURL !== zitadelOptions.issuerUrl) ||
      (storedClientID && storedClientID !== zitadelOptions.clientId)
    ) {
      throw new Error(
        "Stored ZITADEL session was created for a different issuer/client; please sign in again",
      );
    }

    return {
      identity,
      token: zitadelToken,
    };
  } catch (error) {
    logRequestWarn("listingkit proxy auth rejected request", {
      path: request.nextUrl.pathname,
      error:
        error instanceof Error
          ? error.message
          : "ZITADEL token verification failed",
    });
    return {
      response: NextResponse.json(
        {
          error: "zitadel_token_invalid",
          message:
            error instanceof Error
              ? error.message
              : "ZITADEL token verification failed",
        },
        { status: 401 },
      ),
    };
  }
```

- [ ] **Step 6: Run the targeted frontend test and verify it passes**

Run:

```powershell
cd web/listingkit-ui
npm test -- src/app/api/listing-kits/route.test.ts
```

Expected after implementation: PASS.

- [ ] **Step 7: Commit Task 1**

Run:

```powershell
git add web/listingkit-ui/src/app/api/listing-kits/proxy-auth.ts web/listingkit-ui/src/app/api/listing-kits/route.test.ts
git commit -m "fix: let ListingKit proxy forward ZITADEL tokens"
```

## Task 2: Guard Go Backend Auth Authority

**Files:**

- Modify: `internal/listingkit/httpapi/zitadel_auth_test.go`

- [ ] **Step 1: Add failing-or-proving backend authority test**

Add this test after `TestListingKitZitadelAuthMapsVerifiedIdentityToHeaders` in `internal/listingkit/httpapi/zitadel_auth_test.go`:

```go
func TestListingKitZitadelAuthOverwritesCallerSuppliedIdentityHeaders(t *testing.T) {
	var introspectionToken string
	zitadel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": r.Host + "/oauth/v2/authorize",
				"token_endpoint":         r.Host + "/oauth/v2/token",
				"introspection_endpoint": zitadelURL(r) + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			introspectionToken = r.FormValue("token")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "verified-user",
				"urn:zitadel:iam:user:resourceowner:id": "verified-tenant",
				"urn:zitadel:iam:org:project:roles": map[string]any{
					"listingkit_admin": map[string]any{},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
			Required:  true,
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/tasks",
			Module: "listing-kit",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"tenant_id": c.GetHeader("X-Tenant-ID"),
					"user_id":   c.GetHeader("X-User-ID"),
					"user_type": c.GetHeader("X-User-Type"),
					"roles":     c.GetHeader("X-User-Roles"),
				})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	req.Header.Set("X-Tenant-ID", "spoofed-tenant")
	req.Header.Set("tenant-id", "spoofed-tenant")
	req.Header.Set("X-User-ID", "spoofed-user")
	req.Header.Set("X-User-Type", "spoofed")
	req.Header.Set("X-User-Roles", "spoofed_role")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if introspectionToken != "access-token-1" {
		t.Fatalf("introspection token = %q, want access-token-1", introspectionToken)
	}
	var body map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["tenant_id"] != "verified-tenant" || body["user_id"] != "verified-user" || body["user_type"] != "zitadel" || body["roles"] != "listingkit_admin" {
		t.Fatalf("identity headers = %#v, want verified token identity to overwrite caller headers", body)
	}
}
```

- [ ] **Step 2: Run the targeted Go test**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestListingKitZitadelAuth" -count=1
```

Expected: PASS. If it fails because caller-supplied headers survive, update `zitadelAuthMiddleware.Handle` so it always overwrites `X-Tenant-ID`, `tenant-id`, `X-User-ID`, `X-User-Type`, and `X-User-Roles` from the introspected identity.

- [ ] **Step 3: Commit Task 2**

Run:

```powershell
git add internal/listingkit/httpapi/zitadel_auth_test.go internal/listingkit/httpapi/zitadel_auth_middleware.go
git commit -m "test: guard ListingKit backend auth authority"
```

## Task 3: Verification

**Files:**

- No production files required.

- [ ] **Step 1: Run focused frontend verification**

Run:

```powershell
cd web/listingkit-ui
npm test -- src/app/api/listing-kits/route.test.ts
npm run typecheck
```

Expected: PASS.

- [ ] **Step 2: Run focused Go verification**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestListingKitZitadelAuth" -count=1
go test ./internal/app/httpapi ./internal/listingkit/httpapi -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full Go verification**

Run:

```powershell
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit verification docs only if updated**

If no documentation or validation artifact changed, do not create a commit.

If a validation run is recorded, run:

```powershell
git add docs/product/validation
git commit -m "docs: record ListingKit auth convergence validation"
```
