# Image Agent Acceptance Command Ownership Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the local Image Agent acceptance seed executable out of the ListingKit business-domain tree and enforce application-owned command assembly without depguard exceptions.

**Architecture:** A thin executable under `internal/app/runtime/imageagentacceptance/cmd` delegates to `imageagentacceptanceruntime.Run`. The application runtime owns GORM, ZITADEL, and ListingKit repository assembly; the ListingKit package retains acceptance policy and deterministic seed behavior only.

**Tech Stack:** Go 1.26, GORM, ZITADEL, PowerShell/Pester, golangci-lint depguard.

**Spec:** `docs/superpowers/specs/2026-08-30-image-agent-acceptance-command-ownership-design.md`

## Global Constraints

- Do not add a depguard exception or allowlist.
- Preserve the CLI flags and JSON success payload exactly.
- Keep token and runtime file contents out of logs and errors.
- Preserve the exact Compose project and `127.0.0.1:15433` PostgreSQL binding check.
- Keep `web/listingkit-ui/AGENTS.md` and `web/listingkit-ui/CLAUDE.md` untracked and unstaged.
- Update Draft PR #267 only; do not merge or deploy.

---

### Task 1: Move command ownership to the application runtime

**Files:**
- Create: `internal/app/runtime/imageagentacceptance/runtime.go`
- Create: `internal/app/runtime/imageagentacceptance/runtime_test.go`
- Create: `internal/app/runtime/imageagentacceptance/cmd/main.go`
- Delete: `internal/listingkit/imageagentacceptance/cmd/main.go`
- Delete: `internal/listingkit/imageagentacceptance/cmd/main_test.go`
- Modify: `tests/import_boundaries_test.go`

**Interfaces:**
- Consumes: `imageagentacceptance.LoadRuntimeConfig`, `imageagentacceptance.NewEnvironmentGuard`, `imageagentacceptance.Seed`, `zitadel.NewVerifier`, and `listingkitstore.NewTaskRepository`.
- Produces: `func Run(context.Context, []string, io.Writer, io.Writer) error` in package `imageagentacceptanceruntime`.

- [ ] **Step 1: Add the semantic internal-command boundary test**

Add `TestInternalCmdEntrypointsDoNotImportDomainOrInfraPackages` to scan production files below `internal/**/cmd` and compare imports against the same literal package-prefix list used by `TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages`:

```go
func TestInternalCmdEntrypointsDoNotImportDomainOrInfraPackages(t *testing.T) {
	root := filepath.Join("..", "internal")
	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") ||
			!strings.Contains(filepath.ToSlash(path), "/cmd/") {
			continue
		}
		for quotedImport := range facts.imports {
			importPath := strings.Trim(quotedImport, `"`)
			for _, bannedPrefix := range commandEntrypointBannedPrefixes() {
				if importMatchesPrefix(importPath, bannedPrefix) {
					t.Errorf("%s imports banned command-entrypoint package prefix %s via %s", path, bannedPrefix, importPath)
				}
			}
		}
	}
}
```

- [ ] **Step 2: Run the boundary test and verify RED**

Run:

```powershell
go test ./tests -run 'TestInternalCmdEntrypointsDoNotImportDomainOrInfraPackages$' -count=1
```

Expected: FAIL naming both imports from `internal/listingkit/imageagentacceptance/cmd/main.go` to `internal/listingkit/imageagentacceptance` and `internal/listingkit/store`.

- [ ] **Step 3: Move assembly behavior and tests into the application runtime**

Implement this public entrypoint while preserving the existing flag validation, environment guard, verified GORM handle reuse, ZITADEL verifier, repository construction, seed request, and JSON response:

```go
package imageagentacceptanceruntime

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("listingkit-image-agent-acceptance-seed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeFile := flags.String("runtime-file", "", "generated local acceptance runtime file")
	tokenFile := flags.String("token-file", "", "file containing the browser bearer token")
	sourceURL := flags.String("source-url", "", "public HTTPS source image URL")
	styleURL := flags.String("style-url", "", "optional public HTTPS style image URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return errors.New("unexpected positional arguments")
	}
	if strings.TrimSpace(*runtimeFile) == "" || strings.TrimSpace(*tokenFile) == "" || strings.TrimSpace(*sourceURL) == "" {
		return errors.New("-runtime-file, -token-file and -source-url are required")
	}
	runtimeConfig, err := imageagentacceptance.LoadRuntimeConfig(*runtimeFile)
	if err != nil {
		return err
	}
	token, err := readToken(*tokenFile)
	if err != nil {
		return err
	}
	guard := imageagentacceptance.NewEnvironmentGuard(imageagentacceptance.EnvironmentProbes{ComposeProject: dockerComposeProjectProbe})
	db, err := guard.Verify(ctx, runtimeConfig)
	if err != nil {
		return err
	}
	if db == nil {
		return errors.New("acceptance environment guard returned no database")
	}
	result, err := imageagentacceptance.Seed(ctx, verifiedGuard{db: db}, zitadel.NewVerifier(zitadel.Config{
		IssuerURL: runtimeConfig.IssuerURL, ClientID: runtimeConfig.APIClientID, ClientSecret: runtimeConfig.APIClientSecret,
	}), listingkitstore.NewTaskRepository(db), imageagentacceptance.SeedRequest{
		Runtime: runtimeConfig, Token: token, SourceURL: *sourceURL, StyleURL: *styleURL,
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(struct {
		TaskID string `json:"task_id"`; TenantID string `json:"tenant_id"`; UserID string `json:"user_id"`; WorkspaceURL string `json:"workspace_url"`
	}{result.TaskID, result.TenantID, result.UserID, result.WorkspaceURL})
}
```

Move these helpers with the implementation and preserve their exact signatures:

```go
type verifiedGuard struct{ db *gorm.DB }
func (g verifiedGuard) Verify(context.Context, imageagentacceptance.RuntimeConfig) (*gorm.DB, error)
func readToken(path string) (string, error)
func dockerComposeProjectProbe(context.Context, imageagentacceptance.RuntimeConfig) (bool, error)
func validatePostgresBindings(data []byte) bool
```

Move `TestSeedCommandRequiresLocalFilesAndPublicSourceURL` and `TestValidatePostgresBindingsRequiresExactLoopbackAcceptancePort` into `runtime_test.go`, changing the first test to call:

```go
err := Run(context.Background(), args, io.Discard, io.Discard)
```

- [ ] **Step 4: Create the application-owned thin executable**

Create `internal/app/runtime/imageagentacceptance/cmd/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"

	imageagentacceptanceruntime "task-processor/internal/app/runtime/imageagentacceptance"
)

func main() {
	if err := imageagentacceptanceruntime.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Delete the retired ListingKit-owned command files.

- [ ] **Step 5: Run focused tests and CI-equivalent depguard**

Run:

```powershell
go test ./internal/app/runtime/imageagentacceptance ./internal/app/runtime/imageagentacceptance/cmd ./tests -run 'TestSeedCommand|TestValidatePostgresBindings|TestInternalCmdEntrypointsDoNotImportDomainOrInfraPackages|TestBusinessDomainsDoNotImportAppRuntimeAssembly' -count=1
golangci-lint run --config .golangci.yml --enable-only depguard ./...
```

Expected: all tests pass and depguard exits 0 with no output.

- [ ] **Step 6: Commit the ownership migration**

```powershell
git add -- internal/app/runtime/imageagentacceptance internal/listingkit/imageagentacceptance/cmd tests/import_boundaries_test.go
git diff --cached --check
git commit -m "refactor: move image agent acceptance command assembly"
```

### Task 2: Update and verify the PowerShell command path

**Files:**
- Create: `scripts/image-agent-local-acceptance.Tests.ps1`
- Modify: `scripts/image-agent-local-acceptance.ps1`

**Interfaces:**
- Consumes: `go run ./internal/app/runtime/imageagentacceptance/cmd` with the unchanged CLI flags.
- Produces: the existing `seed` PowerShell mode invoking the application-owned command.

- [ ] **Step 1: Write a Pester test for the real seed argument construction**

Use the PowerShell AST to import `Invoke-Seed`, replace only the external `Invoke-GoCommand` boundary, and assert the captured arguments:

```powershell
$scriptPath = Join-Path $PSScriptRoot "image-agent-local-acceptance.ps1"

function Import-SeedFunction {
    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) { throw $errors[0].Message }
    $function = $ast.FindAll({
        param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq "Invoke-Seed"
    }, $true) | Select-Object -First 1
    $definition = $function.Extent.Text -replace '^function\s+Invoke-Seed', 'function global:Invoke-Seed'
    Invoke-Expression $definition
}

Describe "image-agent-local-acceptance seed routing" {
    BeforeEach { Import-SeedFunction }

    It "invokes the application-owned Image Agent seed command" {
        $global:SourceUrl = "https://example.com/source.png"
        $global:StyleUrl = ""
        $global:TokenFile = Join-Path $TestDrive "token.txt"
        $global:runtimeFile = Join-Path $TestDrive "runtime.env"
        Set-Content -LiteralPath $global:TokenFile -Value "test-token"
        $script:goArguments = @()
        function global:Assert-LocalSourceUrl { param([string]$Url, [string]$Name) }
        function global:Invoke-GoCommand { param([string[]]$Arguments) $script:goArguments = $Arguments }

        try {
            Invoke-Seed
            $script:goArguments[0] | Should Be "run"
            $script:goArguments[1] | Should Be "./internal/app/runtime/imageagentacceptance/cmd"
        } finally {
            Remove-Item Function:\global:Invoke-Seed,Function:\global:Assert-LocalSourceUrl,Function:\global:Invoke-GoCommand -ErrorAction SilentlyContinue
            Remove-Variable SourceUrl,StyleUrl,TokenFile,runtimeFile -Scope Global -ErrorAction SilentlyContinue
        }
    }
}
```

- [ ] **Step 2: Run the Pester test and verify RED**

Run:

```powershell
$result = Invoke-Pester -Path scripts/image-agent-local-acceptance.Tests.ps1 -PassThru
if ($result.FailedCount -eq 0) { throw "expected old command path failure" }
```

Expected: FAIL because the captured second argument is `./internal/listingkit/imageagentacceptance/cmd`.

- [ ] **Step 3: Update the orchestrator path**

Change only the seed command package argument:

```powershell
$arguments = @("run", "./internal/app/runtime/imageagentacceptance/cmd", "-runtime-file", $runtimeFile, "-token-file", $path, "-source-url", $SourceUrl)
```

- [ ] **Step 4: Run PowerShell tests in both supported hosts**

Run:

```powershell
Invoke-Pester -Path @(
  'scripts/image-agent-local-acceptance.Tests.ps1',
  'scripts/start-listingkit-local-api.Tests.ps1',
  'scripts/start-listingkit-local-ui.Tests.ps1'
) -PassThru

powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command '& {
  $result = Invoke-Pester -Path @(
    "scripts/image-agent-local-acceptance.Tests.ps1",
    "scripts/start-listingkit-local-api.Tests.ps1",
    "scripts/start-listingkit-local-ui.Tests.ps1"
  ) -PassThru
  if ($result.FailedCount -gt 0) { exit 1 }
}'
```

Expected: all tests pass in PowerShell 7 and Windows PowerShell 5.1.

- [ ] **Step 5: Commit the orchestrator migration**

```powershell
git add -- scripts/image-agent-local-acceptance.ps1 scripts/image-agent-local-acceptance.Tests.ps1
git diff --cached --check
git commit -m "test: verify image agent acceptance command routing"
```

### Task 3: Update architecture baselines and close PR verification

**Files:**
- Modify: `docs/architecture/architecture-review-checklist.md`
- Modify: `docs/superpowers/plans/2026-08-30-image-agent-local-acceptance.md`
- Modify: `docs/superpowers/plans/2026-08-30-image-agent-acceptance-command-ownership.md`

**Interfaces:**
- Consumes: the final application runtime and command paths from Tasks 1 and 2.
- Produces: current guard documentation and verification evidence for Draft PR #267.

- [ ] **Step 1: Update the guard baseline and acceptance ownership map**

Add this exact guard to the architecture checklist immediately after the existing command guard:

```markdown
- `TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages`
- `TestInternalCmdEntrypointsDoNotImportDomainOrInfraPackages`
```

Document these final owners in the acceptance plan:

```markdown
| internal/app/runtime/imageagentacceptance/runtime.go | Application-layer seed assembly for GORM, ZITADEL verification, and ListingKit repositories. |
| internal/app/runtime/imageagentacceptance/cmd/main.go | Application-owned thin seed executable. |
```

Remove references to `internal/listingkit/imageagentacceptance/cmd`.

- [ ] **Step 2: Run architecture documentation tests**

Run:

```powershell
go test ./tests -run 'TestArchitectureReviewChecklistTracksEveryImportBoundaryGuard|TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages|TestInternalCmdEntrypointsDoNotImportDomainOrInfraPackages|TestBusinessDomainsDoNotImportAppRuntimeAssembly' -count=1
```

Expected: PASS.

- [ ] **Step 3: Reproduce the two unrelated full-suite failures independently**

Run each twice without changing their production code:

```powershell
go test ./internal/pkg/fileio -run 'TestFileUtil_SaveJSONToFile$' -count=1
go test ./internal/pkg/fileio -run 'TestFileUtil_SaveJSONToFile$' -count=1
go test ./internal/sdslogin -run 'TestServiceLoadAuthStateReturnsPersistedAccessTokenWithoutCookies$' -count=1
go test ./internal/sdslogin -run 'TestServiceLoadAuthStateReturnsPersistedAccessTokenWithoutCookies$' -count=1
```

If either failure repeats, report it as a separate pre-existing blocker and do not alter unrelated packages in this PR. If both pass twice, record the earlier full-suite failures as non-deterministic and continue.

- [ ] **Step 4: Run final verification**

Run:

```powershell
golangci-lint run --config .golangci.yml --enable-only depguard ./...
go test -p 1 ./...
git diff --check
```

Expected: depguard and the serial full Go suite pass. If an unrelated flaky test fails, rerun only that test to classify it before deciding whether the PR is blocked.

- [ ] **Step 5: Commit documentation and plan status**

```powershell
git add -- docs/architecture/architecture-review-checklist.md docs/superpowers/plans/2026-08-30-image-agent-local-acceptance.md docs/superpowers/plans/2026-08-30-image-agent-acceptance-command-ownership.md
git diff --cached --check
git commit -m "docs: record acceptance command ownership guards"
```

- [ ] **Step 6: Push and inspect Draft PR #267**

```powershell
git push origin codex/image-agent-local-acceptance
gh pr view 267 --json isDraft,state,mergeStateStatus,statusCheckRollup,reviews
```

Expected: PR remains open and Draft; a new CI run starts. Do not mark Ready, merge, or deploy.
