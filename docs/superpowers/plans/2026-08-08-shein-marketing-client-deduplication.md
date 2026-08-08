# SHEIN Marketing Client Deduplication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the unused `productsync` marketing client and make `internal/shein/api/marketing.Client` the only concrete SHEIN marketing API implementation.

**Architecture:** `internal/shein/api/marketing` continues to own marketing protocol types, request construction, response validation, and the concrete client. `internal/shein/productsync` is constrained by a source-boundary regression test to product synchronization responsibilities and no longer imports or implements marketing transport.

**Tech Stack:** Go 1.24, standard `testing` package, `os` and `strings` source-boundary checks, `go vet`, `jscpd` 4.0.5.

## Global Constraints

- Do not change any production marketing request path, endpoint, payload, response decoding, error wrapping, or API error message.
- Do not add a compatibility alias, adapter, registry, or replacement implementation under `internal/shein/productsync`.
- Keep `internal/shein/api/marketing.Client` and `NewClient(*client.BaseAPIClient) *Client` as the canonical concrete client and constructor.
- Keep the workflow example comment-only; do not introduce executable example workflow code.
- Do not refactor protocol types, activity enrollment, scheduler, pricing, ListingKit behavior, or unrelated duplicate-code findings.
- Search all tracked production Go files, including build-tagged files, before claiming that the legacy constructor has no consumer.

---

### Task 1: Enforce the Package Boundary and Remove the Duplicate Client

**Files:**
- Create: `internal/shein/productsync/marketing_client_boundary_test.go`
- Delete: `internal/shein/productsync/marketing_repo.go`
- Delete: `internal/shein/productsync/marketing_repo_test.go`
- Modify: `internal/shein/api/marketing/workflow_example.go`

**Interfaces:**
- Consumes: canonical `marketing.NewClient(baseClient *client.BaseAPIClient) *marketing.Client` and the existing canonical client tests.
- Produces: a green `productsync` boundary test, no legacy `productsync.MarketingAPI` implementation, and a corrected comment-only canonical client example.

- [ ] **Step 1: Add the failing package-boundary test**

Create `internal/shein/productsync/marketing_client_boundary_test.go`:

```go
package productsync

import (
	"os"
	"strings"
	"testing"
)

func TestPackageDoesNotOwnMarketingAPIClient(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		for _, forbidden := range []string{
			`"task-processor/internal/shein/api/marketing"`,
			"type MarketingAPI struct",
			"func NewMarketingAPI(",
		} {
			if strings.Contains(string(content), forbidden) {
				t.Fatalf("%s must not contain %q; marketing transport belongs to internal/shein/api/marketing", name, forbidden)
			}
		}
	}
}
```

- [ ] **Step 2: Run the boundary test and verify RED**

Run:

```powershell
go test ./internal/shein/productsync -run TestPackageDoesNotOwnMarketingAPIClient -count=1
```

Expected: FAIL because `marketing_repo.go` contains `"task-processor/internal/shein/api/marketing"`. A compilation failure or unrelated test error is not the expected RED state.

- [ ] **Step 3: Delete the duplicate implementation and redundant test**

Delete exactly:

```text
internal/shein/productsync/marketing_repo.go
internal/shein/productsync/marketing_repo_test.go
```

Do not add a wrapper, alias, or forwarding constructor. The canonical `internal/shein/api/marketing/client.go` remains unchanged.

- [ ] **Step 4: Correct the stale workflow example**

In `internal/shein/api/marketing/workflow_example.go`, replace:

```go
// marketingAPI := repo.NewMarketingAPI(baseClient)
```

with:

```go
// marketingAPI := NewClient(baseClient)
```

Do not uncomment or otherwise modify the example workflow.

- [ ] **Step 5: Format the new test and verify GREEN**

Run:

```powershell
gofmt -w internal/shein/productsync/marketing_client_boundary_test.go
go test ./internal/shein/productsync -run TestPackageDoesNotOwnMarketingAPIClient -count=1
```

Expected: PASS.

- [ ] **Step 6: Run canonical and related-consumer tests**

Run:

```powershell
go test ./internal/shein/api/marketing ./internal/shein/productsync ./internal/shein/activity ./internal/listingkit/sheinsync -count=1
```

Expected: all four packages PASS. The canonical package run must include `TestClientSaveConfigSendsPromotionType`, which replaces the deleted duplicate assertion.

- [ ] **Step 7: Review and commit the focused implementation**

Run:

```powershell
git status --short
git diff --check
git diff -- internal/shein/productsync/marketing_client_boundary_test.go internal/shein/productsync/marketing_repo.go internal/shein/productsync/marketing_repo_test.go internal/shein/api/marketing/workflow_example.go
git add -- internal/shein/productsync/marketing_client_boundary_test.go internal/shein/productsync/marketing_repo.go internal/shein/productsync/marketing_repo_test.go internal/shein/api/marketing/workflow_example.go
git diff --cached --check
git diff --cached --stat
git commit -m "refactor: remove duplicate SHEIN marketing client"
```

Expected: one focused commit containing the two deletions, boundary test, and one-line example correction.

---

### Task 2: Prove Single Ownership and Complete Verification

**Files:**
- Verify: all files changed in Task 1.
- Verify: `internal/shein/api/marketing/client.go` and `client_test.go` remain the canonical implementation and behavioral coverage.

**Interfaces:**
- Consumes: the committed Task 1 deletion and boundary test.
- Produces: exact evidence that production ownership is singular, the known clone is gone, all affected packages are healthy, and the branch is clean.

- [ ] **Step 1: Search every tracked production Go file for legacy ownership**

Run:

```powershell
$productionGoFiles = git ls-files '*.go' | Where-Object { $_ -notmatch '_test\.go$' }
$legacyMatches = Select-String -LiteralPath $productionGoFiles -Pattern 'NewMarketingAPI|productsync\.MarketingAPI|marketing_repo'
$legacyMatches
if ($legacyMatches) { exit 1 }
```

Expected: exit 0 with no output. This uses `git ls-files`, so build-tagged tracked Go files are included.

Run the broader reference audit:

```powershell
git grep -n -E 'NewMarketingAPI|productsync\.MarketingAPI|marketing_repo' -- ':!docs/superpowers/**'
```

Expected: the only permitted `NewMarketingAPI` match is the forbidden-string assertion in `internal/shein/productsync/marketing_client_boundary_test.go`; there must be no definition, call, stale example, or `marketing_repo` path reference.

- [ ] **Step 2: Confirm the canonical implementation remains unchanged**

Run:

```powershell
git diff d4a464eca85da46f3040eec70d3301309ff99176..HEAD -- internal/shein/api/marketing/client.go internal/shein/api/marketing/client_test.go
```

Expected: no output. The deletion must reuse the open-source repository's existing canonical client rather than reimplementing it.

- [ ] **Step 3: Re-run the scoped duplicate-code scan**

Run:

```powershell
npx.cmd --yes jscpd@4.0.5 internal/shein/productsync internal/shein/api/marketing --min-lines 12 --min-tokens 80 --reporters console --ignore "**/*_test.go,**/workflow_example.go"
```

Expected: no clone pair can reference `internal/shein/productsync/marketing_repo.go`; that path no longer exists. Record any unrelated remaining clone separately rather than broadening this branch.

- [ ] **Step 4: Run static checks**

Run:

```powershell
go vet ./internal/shein/api/marketing ./internal/shein/productsync ./internal/shein/activity ./internal/listingkit/sheinsync
git diff --check d4a464eca85da46f3040eec70d3301309ff99176..HEAD
```

Expected: both commands exit 0 with no diagnostics.

- [ ] **Step 5: Run the full backend suite with an extended timeout**

Run from an invocation with at least a ten-minute timeout:

```powershell
go test ./... -count=1
```

Expected: PASS. If it fails or times out, capture the exact package and output and do not report the suite as passing.

- [ ] **Step 6: Review final branch scope**

Run:

```powershell
git status --short
git log --oneline d4a464eca85da46f3040eec70d3301309ff99176..HEAD
git diff --stat d4a464eca85da46f3040eec70d3301309ff99176..HEAD
git diff --check d4a464eca85da46f3040eec70d3301309ff99176..HEAD
```

Expected: the worktree is clean. The branch contains only the design, this plan, the two legacy-file deletions, the boundary test, and the one-line workflow-example correction. Do not create an empty verification commit.
