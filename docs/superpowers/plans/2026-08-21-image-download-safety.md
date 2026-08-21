# Image Download Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route local listing-runtime image downloads through the shared SSRF-safe client with a strict, tested response-size limit and no TLS bypass.

**Architecture:** Extend `internal/pkg/safeimagehttp` with one transport-independent bounded fetch helper. Keep URL validation, DNS/IP filtering, redirect validation, and body-size enforcement in that package. Make `internal/listingruntime/local.ImageDownloader` a thin adapter that owns only timeout configuration and the existing `DownloadImage` interface.

**Tech Stack:** Go 1.24+, `net/http`, `io`, standard `testing`, existing `internal/pkg/safeimagehttp`.

**Spec:** `docs/superpowers/specs/2026-08-21-image-download-safety-design.md`

## Global Constraints

- Accept only absolute public HTTPS URLs.
- Validate every redirect and dial only public resolved IPs.
- Disable environment proxies for public image requests.
- Read at most `maxBytes + 1` bytes and reject overflow; never silently truncate.
- Preserve `DownloadImage(url string) ([]byte, error)` for runtime consumers.
- Do not change platform anti-bot behavior or the SHEIN API downloader in this plan.
- Do not read or print credentials, cookies, or proxy settings.

---

### Task 1: Add bounded shared image fetch behavior

**Files:**
- Modify: `internal/pkg/safeimagehttp/client.go`
- Modify: `internal/pkg/safeimagehttp/client_test.go`

**Interfaces:**
- Produces `safeimagehttp.DefaultMaxBodyBytes int64` with value `32 << 20`.
- Produces `safeimagehttp.Download(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, error)`.
- `Download` validates `rawURL`, uses the provided client or a new public client, checks a positive limit, rejects non-2xx responses, checks a declared `Content-Length` over the limit, and reads through `maxBytes + 1` bytes before returning an overflow error.

- [ ] **Step 1: Write the failing declared-length test**

Add a fake `RoundTripper` that returns a 200 response with `ContentLength` greater than a small limit and a body that would fail if read. Assert `Download` returns an oversize error and the transport was called only after URL validation.

- [ ] **Step 2: Run the test and verify the expected failure**

Run:

```powershell
go test ./internal/pkg/safeimagehttp -run TestDownloadRejectsDeclaredBodyOverLimit -count=1
```

Expected: compilation or test failure because `Download` and `DefaultMaxBodyBytes` do not exist.

- [ ] **Step 3: Write the failing streamed-overflow test**

Add a 200 response whose `ContentLength` is unknown and whose body contains `limit + 1` bytes. Assert the returned error identifies an oversized image and the returned data is nil.

- [ ] **Step 4: Implement the bounded helper**

Use this behavior in `client.go`:

```go
const DefaultMaxBodyBytes int64 = 32 << 20

func Download(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, error) {
	validatedURL, err := ValidatePublicHTTPSURL(rawURL)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("image body limit must be positive")
	}
	if client == nil {
		client = NewPublicImageHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download image %s: status %d", validatedURL, resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("image body exceeds limit: %d bytes (max %d)", resp.ContentLength, maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("image body exceeds limit: more than %d bytes", maxBytes)
	}
	return data, nil
}
```

Keep redirect validation in `NewPublicImageHTTPClient`; the helper must not introduce a second client policy.

- [ ] **Step 5: Run the helper tests**

Run:

```powershell
go test ./internal/pkg/safeimagehttp -count=1
```

Expected: PASS with the existing proxy/cancellation tests and both new overflow tests.

- [ ] **Step 6: Commit the helper**

```powershell
git add internal/pkg/safeimagehttp/client.go internal/pkg/safeimagehttp/client_test.go
git commit -m "fix: bound shared image downloads"
```

### Task 2: Migrate the local listing runtime

**Files:**
- Modify: `internal/listingruntime/local/image_downloader.go`
- Modify: `internal/listingruntime/local/local_runtime_adapter.go`
- Create: `internal/listingruntime/local/image_downloader_test.go`

**Interfaces:**
- `NewImageDownloader(timeout time.Duration) *ImageDownloader` creates `safeimagehttp.NewPublicImageHTTPClient`, sets its timeout, and never disables TLS verification.
- `LocalRuntimeOptions` contains only `SheinCookieProvider`; `ImageDownloadInsecureTLS` and the `insecureImages` field are removed because no maintained caller uses them.
- `ImageDownloader.DownloadImage` calls `safeimagehttp.Download(context.Background(), d.client, url, safeimagehttp.DefaultMaxBodyBytes)` and preserves the nil-downloader guard.

- [ ] **Step 1: Write the failing local runtime safety tests**

Create a local-package test with a counting `RoundTripper`. Assert a non-HTTPS URL returns an error without invoking the transport. Add a second test using a bounded `https://example.com/image` response and assert the bytes are returned.

- [ ] **Step 2: Run the tests and verify they fail for the old implementation**

Run:

```powershell
go test ./internal/listingruntime/local -run 'TestImageDownloader' -count=1
```

Expected: the non-HTTPS test fails because the old downloader accepts it, and the constructor signature test fails until the bypass option is removed.

- [ ] **Step 3: Implement the local adapter migration**

Replace the custom transport construction with the shared public client, remove the unused TLS-bypass state and option, and delegate body/status handling to `safeimagehttp.Download`. Keep the existing 120-second timeout in `GetImageDownloader`.

- [ ] **Step 4: Run focused local runtime tests**

Run:

```powershell
go test ./internal/listingruntime/local ./internal/app/bootstrap/resources ./internal/platforms/shein ./internal/shein/pipeline -count=1
```

Expected: PASS with no compile errors from the removed option.

- [ ] **Step 5: Commit the local migration**

```powershell
git add internal/listingruntime/local/image_downloader.go internal/listingruntime/local/local_runtime_adapter.go internal/listingruntime/local/image_downloader_test.go
git commit -m "fix: secure local runtime image downloads"
```

### Task 3: Verify boundaries and repository health

**Files:**
- No source changes expected.

- [ ] **Step 1: Run focused architecture checks**

```powershell
go test ./tests -run 'Import|Architecture' -count=1
git diff --check
```

Expected: PASS and no whitespace errors.

- [ ] **Step 2: Run the full Go suite**

```powershell
go test ./... -count=1
```

Expected: PASS. If an unrelated failure appears, record its exact package and output instead of claiming a clean suite.

- [ ] **Step 3: Confirm the change is scoped**

```powershell
git status --short
git diff origin/master...HEAD --stat
```

Expected: only the design/spec commits and the two implementation commits appear; no files from the user's dirty PR166 checkout are included.

- [ ] **Step 4: Commit verification notes if needed**

Only add a documentation change if the verification result requires recording a new repository fact; otherwise leave the worktree clean after the implementation commits.
