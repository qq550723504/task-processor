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
$cleanupRequired = $false
$passed = $false
$failure = $null
$activeStage = $null
$acceptanceFailed = $false

function Write-Chain1Stage {
    param([string]$Name, [string]$Status)
    Write-Host "CHAIN1_STAGE $Name $Status"
}

Push-Location $repoRoot
try {
    Write-Chain1Stage "resources" "NOT_RUN"
    $activeStage = "resources"
    $cleanupRequired = $true
    docker compose -p $project -f $composeFile down --volumes --remove-orphans 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 initial Compose cleanup failed with exit code $LASTEXITCODE" }
    docker compose -p $project -f $composeFile up -d --wait
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 Compose startup failed with exit code $LASTEXITCODE" }
    Write-Chain1Stage "resources" "PASS"
    $activeStage = $null

    $head = (git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 git HEAD lookup failed with exit code $LASTEXITCODE" }
    Write-Host "CHAIN1_HEAD $head"
    Write-Chain1Stage "business-chain" "NOT_RUN"
    $activeStage = "business-chain"
    go test -race ./internal/app/httpapi -run '^TestChain1LocalProductAcceptance$' -count=1 -v
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 Go acceptance failed" }
    Write-Chain1Stage "business-chain" "PASS"
    $activeStage = $null

    Write-Chain1Stage "contract" "NOT_RUN"
    $activeStage = "contract"
    go test ./tests -run '^TestIssue388Chain1AcceptanceAssemblyContract$' -count=1
    if ($LASTEXITCODE -ne 0) { throw "CHAIN-1 assembly contract failed" }
    Write-Chain1Stage "contract" "PASS"
    $activeStage = $null
    $passed = $true
}
catch {
    if ($null -ne $activeStage) {
        Write-Chain1Stage $activeStage "FAIL"
        $activeStage = $null
    }
    Write-Chain1Stage "acceptance" "FAIL"
    $acceptanceFailed = $true
    $failure = $_
}
finally {
    Pop-Location
    if ($cleanupRequired -and -not $KeepResources) {
        docker compose -p $project -f $composeFile down --volumes --remove-orphans | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Chain1Stage "cleanup" "PASS"
        } else {
            Write-Chain1Stage "cleanup" "FAIL"
            if ($null -eq $failure) {
                $failure = [System.InvalidOperationException]::new("CHAIN-1 Compose cleanup failed with exit code $LASTEXITCODE")
            }
            if (-not $acceptanceFailed) {
                Write-Chain1Stage "acceptance" "FAIL"
                $acceptanceFailed = $true
            }
        }
    } elseif ($cleanupRequired) {
        Write-Chain1Stage "cleanup" "SKIP"
    } else {
        Write-Chain1Stage "cleanup" "NOT_RUN"
    }
    Remove-Item Env:\CHAIN1_DB_PASSWORD,Env:\CHAIN1_DB_PORT,Env:\CHAIN1_TEMPORAL_PORT,Env:\CHAIN1_TEMPORAL_UI_PORT,Env:\CHAIN1_ACCEPTANCE_DSN,Env:\CHAIN1_TEMPORAL_ADDRESS,Env:\CHAIN1_COMPOSE_FILE,Env:\CHAIN1_COMPOSE_PROJECT -ErrorAction SilentlyContinue
}

if ($null -ne $failure) {
    Write-Error -Message $failure.Exception.Message -ErrorAction Continue
    exit 1
}

if (-not $passed) { exit 1 }
Write-Chain1Stage "acceptance" "PASS"
