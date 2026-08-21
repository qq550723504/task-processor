# Image Download Safety Boundary Design

## Status

Approved first implementation slice: local listing runtime image downloads.

## Problem

The local listing runtime currently downloads arbitrary image URLs with a plain
`http.Client`, can optionally disable TLS verification, and reads the complete
response body without a byte limit. This creates inconsistent SSRF, redirect,
TLS, timeout, and memory-safety behavior compared with the existing
`internal/pkg/safeimagehttp` client used by newer image flows.

## Scope

This slice changes only `internal/listingruntime/local` and the shared
`internal/pkg/safeimagehttp` contract needed by it:

- require public HTTPS URLs and validate every redirect;
- resolve and dial only public IP addresses, without an environment proxy;
- retain the existing runtime timeout;
- reject responses whose declared or actual body exceeds one explicit maximum;
- preserve non-2xx errors and the existing `DownloadImage(string)` interface;
- remove the unused local-runtime TLS-bypass option so the image path is always
  strict.

The SHEIN API downloader and the platform-specific anti-bot downloader remain
separate follow-up slices because they have different headers, retry, and
platform-policy responsibilities. This change does not merge those policies
into the generic safety package.

## Design

`internal/pkg/safeimagehttp` owns transport-level safety. It exposes a bounded
body reader/fetch helper that performs URL validation before the request,
checks `Content-Length` when available, and reads at most `maxBytes + 1` bytes
so an oversized body is rejected rather than silently truncated.

`internal/listingruntime/local.ImageDownloader` constructs the shared public
client, applies its timeout, and delegates response handling to the bounded
helper. It no longer constructs a custom TLS configuration. The public API
remains compatible for callers that only depend on `DownloadImage`.

The default maximum image body is 32 MiB. Callers may pass a smaller explicit
limit to the shared helper, but no caller may disable the overflow check.

## Error behavior

- malformed, non-HTTPS, local, private, or link-local URLs return a validation
  error without making a request;
- a redirect to an unsafe URL is rejected by the shared client's redirect hook;
- non-2xx responses return the existing status error from the local downloader;
- a declared or observed body larger than the limit returns an oversize error;
- timeout and cancellation errors are returned unchanged.

## Verification

Add regression tests for:

1. safeimagehttp rejects an oversized declared body;
2. safeimagehttp rejects an oversized streamed body without truncating it;
3. local runtime rejects non-HTTPS URLs before its transport is called;
4. local runtime preserves its configured timeout and accepts a bounded body.

Run the focused packages, the architecture tests, and the full Go suite before
publication. The existing dirty PR166 checkout is outside this worktree and
must remain untouched.
