# Release Boundary Test Portability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make repository-text release-boundary tests execute identically on LF and CRLF worktrees without changing runtime files, workflow semantics, or repository-wide line-ending policy.

**Architecture:** Add one test-only reader in package `tests` that normalizes checked-out text to LF in memory. Route only exact textual mutation and fenced-code-block fixtures through it; leave YAML parsing, binary reads, logs, and production code unchanged.

**Tech Stack:** Go `testing`, `os`, `path/filepath`, `strings`.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-effect-policy-design.md`

## Global Constraints

- No production Go, Markdown, YAML, shell, workflow, or migration file may change.
- Do not add or modify `.gitattributes`, Git configuration, or checkout files.
- Normalize only bytes read as repository text fixtures by exact-string tests.
- YAML tests that parse YAML must continue using the parser and original bytes.
- The helper must normalize both CRLF and bare CR while preserving existing LF.
- This slice is committed and verified independently from the Image Agent refactor.

---

### Task 1: Add the normalized repository-text fixture boundary

**Files:**
- Create: `tests/repository_text_fixture_test.go`

- [ ] **Step 1: Write the failing normalization test**

Create the file with the test first, before the helper exists:

```go
package tests

import "testing"

func TestNormalizeRepositoryTextHandlesWindowsAndBareCR(t *testing.T) {
	got := normalizeRepositoryText([]byte("alpha\r\nbeta\rgamma\n"))
	want := "alpha\nbeta\ngamma\n"
	if got != want {
		t.Fatalf("normalize repository text = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```text
go test ./tests -run '^TestNormalizeRepositoryTextHandlesWindowsAndBareCR$' -count=1
```

Expected: build FAIL with `undefined: normalizeRepositoryText`.

- [ ] **Step 3: Implement the test-only helper**

Add these imports and functions to the same file:

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func normalizeRepositoryText(raw []byte) string {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

func readRepositoryText(t *testing.T, path ...string) string {
	t.Helper()
	fixturePath := filepath.Join(path...)
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read repository text %s: %v", fixturePath, err)
	}
	return normalizeRepositoryText(raw)
}
```

- [ ] **Step 4: Verify GREEN**

Run:

```text
go test ./tests -run '^TestNormalizeRepositoryTextHandlesWindowsAndBareCR$' -count=1
```

Expected: PASS and one named test executes.

- [ ] **Step 5: Commit the helper contract**

```text
git add tests/repository_text_fixture_test.go
git commit -m "test: normalize repository text fixtures"
```

### Task 2: Migrate the known CRLF-sensitive release-boundary fixtures

**Files:**
- Modify: `tests/listingkit_release_boundary_test.go`
- Modify: `tests/listingkit_release_authority_policy_test.go`
- Test: `tests/repository_text_fixture_test.go`

- [ ] **Step 1: Preserve the current Windows failure evidence**

List the exact target tests before running a regex:

```text
go test ./tests -list 'ListingKit(ImageAgentDrainRunbook|ReleaseAuthorityCrossFilePolicy)'
```

Then run:

```text
go test ./tests -run '^(TestListingKitImageAgentDrainRunbookDefinesCompleteSafeInventoryAndRecoveryHorizon|TestListingKitReleaseAuthorityCrossFilePolicyRejectsReleaseGateInvocationOverrides|TestListingKitReleaseAuthorityCrossFilePolicyRejectsStaleOIDCAdjacency|TestListingKitReleaseAuthorityCrossFilePolicyRejectsLaterMutationBeforeRefresh|TestListingKitReleaseAuthorityCrossFilePolicyRejectsLaterUIMutationBeforeRefresh|TestListingKitReleaseAuthorityCrossFilePolicyRejectsNativeSidecarRunner)$' -count=1
```

Expected on a CRLF checkout: FAIL because LF-only snippets or `````bash\n`` delimiters are not found. Capture the exact failing names in the implementation notes; do not reinterpret them as workflow defects.

- [ ] **Step 2: Replace only exact-text fixture reads**

In `TestListingKitImageAgentDrainRunbookDefinesCompleteSafeInventoryAndRecoveryHorizon`, replace the `os.ReadFile`/`string(raw)` sequence for the runbook with:

```go
runbook := readRepositoryText(t, "..", "deployments", "kubernetes", "listingkit-workbench", "README.md")
```

Keep its existing `strings.Split(section, "```bash\n")` and shell syntax checks unchanged so they operate on normalized text.

In the five release-authority mutation tests, replace each direct `os.ReadFile`/`string(raw)` sequence for repository Markdown/YAML/shell fixtures with `readRepositoryText`. Use the same path segments the test already passes to `filepath.Join`; do not normalize parsed YAML inputs or unrelated test data.

- [ ] **Step 3: Verify the exact regression set**

Run the same six-test command from Step 1.

Expected: PASS and all six named tests execute.

- [ ] **Step 4: Verify the complete tests package**

Run:

```text
go test ./tests -count=1
```

Expected: PASS. If another exact-text test fails only because of CRLF, prove that by normalizing its input in memory before adding it to this slice; do not broaden the helper mechanically.

- [ ] **Step 5: Inspect scope and commit**

Run:

```text
git diff --check
git diff --stat
git status --short
```

Expected: only the test helper and the two named release-boundary test files changed.

```text
git add tests/repository_text_fixture_test.go tests/listingkit_release_boundary_test.go tests/listingkit_release_authority_policy_test.go
git commit -m "test: make release fixtures line-ending neutral"
```

## Acceptance Verification

- [ ] Run `go test ./tests -run '^TestNormalizeRepositoryTextHandlesWindowsAndBareCR$' -count=1` and confirm the named test executes.
- [ ] Run the six-test regression command and confirm every named test executes.
- [ ] Run `go test ./tests -count=1` and confirm PASS.
- [ ] Run `git show --stat --oneline HEAD` and confirm the commit contains test files only.
- [ ] Record the exact commit SHA before starting the policy extraction slice.
