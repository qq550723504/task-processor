[CmdletBinding()]
param(
    [switch]$KeepResources
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot "deployments\docker\chain1-acceptance\docker-compose.yml"
$project = "task-processor-chain1-acceptance-388"
$password = "chain1-local-only-$([guid]::NewGuid().ToString('N'))"
$env:CHAIN1_DB_PASSWORD = $password
$env:CHAIN1_DB_PORT = "17443"
$env:CHAIN1_TEMPORAL_PORT = "17333"
$env:CHAIN1_TEMPORAL_UI_PORT = "18333"
$env:CHAIN1_ACCEPTANCE_DSN = "host=127.0.0.1 port=17443 user=chain1 password=$password dbname=chain1_acceptance sslmode=disable"
$env:CHAIN1_TEMPORAL_ADDRESS = "127.0.0.1:17333"
$env:CHAIN1_COMPOSE_FILE = $composeFile
$env:CHAIN1_COMPOSE_PROJECT = $project
$resourcesTouched = $false
$passed = $false
$failure = $null
$activeStage = $null
$stageStatuses = [ordered]@{}

function Set-Chain1Stage {
    param([string]$Name, [string]$Status)
    $stageStatuses[$Name] = $Status
    Write-Host "CHAIN1_STAGE $Name $Status"
}

foreach ($stageName in @("resources", "business-chain", "contract", "cleanup", "acceptance")) {
    Set-Chain1Stage $stageName "NOT_RUN"
}

Push-Location $repoRoot
try {
    $gitRootOutput = @(git rev-parse --show-toplevel 2>&1)
    $gitRootExitCode = $LASTEXITCODE
    if ($gitRootExitCode -ne 0) { throw "CHAIN-1 git root lookup failed with exit code $gitRootExitCode" }
    $gitRoot = ($gitRootOutput -join "`n").Trim()
    if ([string]::IsNullOrWhiteSpace($gitRoot)) { throw "CHAIN-1 git root lookup returned no repository" }
    $resolvedRepoRoot = [System.IO.Path]::GetFullPath($repoRoot).TrimEnd('\', '/')
    $resolvedGitRoot = [System.IO.Path]::GetFullPath($gitRoot).TrimEnd('\', '/')
    if (-not [string]::Equals($resolvedRepoRoot, $resolvedGitRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "CHAIN-1 script root does not match git repository root"
    }

    $headOutput = @(git rev-parse --verify HEAD 2>&1)
    $headExitCode = $LASTEXITCODE
    if ($headExitCode -ne 0) { throw "CHAIN-1 git HEAD lookup failed with exit code $headExitCode" }
    $head = ($headOutput -join "`n").Trim()
    if ([string]::IsNullOrWhiteSpace($head)) { throw "CHAIN-1 git HEAD lookup returned no commit" }

    $statusOutput = @(git status --porcelain=v1 --untracked-files=all 2>&1)
    $statusExitCode = $LASTEXITCODE
    if ($statusExitCode -ne 0) { throw "CHAIN-1 git status lookup failed with exit code $statusExitCode" }
    if (-not [string]::IsNullOrWhiteSpace($statusOutput -join "`n")) {
        throw "CHAIN-1 requires a clean git index and worktree before attributing acceptance to HEAD"
    }
    Write-Host "CHAIN1_HEAD $head"

    $activeStage = "resources"
    $resourcesTouched = $true
    docker compose -p $project -f $composeFile down --volumes --remove-orphans 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 initial Compose cleanup failed with exit code $LASTEXITCODE" }
    docker compose -p $project -f $composeFile up -d --wait
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 Compose startup failed with exit code $LASTEXITCODE" }
    Set-Chain1Stage "resources" "PASS"
    $activeStage = $null

    $activeStage = "business-chain"
    go test -race ./internal/app/httpapi -run '^TestChain1LocalProductAcceptance$' -count=1 -v
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 Go acceptance failed" }
    Set-Chain1Stage "business-chain" "PASS"
    $activeStage = $null

    $activeStage = "contract"
    go test ./tests -run '^TestIssue388Chain1AcceptanceAssemblyContract$' -count=1
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 assembly contract failed" }
    Set-Chain1Stage "contract" "PASS"
    $activeStage = $null
    $passed = $true
}
catch {
    if ($null -ne $activeStage) {
        Set-Chain1Stage $activeStage "FAIL"
        $activeStage = $null
    }
    Set-Chain1Stage "acceptance" "FAIL"
    $failure = $_
}
finally {
    Pop-Location
    if ($resourcesTouched -and -not $KeepResources) {
        docker compose -p $project -f $composeFile down --volumes --remove-orphans | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Set-Chain1Stage "cleanup" "PASS"
        } else {
            Set-Chain1Stage "cleanup" "FAIL"
            if ($null -eq $failure) {
                $failure = [System.InvalidOperationException]::new("CHAIN-1 Compose cleanup failed with exit code $LASTEXITCODE")
            }
            Set-Chain1Stage "acceptance" "FAIL"
        }
    } elseif ($resourcesTouched) {
        Set-Chain1Stage "cleanup" "SKIP"
    }
    Remove-Item Env:\CHAIN1_DB_PASSWORD,Env:\CHAIN1_DB_PORT,Env:\CHAIN1_TEMPORAL_PORT,Env:\CHAIN1_TEMPORAL_UI_PORT,Env:\CHAIN1_ACCEPTANCE_DSN,Env:\CHAIN1_TEMPORAL_ADDRESS,Env:\CHAIN1_COMPOSE_FILE,Env:\CHAIN1_COMPOSE_PROJECT -ErrorAction SilentlyContinue
}

if ($null -ne $failure) {
    Write-Error -Message $failure.Exception.Message -ErrorAction Continue
    exit 1
}

if (-not $passed) { exit 1 }
Set-Chain1Stage "acceptance" "PASS"
