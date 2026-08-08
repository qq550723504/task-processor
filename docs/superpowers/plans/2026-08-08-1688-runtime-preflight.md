# 1688 Runtime Preflight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the local 1688 replay entry point independent of the historical checkout path and prove the behavior with a PowerShell regression test.

**Architecture:** Keep the existing replay script's runtime settings. Replace its fixed working directory with repository-root resolution from `$PSScriptRoot`, matching the maintained local API starter. Test the script as text/AST so the regression check has no database, Redis, Kubernetes, or credential side effects.

**Tech Stack:** PowerShell, Pester, Go command-line entry point.

## Global Constraints

- Do not print, read, or modify credential values.
- Do not start port-forwarding, call a remote API, create a task, or submit to a marketplace as part of automated tests.
- Preserve the existing local ports and environment variables.
- Keep unrelated working-tree changes untouched.

---

### Task 1: Lock the dynamic repository-root contract with a failing Pester test

**Files:**
- Create: `scripts/start-listingkit-api-local-replay.Tests.ps1`
- Test: `scripts/start-listingkit-api-local-replay.Tests.ps1`

**Interfaces:**
- Consumes: the replay script at `scripts/start-listingkit-api-local-replay.ps1`.
- Produces: a test that fails while the script still contains the stale absolute path.

- [x] **Step 1: Write the failing test**

  Create `scripts/start-listingkit-api-local-replay.Tests.ps1` with:

  ```powershell
  $scriptPath = Join-Path $PSScriptRoot "start-listingkit-api-local-replay.ps1"

  Describe "start-listingkit-api-local-replay repository root" {
      It "resolves and uses the repository root from the script location" {
          $content = Get-Content -LiteralPath $scriptPath -Raw

          $content | Should Match '\$PSScriptRoot'
          $content | Should Match '\$repoRoot'
          $content | Should Match 'Set-Location\s+\$repoRoot'
          $content | Should Not Match 'D:\\code\\task-processor'
      }

      It "has no PowerShell parser errors" {
          $tokens = $null
          $errors = $null
          [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors) | Out-Null

          $errors.Count | Should Be 0
      }
  }
  ```

- [x] **Step 2: Run the focused test and verify it fails for the expected reason**

  Run:

  ```powershell
  Invoke-Pester -Path scripts/start-listingkit-api-local-replay.Tests.ps1 -Output Detailed
  ```

  Expected: the stale-path assertion fails before the production script is changed.

### Task 2: Fix the replay entry point and make the focused test pass

**Files:**
- Modify: `scripts/start-listingkit-api-local-replay.ps1`
- Test: `scripts/start-listingkit-api-local-replay.Tests.ps1`

**Interfaces:**
- Consumes: `$PSScriptRoot` supplied by PowerShell when the script runs.
- Produces: a replay entry point that can be called from any current directory and invokes `go run` from the repository root.

- [x] **Step 1: Add repository-root resolution**

  Add:

  ```powershell
  $ErrorActionPreference = "Stop"
  $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
  ```

- [x] **Step 2: Replace the hard-coded directory change**

  Replace the historical `Set-Location 'D:\code\task-processor'` with:

  ```powershell
  Set-Location $repoRoot
  ```

  Leave the existing environment variables and `go run ./cmd/product-listing-api -config config/config-dev.yaml -port 8085 -log-level info` invocation unchanged.

- [x] **Step 3: Run the focused Pester test**

  Run:

  ```powershell
  Invoke-Pester -Path scripts/start-listingkit-api-local-replay.Tests.ps1 -Output Detailed
  ```

  Expected: all tests pass with no remote calls or process startup.

- [x] **Step 4: Run static script parsing**

  Run:

  ```powershell
  $tokens = $null
  $errors = $null
  [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path scripts/start-listingkit-api-local-replay.ps1), [ref]$tokens, [ref]$errors) | Out-Null
  if ($errors.Count -gt 0) { throw $errors[0].Message }
  ```

  Expected: no parser errors.

### Task 3: Verify the complete change and record the runtime boundary

**Files:**
- Modify: `docs/product/validation/2026-08-08-1688-controlled-replay.md`

**Interfaces:**
- Consumes: focused Pester output, script parser output, and existing Go baseline evidence.
- Produces: an explicit note that the entry-point fix is verified while live runtime acceptance remains pending credentials and environment preflight.

- [x] **Step 1: Run the repository regression suite**

  Run:

  ```powershell
  go test ./...
  ```

  Expected: exit code 0.

- [x] **Step 2: Check the worktree diff and scope**

  Run:

  ```powershell
  git diff --check
  git status --short
  git diff -- scripts/start-listingkit-api-local-replay.ps1 scripts/start-listingkit-api-local-replay.Tests.ps1 docs/product/validation/2026-08-08-1688-controlled-replay.md
  ```

  Expected: only the scoped script, regression test, validation note, design spec, and plan are changed; no credentials or generated runtime files are staged.

- [x] **Step 3: Update the validation note**

  Record that the replay entry point is path-independent and that actual API/token/tenant/store preflight must be run manually before any task creation.

- [x] **Step 4: Commit the scoped change**

  ```powershell
  git add scripts/start-listingkit-api-local-replay.ps1 scripts/start-listingkit-api-local-replay.Tests.ps1 docs/product/validation/2026-08-08-1688-controlled-replay.md
  git commit -m "fix: make 1688 replay entry point portable"
  ```
