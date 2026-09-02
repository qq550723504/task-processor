package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProvisionPhaseWritesRuntimeFileWithoutPrintingSecrets(t *testing.T) {
	tempDir := t.TempDir()
	managementTokenFile := filepath.Join(tempDir, "management-token.txt")
	runtimeFile := acceptanceRuntimeFile(tempDir)
	if err := os.WriteFile(managementTokenFile, []byte("management-token-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var apiBody map[string]any
	var oidcBody map[string]any
	var projectUpdateBody map[string]any
	var grantBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireManagementAuth(t, r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/_search":
			writeMainJSON(t, w, map[string]any{"result": []map[string]any{{"id": "project-1", "name": "ListingKit"}}})
		case r.Method == http.MethodPut && r.URL.Path == "/management/v1/projects/project-1":
			if err := json.NewDecoder(r.Body).Decode(&projectUpdateBody); err != nil {
				t.Fatalf("decode project update body: %v", err)
			}
			writeMainJSON(t, w, map[string]any{"details": map[string]any{"sequence": "2"}})
		case r.Method == http.MethodGet && r.URL.Path == "/management/v1/global/users/_by_login_name":
			writeMainJSON(t, w, map[string]any{"user": map[string]any{
				"id": "user-1", "preferredLoginName": "zitadel-admin@zitadel.localhost",
				"details": map[string]any{"resourceOwner": "org-1"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/users/grants/_search":
			writeMainJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			if err := json.NewDecoder(r.Body).Decode(&grantBody); err != nil {
				t.Fatalf("decode grant body: %v", err)
			}
			writeMainJSON(t, w, map[string]any{"id": "authorization-1"})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/roles/_search":
			writeMainJSON(t, w, map[string]any{"result": localDefaultRoleResponses()})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/apps/_search":
			writeMainJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/apps/api":
			if err := json.NewDecoder(r.Body).Decode(&apiBody); err != nil {
				t.Fatalf("decode API app body: %v", err)
			}
			writeMainJSON(t, w, map[string]any{
				"appId":        "api-app-1",
				"clientId":     "api-client-1",
				"clientSecret": "api-secret-1",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/apps/oidc":
			if err := json.NewDecoder(r.Body).Decode(&oidcBody); err != nil {
				t.Fatalf("decode OIDC app body: %v", err)
			}
			writeMainJSON(t, w, map[string]any{
				"appId":        "oidc-app-1",
				"clientId":     "oidc-client-1",
				"clientSecret": "oidc-secret-1",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{
		"provision",
		"-issuer-url", server.URL,
		"-management-token-file", managementTokenFile,
		"-runtime-file", runtimeFile,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run provision returned error: %v", err)
	}

	output := stdout.String() + stderr.String()
	for _, secret := range []string{"management-token-secret", "api-secret-1", "oidc-secret-1"} {
		if strings.Contains(output, secret) {
			t.Fatalf("command output leaked %q: %s", secret, output)
		}
	}
	if !strings.Contains(output, "status=ok phase=provision") {
		t.Fatalf("output = %q, want provision success status", output)
	}
	if apiBody["name"] != "ListingKit Local API" || apiBody["authMethodType"] != "API_AUTH_METHOD_TYPE_BASIC" {
		t.Fatalf("API app body = %#v", apiBody)
	}
	if oidcBody["name"] != "ListingKit Local OIDC" || oidcBody["appType"] != "OIDC_APP_TYPE_WEB" ||
		oidcBody["authMethodType"] != "OIDC_AUTH_METHOD_TYPE_BASIC" ||
		oidcBody["accessTokenType"] != "OIDC_TOKEN_TYPE_BEARER" || oidcBody["accessTokenRoleAssertion"] != true ||
		oidcBody["idTokenRoleAssertion"] != true || oidcBody["devMode"] != true {
		t.Fatalf("OIDC app body = %#v", oidcBody)
	}
	if projectUpdateBody["name"] != "ListingKit" || projectUpdateBody["projectRoleAssertion"] != true ||
		projectUpdateBody["projectRoleCheck"] != true || projectUpdateBody["hasProjectCheck"] != false {
		t.Fatalf("project update body = %#v", projectUpdateBody)
	}
	if grantBody["userId"] != "user-1" || grantBody["organizationId"] != "org-1" || grantBody["projectId"] != "project-1" {
		t.Fatalf("bootstrap grant body = %#v", grantBody)
	}

	runtimeData, err := os.ReadFile(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	runtime := string(runtimeData)
	for _, want := range []string{
		"ZITADEL_ISSUER_URL=" + server.URL,
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=project-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_MANAGEMENT_TOKEN=management-token-secret",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID=api-client-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET=api-secret-1",
		"ZITADEL_CLIENT_ID=oidc-client-1",
		"ZITADEL_CLIENT_SECRET=oidc-secret-1",
		"ZITADEL_REDIRECT_URI=http://localhost:3000/api/auth/callback/zitadel",
		"ZITADEL_POST_LOGOUT_REDIRECT_URI=http://localhost:3000",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED=true",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_BOOTSTRAP_TENANT_ID=org-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_BOOTSTRAP_USER_ID=user-1",
	} {
		if !strings.Contains(runtime, want+"\n") {
			t.Fatalf("runtime file missing %q:\n%s", want, runtime)
		}
	}
	if !strings.Contains(runtime, "ZITADEL_SCOPES=openid profile email ") {
		t.Fatalf("runtime file missing ZITADEL scopes:\n%s", runtime)
	}
}

func TestAuthorizePhaseVerifiesBrowserTokenBeforeGrant(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "user-token.txt")
	runtimeFile := acceptanceRuntimeFile(tempDir)
	if err := os.WriteFile(tokenFile, []byte("browser-token-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var verified bool
	var grantBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/.well-known/openid-configuration":
			writeMainJSON(t, w, map[string]any{"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect"})
		case r.Method == http.MethodPost && r.URL.Path == "/oauth/v2/introspect":
			if got := r.Header.Get("Authorization"); got != "Basic "+base64.StdEncoding.EncodeToString([]byte("api-client-1:api-secret-1")) {
				t.Fatalf("introspection Authorization = %q", got)
			}
			if got := r.FormValue("token"); got != "browser-token-secret" {
				t.Fatalf("introspection token = %q", got)
			}
			verified = true
			writeMainJSON(t, w, map[string]any{
				"active":                                true,
				"sub":                                   "user-1",
				"urn:zitadel:iam:user:resourceowner:id": "org-1",
				"roles":                                 []string{"listingkit_operator"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/users/grants/_search":
			if !verified {
				t.Fatal("grant search happened before browser token verification")
			}
			requireManagementAuth(t, r)
			writeMainJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			if !verified {
				t.Fatal("authorization creation happened before browser token verification")
			}
			requireManagementAuth(t, r)
			if got := r.Header.Get("Connect-Protocol-Version"); got != "1" {
				t.Fatalf("Connect-Protocol-Version = %q, want 1", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&grantBody); err != nil {
				t.Fatalf("decode grant body: %v", err)
			}
			writeMainJSON(t, w, map[string]any{"id": "authorization-1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	writeRuntimeFixture(t, runtimeFile, server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{
		"authorize",
		"-token-file", tokenFile,
		"-runtime-file", runtimeFile,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run authorize returned error: %v", err)
	}
	if grantBody["userId"] != "user-1" || grantBody["organizationId"] != "org-1" || grantBody["projectId"] != "project-1" {
		t.Fatalf("grant body = %#v", grantBody)
	}
	assertMainStringSlice(t, grantBody["roleKeys"], []string{"listingkit_operator"})

	output := stdout.String() + stderr.String()
	for _, secret := range []string{"browser-token-secret", "management-token-secret", "api-secret-1"} {
		if strings.Contains(output, secret) {
			t.Fatalf("command output leaked %q: %s", secret, output)
		}
	}
	if !strings.Contains(output, "status=ok phase=authorize") {
		t.Fatalf("output = %q, want authorize success status", output)
	}
	runtimeData, err := os.ReadFile(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(runtimeData), "TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_") {
		t.Fatalf("runtime file contains retired ImageAgent tenant gate: %s", runtimeData)
	}
}

func TestAuthorizePhaseGrantsAdminOnlyWithFlag(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "user-token.txt")
	runtimeFile := acceptanceRuntimeFile(tempDir)
	if err := os.WriteFile(tokenFile, []byte("browser-token-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var grantBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeMainJSON(t, w, map[string]any{"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect"})
		case "/oauth/v2/introspect":
			writeMainJSON(t, w, map[string]any{
				"active":                                true,
				"sub":                                   "user-1",
				"urn:zitadel:iam:user:resourceowner:id": "org-1",
			})
		case "/management/v1/users/grants/_search":
			writeMainJSON(t, w, map[string]any{"result": []any{}})
		case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			if err := json.NewDecoder(r.Body).Decode(&grantBody); err != nil {
				t.Fatal(err)
			}
			writeMainJSON(t, w, map[string]any{"id": "authorization-1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	writeRuntimeFixture(t, runtimeFile, server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{
		"authorize",
		"-token-file", tokenFile,
		"-runtime-file", runtimeFile,
		"-grant-admin",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run authorize returned error: %v", err)
	}
	assertMainStringSlice(t, grantBody["roleKeys"], []string{"listingkit_operator", "listingkit_admin"})
}

func TestAuthorizePhaseRejectsAValidTokenForAnotherLocalUser(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "user-token.txt")
	runtimeFile := acceptanceRuntimeFile(tempDir)
	if err := os.WriteFile(tokenFile, []byte("browser-token-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeMainJSON(t, w, map[string]any{"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect"})
		case "/oauth/v2/introspect":
			writeMainJSON(t, w, map[string]any{
				"active":                                true,
				"sub":                                   "other-user",
				"urn:zitadel:iam:user:resourceowner:id": "org-1",
			})
		default:
			t.Fatalf("authorization mutation happened for a mismatched user: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	writeRuntimeFixture(t, runtimeFile, server.URL)

	err := run(context.Background(), []string{
		"authorize",
		"-token-file", tokenFile,
		"-runtime-file", runtimeFile,
	}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("run authorize error = %v, want bootstrap identity mismatch", err)
	}
}

func TestCommandDoesNotAcceptTenantOrUserFlags(t *testing.T) {
	for _, args := range [][]string{
		{"provision", "-tenant-id", "org-1"},
		{"authorize", "-user-id", "user-1"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		err := run(context.Background(), args, &stdout, &stderr)
		if err == nil {
			t.Fatalf("run(%v) accepted tenant/user flag", args)
		}
		if !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Fatalf("run(%v) error = %v", args, err)
		}
	}
}

func TestLocalApplicationConfigAlwaysRotatesExistingSecretsForCrashRecovery(t *testing.T) {
	config := localApplicationConfig()
	if !config.RotateAPIClientSecret || !config.RotateOIDCClientSecret {
		t.Fatalf("local application config = %#v, want unconditional retry-safe rotation", config)
	}
}

func TestWriteRuntimeEnvUsesProtectedAtomicFile(t *testing.T) {
	runtimeFile := acceptanceRuntimeFile(t.TempDir())
	if err := writeRuntimeEnv(runtimeFile, map[string]string{"SAFE_KEY": "safe-value"}); err != nil {
		t.Fatalf("writeRuntimeEnv() error = %v", err)
	}
	data, err := os.ReadFile(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "SAFE_KEY=safe-value\n" {
		t.Fatalf("runtime content = %q", data)
	}
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(runtimeFile)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("runtime mode = %o, want 0600", info.Mode().Perm())
		}
	}
	if err := writeRuntimeEnv(runtimeFile, map[string]string{"SAFE_KEY": "replaced"}); err != nil {
		t.Fatalf("writeRuntimeEnv() replacement error = %v", err)
	}
	replaced, err := os.ReadFile(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(replaced) != "SAFE_KEY=replaced\n" {
		t.Fatalf("runtime replacement content = %q", replaced)
	}
}

func TestWriteRuntimeEnvRejectsSymlink(t *testing.T) {
	runtimeFile := acceptanceRuntimeFile(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(runtimeFile), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.env")
	if err := os.Symlink(outside, runtimeFile); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("Windows symlink privilege is unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := writeRuntimeEnv(runtimeFile, map[string]string{"SAFE_KEY": "changed"}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("writeRuntimeEnv() symlink error = %v", err)
	}
}

func TestWritePrivateFilePreservesExistingFileWhenAtomicReplaceFails(t *testing.T) {
	runtimeFile := acceptanceRuntimeFile(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(runtimeFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeFile, []byte("OLD_VALUE=preserved\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	replaceErr := errors.New("injected replace failure")
	err := writePrivateFileWithReplace(runtimeFile, []byte("NEW_VALUE=not-installed\n"), func(_, _ string) error {
		return replaceErr
	})
	if !errors.Is(err, replaceErr) {
		t.Fatalf("writePrivateFileWithReplace() error = %v, want injected replace failure", err)
	}
	data, readErr := os.ReadFile(runtimeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "OLD_VALUE=preserved\n" {
		t.Fatalf("runtime content after failed replace = %q, want original content", data)
	}
}

func TestRejectSymlinkPathRejectsWindowsJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction behavior")
	}
	target := filepath.Join(t.TempDir(), "target")
	junction := filepath.Join(t.TempDir(), "junction")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("cannot create Windows junction: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	err := rejectSymlinkPath(filepath.Join(junction, "runtime.env"))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "reparse") {
		t.Fatalf("rejectSymlinkPath() junction error = %v, want reparse-point rejection", err)
	}
}

func writeRuntimeFixture(t *testing.T, path string, issuerURL string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"ZITADEL_ISSUER_URL=" + issuerURL,
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=project-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_MANAGEMENT_TOKEN=management-token-secret",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID=api-client-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET=api-secret-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_BOOTSTRAP_TENANT_ID=org-1",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_BOOTSTRAP_USER_ID=user-1",
		"ZITADEL_CLIENT_ID=oidc-client-1",
		"ZITADEL_CLIENT_SECRET=oidc-secret-1",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func acceptanceRuntimeFile(tempDir string) string {
	return filepath.Join(tempDir, ".local", "image-agent-acceptance", "runtime.env")
}

func requireManagementAuth(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer management-token-secret" {
		t.Fatalf("Authorization = %q, want Bearer management-token-secret", got)
	}
}

func localDefaultRoleResponses() []map[string]any {
	return []map[string]any{
		{"key": "listingkit_viewer", "displayName": "ListingKit Viewer", "group": "ListingKit"},
		{"key": "listingkit_operator", "displayName": "ListingKit Operator", "group": "ListingKit"},
		{"key": "listingkit_admin", "displayName": "ListingKit Admin", "group": "ListingKit"},
		{"key": "platform_admin", "displayName": "Platform Admin", "group": "ListingKit"},
	}
}

func writeMainJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func serverURL(t *testing.T, r *http.Request) string {
	t.Helper()
	return "http://" + r.Host
}

func assertMainStringSlice(t *testing.T, got any, want []string) {
	t.Helper()
	values, ok := got.([]any)
	if !ok {
		t.Fatalf("value = %#v, want []string", got)
	}
	if len(values) != len(want) {
		t.Fatalf("slice = %#v, want %#v", values, want)
	}
	for index := range want {
		if values[index] != want[index] {
			t.Fatalf("slice = %#v, want %#v", values, want)
		}
	}
}
