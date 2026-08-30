[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [Parameter(Position = 0)]
    [ValidateSet("start", "provision", "authorize", "seed", "status", "stop")]
    [string]$Mode = "status",
    [string]$ManagementTokenFile = "",
    [string]$TokenFile = "",
    [string]$SourceUrl = "",
    [string]$StyleUrl = "",
    [switch]$Reset,
    [string]$DockerCommand = "docker",
    [string]$GoCommand = "go"
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$acceptanceRoot = Join-Path $repoRoot ".local\image-agent-acceptance"
$runtimeFile = Join-Path $acceptanceRoot "runtime.env"
$composeEnvFile = Join-Path $acceptanceRoot "compose.env"
$composeFile = Join-Path $repoRoot "deployments\docker\image-agent-acceptance\docker-compose.yml"
$zitadelComposeFile = Join-Path $repoRoot "deployments\docker\zitadel\docker-compose.yml"
$zitadelEnvFile = Join-Path $acceptanceRoot "zitadel.env"
$composeProject = "task-processor-image-agent-acceptance"
$zitadelProject = "task-processor-local-zitadel"
$databaseName = "image_agent_acceptance"
$databasePort = 15433
$redisPort = 16379
$temporalPort = 17233
$temporalUIPort = 18233
$s3Port = 19000
$s3ConsolePort = 19001
$apiPort = 18085
$uiPort = 3000
$apiRuntimeDir = Join-Path $acceptanceRoot "api"
$uiRuntimeDir = Join-Path $acceptanceRoot "ui"
$apiPidFile = Join-Path $apiRuntimeDir "product-listing-api-local.pid"
$uiPidFile = Join-Path $uiRuntimeDir "listingkit-ui-local.pid"
$workerDir = Join-Path $acceptanceRoot "worker"
$workerBinary = Join-Path $workerDir "image-agent-temporal-worker.exe"
$workerStdout = Join-Path $workerDir "stdout.log"
$workerStderr = Join-Path $workerDir "stderr.log"
$workerPidFile = Join-Path $workerDir "worker.pid"

function Ensure-AcceptancePath {
    Assert-NoReparsePoint -Path (Split-Path -Parent $acceptanceRoot)
    if (-not (Test-Path -LiteralPath $acceptanceRoot)) {
        New-Item -ItemType Directory -Path $acceptanceRoot -Force | Out-Null
    }
    Assert-NoReparsePoint -Path $acceptanceRoot
}

function Assert-NoReparsePoint {
    param([string]$Path)

    $current = [System.IO.Path]::GetFullPath($Path)
    while ($current -and $current.StartsWith($repoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        if (Test-Path -LiteralPath $current) {
            $item = Get-Item -LiteralPath $current -Force
            if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
                throw "acceptance private path must not contain a reparse point"
            }
        }
        if ($current -eq $repoRoot) { break }
        $current = Split-Path -Parent $current
    }
}

function Protect-WindowsPrivateFile {
    param([string]$Path)

    $currentSid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    & icacls.exe $Path /inheritance:r /grant:r "*$($currentSid):(F)" "*S-1-5-18:(F)" "*S-1-5-32-544:(F)" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "protect local runtime file ACL failed" }
    $sddl = (Get-Acl -LiteralPath $Path).Sddl
    if ($sddl -notmatch 'D:P') { throw "local runtime file ACL inheritance is not disabled" }
    $allowed = @($currentSid, "SY", "BA", "S-1-5-18", "S-1-5-32-544")
    $identities = @([regex]::Matches($sddl, ';;;([^\)]+)\)') | ForEach-Object { $_.Groups[1].Value })
    if ($identities.Count -eq 0 -or @($identities | Where-Object { $_ -notin $allowed }).Count -gt 0) {
        throw "local runtime file ACL contains an unexpected principal"
    }
}

function New-LocalSecret {
    param([int]$Length = 32)

    $alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    $bytes = New-Object byte[] $Length
    $random = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $random.GetBytes($bytes) } finally { $random.Dispose() }
    $value = New-Object System.Text.StringBuilder
    foreach ($byte in $bytes) {
        [void]$value.Append($alphabet[$byte % $alphabet.Length])
    }
    return $value.ToString()
}

function Read-KeyValueFile {
    param([string]$Path)

    $values = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $values }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) { continue }
        $parts = $trimmed -split "=", 2
        if ($parts.Count -eq 2) { $values[$parts[0].Trim()] = $parts[1].Trim() }
    }
    return $values
}

function Write-PrivateFile {
    param([string]$Path, [string]$Content)

    Ensure-AcceptancePath
    Assert-NoReparsePoint -Path (Split-Path -Parent $Path)
    if (Test-Path -LiteralPath $Path) { Assert-NoReparsePoint -Path $Path }
    $temporaryPath = "$Path.$PID.$([guid]::NewGuid().ToString('N')).tmp"
    [System.IO.File]::WriteAllText($temporaryPath, $Content, [System.Text.UTF8Encoding]::new($false))
    $isWindowsPlatform = [System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT
    try {
        if ($isWindowsPlatform) {
            Protect-WindowsPrivateFile -Path $temporaryPath
        } else {
            & chmod 600 $temporaryPath
            if ($LASTEXITCODE -ne 0) { throw "protect local runtime file failed" }
        }
        Assert-NoReparsePoint -Path (Split-Path -Parent $Path)
        if (Test-Path -LiteralPath $Path) {
            Assert-NoReparsePoint -Path $Path
            Remove-Item -LiteralPath $Path -Force
        }
        Move-Item -LiteralPath $temporaryPath -Destination $Path
    } finally {
        if (Test-Path -LiteralPath $temporaryPath) { Remove-Item -LiteralPath $temporaryPath -Force }
    }
}

function Invoke-DockerCompose {
    param([string[]]$Arguments)

    & $DockerCommand compose --project-name $composeProject --file $composeFile --env-file $composeEnvFile @Arguments
    if ($LASTEXITCODE -ne 0) { throw "docker Compose command failed" }
}

function Invoke-ZitadelCompose {
    param([string[]]$Arguments)

    & $DockerCommand compose --project-name $zitadelProject --file $zitadelComposeFile --env-file $zitadelEnvFile @Arguments
    if ($LASTEXITCODE -ne 0) { throw "local ZITADEL Compose command failed" }
}

function Assert-LocalSourceUrl {
    param([string]$Url, [string]$Name)

    if ([string]::IsNullOrWhiteSpace($Url)) { throw "-$Name is required" }
    try { $parsed = [Uri]$Url } catch { throw "-$Name must be a valid public HTTPS URL" }
    if ($parsed.Scheme -cne "https" -or [string]::IsNullOrWhiteSpace($parsed.Host)) {
        throw "-$Name must be a valid public HTTPS URL"
    }
}

function Invoke-GoCommand {
    param([string[]]$Arguments)

    & $GoCommand @Arguments
    if ($LASTEXITCODE -ne 0) { throw "acceptance Go command failed" }
}

function Stop-LocalProcess {
    param(
        [string]$PidFile,
        [string]$ExpectedExecutablePath = "",
        [string]$ExpectedCommandContains = "",
        [int]$ExpectedPort = 0
    )

    if (-not (Test-Path -LiteralPath $PidFile)) { return }
    $rawPid = (Get-Content -LiteralPath $PidFile -Raw).Trim()
    $processId = 0
    if (-not [int]::TryParse($rawPid, [ref]$processId)) {
        throw "invalid acceptance PID file: $PidFile"
    }
    $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if ($null -eq $process) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
        return
    }

    if (-not [string]::IsNullOrWhiteSpace($ExpectedExecutablePath)) {
        $actualPath = $process.Path
        if (
            [string]::IsNullOrWhiteSpace($actualPath) -or
            -not [System.IO.Path]::GetFullPath($actualPath).Equals(
                [System.IO.Path]::GetFullPath($ExpectedExecutablePath),
                [System.StringComparison]::OrdinalIgnoreCase
            )
        ) {
            throw "PID $processId is not the acceptance executable recorded for $PidFile"
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedCommandContains)) {
        $commandLine = (Get-CimInstance Win32_Process -Filter "ProcessId = $processId" -ErrorAction SilentlyContinue).CommandLine
        if ([string]::IsNullOrWhiteSpace($commandLine) -or $commandLine.IndexOf($ExpectedCommandContains, [System.StringComparison]::OrdinalIgnoreCase) -lt 0) {
            throw "PID $processId command line does not belong to this acceptance checkout"
        }
    }
    if ($ExpectedPort -gt 0) {
        $ownsPort = @(Get-NetTCPConnection -State Listen -LocalPort $ExpectedPort -ErrorAction SilentlyContinue |
            Where-Object { $_.OwningProcess -eq $processId }).Count -gt 0
        if (-not $ownsPort) {
            throw "PID $processId does not own expected acceptance port $ExpectedPort"
        }
    }

    Stop-Process -Id $processId -Force
    $process.WaitForExit()
    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
}

function Import-ProvisionedRuntime {
    $runtime = Read-KeyValueFile -Path $runtimeFile
    foreach ($name in @(
        "ZITADEL_CLIENT_ID",
        "ZITADEL_CLIENT_SECRET",
        "ZITADEL_ISSUER_URL",
        "ZITADEL_REDIRECT_URI",
        "ZITADEL_POST_LOGOUT_REDIRECT_URI",
        "ZITADEL_SCOPES",
        "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID",
        "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET",
        "TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
        "TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED",
        "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES"
    )) {
        if ($runtime.ContainsKey($name)) { [Environment]::SetEnvironmentVariable($name, $runtime[$name], "Process") }
    }
}

function Import-ProvisionedApiRuntime {
    $runtime = Read-KeyValueFile -Path $runtimeFile
    foreach ($mapping in @{
        "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID" = "ZITADEL_CLIENT_ID"
        "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET" = "ZITADEL_CLIENT_SECRET"
    }.GetEnumerator()) {
        if ($runtime.ContainsKey($mapping.Key)) {
            [Environment]::SetEnvironmentVariable($mapping.Value, $runtime[$mapping.Key], "Process")
        }
    }
}

function Start-LocalUi {
    Import-ProvisionedRuntime
    $env:KUBECONFIG = ""
    $env:LISTINGKIT_ACCEPTANCE_TOKEN_FILE = Join-Path $acceptanceRoot "user-token.txt"
    Stop-LocalProcess -PidFile $uiPidFile -ExpectedCommandContains (Join-Path $repoRoot "web\listingkit-ui") -ExpectedPort $uiPort
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "start-listingkit-local-ui.ps1") -Port $uiPort -ApiBase "http://localhost:$apiPort/api/v1/listing-kits" -ServiceApiBase "http://localhost:$apiPort/api/v1" -IsolatedAcceptance -RuntimeDirectory $uiRuntimeDir
    if ($LASTEXITCODE -ne 0) { throw "local ListingKit UI failed to start" }
}

function Set-LocalAcceptanceEnvironment {
    # Local acceptance must never consult a machine-level kubeconfig or import
    # deployed credentials/object-storage settings from another environment.
    $env:KUBECONFIG = ""
    $env:TASK_PROCESSOR_DATABASE_HOST = "127.0.0.1"
    $env:TASK_PROCESSOR_DATABASE_PORT = $databasePort.ToString()
    $env:TASK_PROCESSOR_DATABASE_USER = "acceptance"
    $env:TASK_PROCESSOR_DATABASE_PASSWORD = (Read-KeyValueFile $composeEnvFile)["LISTINGKIT_ACCEPTANCE_DB_PASSWORD"]
    $env:TASK_PROCESSOR_DATABASE_NAME = $databaseName
    $env:TASK_PROCESSOR_REDIS_HOST = "127.0.0.1"
    $env:TASK_PROCESSOR_REDIS_PORT = $redisPort.ToString()
    $env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST = "127.0.0.1"
    $env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT = $redisPort.ToString()
    $env:LISTINGKIT_TEMPORAL_ENABLED = "true"
    $env:LISTINGKIT_TEMPORAL_ADDRESS = "127.0.0.1:$temporalPort"
    $env:IMAGE_AGENT_TEMPORAL_ENABLED = "true"
    $env:IMAGE_AGENT_TEMPORAL_ADDRESS = "127.0.0.1:$temporalPort"
    $env:TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE = "false"
    $env:ZITADEL_ISSUER_URL = "http://localhost:8080"
    $env:TASK_PROCESSOR_OPENAI_API_KEY = "local-acceptance-provider-disabled"
    $env:TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY = "local-acceptance-provider-disabled"
    $composeEnv = Read-KeyValueFile -Path $composeEnvFile
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_ENABLED = "true"
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PROVIDER = "s3"
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PUBLICBASE = "https://local.acceptance.invalid/image-agent-assets"
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_BUCKET = "listingkit-assets"
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_REGION = "us-east-1"
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ENDPOINT = "http://127.0.0.1:$s3Port"
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ACCESSKEYID = $composeEnv["MINIO_ROOT_USER"]
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_SECRETACCESSKEY = $composeEnv["MINIO_ROOT_PASSWORD"]
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_USEPATHSTYLE = "true"
    $env:TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ARTIFACTMODE = "aws"
    $runtime = Read-KeyValueFile -Path $runtimeFile
    if ($runtime.ContainsKey("TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ENABLED")) {
        $env:TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ENABLED = $runtime["TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ENABLED"]
    }
    if ($runtime.ContainsKey("TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS")) {
        $env:TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS = $runtime["TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS"]
    }
}

function Start-LocalApi {
    param([switch]$RequireReadiness)
    Import-ProvisionedRuntime
    Import-ProvisionedApiRuntime
    Set-LocalAcceptanceEnvironment
    Stop-LocalProcess -PidFile $apiPidFile -ExpectedExecutablePath (Join-Path $apiRuntimeDir "product-listing-api-local.exe") -ExpectedPort $apiPort
    $arguments = @("-Port", $apiPort.ToString(), "-IsolatedAcceptance", "-RuntimeDirectory", $apiRuntimeDir)
    if ($RequireReadiness) { $arguments += "-RequireReadiness" }
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "start-listingkit-local-api.ps1") @arguments
    if ($LASTEXITCODE -ne 0) { throw "local ListingKit API failed to start" }
}

function Start-LocalWorker {
    Ensure-AcceptancePath
    if (-not (Test-Path -LiteralPath $workerDir)) { New-Item -ItemType Directory -Path $workerDir -Force | Out-Null }
    Stop-LocalProcess -PidFile $workerPidFile -ExpectedExecutablePath $workerBinary
    if (Test-Path -LiteralPath $workerStdout) { Remove-Item -LiteralPath $workerStdout -Force }
    if (Test-Path -LiteralPath $workerStderr) { Remove-Item -LiteralPath $workerStderr -Force }
    Set-LocalAcceptanceEnvironment
    & $GoCommand build -o $workerBinary ./cmd/image-agent-temporal-worker
    if ($LASTEXITCODE -ne 0) { throw "image agent temporal worker build failed" }
    $process = Start-Process -FilePath $workerBinary -ArgumentList @("-config", "config/config-dev.yaml", "-log-level", "info", "-wire-mode", "v3", "-task-queue", "image-agent-manual-v3") -WorkingDirectory $repoRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput $workerStdout -RedirectStandardError $workerStderr
    Start-Sleep -Seconds 2
    if ($process.HasExited) { throw "image agent temporal worker exited during startup" }
    Set-Content -LiteralPath $workerPidFile -Value $process.Id -NoNewline
    Write-Host "worker=running task_queue=image-agent-manual-v3"
}

function Initialize-AcceptanceRuntime {
    Ensure-AcceptancePath
    $existing = Read-KeyValueFile -Path $composeEnvFile
    $password = if ($existing.ContainsKey("LISTINGKIT_ACCEPTANCE_DB_PASSWORD")) { $existing["LISTINGKIT_ACCEPTANCE_DB_PASSWORD"] } else { New-LocalSecret }
    $marker = if ($existing.ContainsKey("LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER")) { $existing["LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER"] } else { "image-agent-acceptance-$(New-LocalSecret 16)" }
    $composeContent = @(
        "LISTINGKIT_ACCEPTANCE_DB_PASSWORD=$password",
        "LISTINGKIT_ACCEPTANCE_DB_PORT=$databasePort",
        "LISTINGKIT_ACCEPTANCE_REDIS_PORT=$redisPort",
        "LISTINGKIT_ACCEPTANCE_TEMPORAL_PORT=$temporalPort",
        "LISTINGKIT_ACCEPTANCE_TEMPORAL_UI_PORT=$temporalUIPort",
        "LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER=$marker",
        "MINIO_ROOT_USER=acceptance-admin",
        "MINIO_ROOT_PASSWORD=$(if ($existing.ContainsKey('MINIO_ROOT_PASSWORD')) { $existing['MINIO_ROOT_PASSWORD'] } else { New-LocalSecret 40 })",
        "LISTINGKIT_ACCEPTANCE_S3_PORT=$s3Port",
        "LISTINGKIT_ACCEPTANCE_S3_CONSOLE_PORT=$s3ConsolePort"
    ) -join "`n"
    Write-PrivateFile -Path $composeEnvFile -Content ($composeContent + "`n")

    $runtime = Read-KeyValueFile -Path $runtimeFile
    # This DSN is generated from the fixed acceptance target. Recompute it on
    # every start so older runtimes created with a PowerShell interpolation bug
    # are repaired instead of sending Seed to a database named "=disable".
    $runtime["LISTINGKIT_ACCEPTANCE_DATABASE_DSN"] = "postgres://acceptance:$password@127.0.0.1:$databasePort/${databaseName}?sslmode=disable"
    if (-not $runtime.ContainsKey("LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER")) { $runtime["LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER"] = $marker }
    if (-not $runtime.ContainsKey("LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT")) { $runtime["LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT"] = $composeProject }
    if (-not $runtime.ContainsKey("ZITADEL_ISSUER_URL")) { $runtime["ZITADEL_ISSUER_URL"] = "http://localhost:8080" }
    if (-not $runtime.ContainsKey("TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID")) { $runtime["TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID"] = "pending-provision" }
    if (-not $runtime.ContainsKey("TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET")) { $runtime["TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET"] = "pending-provision" }
    if (-not $runtime.ContainsKey("ZITADEL_CLIENT_SECRET")) { $runtime["ZITADEL_CLIENT_SECRET"] = "pending-provision" }
    $keys = @($runtime.Keys | Sort-Object)
    $runtimeContent = (($keys | ForEach-Object { "$_=$($runtime[$_])" }) -join "`n") + "`n"
    Write-PrivateFile -Path $runtimeFile -Content $runtimeContent
}

function Initialize-ZitadelRuntime {
    if (Test-Path -LiteralPath $zitadelEnvFile) {
        $lines = @(Get-Content -LiteralPath $zitadelEnvFile | Where-Object {
            $_ -notmatch '^\s*PROXY_HTTP_PUBLISHED_HOST='
        })
        Write-PrivateFile -Path $zitadelEnvFile -Content (((@($lines) + "PROXY_HTTP_PUBLISHED_HOST=127.0.0.1") -join "`n") + "`n")
        return
    }
    $password = New-LocalSecret
    $masterKey = New-LocalSecret
    $content = @(
        "ZITADEL_DOMAIN=localhost",
        "PROXY_HTTP_PUBLISHED_HOST=127.0.0.1",
        "PROXY_HTTP_PUBLISHED_PORT=8080",
        "ZITADEL_EXTERNALPORT=8080",
        "ZITADEL_EXTERNALSECURE=false",
        "ZITADEL_PUBLIC_SCHEME=http",
        "ZITADEL_MASTERKEY=$masterKey",
        "LOGIN_CLIENT_PAT_EXPIRATION=2099-01-01T00:00:00Z",
        "ZITADEL_VERSION=v4.13.0",
        "TRAEFIK_IMAGE=traefik:v3.6.8",
        "POSTGRES_IMAGE=postgres:17.2-alpine",
        "POSTGRES_DB=zitadel",
        "POSTGRES_ADMIN_USER=postgres",
        "POSTGRES_ADMIN_PASSWORD=$password",
        "TRAEFIK_DASHBOARD_ENABLED=false",
        "TRAEFIK_LOG_LEVEL=INFO",
        "TRAEFIK_ACCESSLOG_ENABLED=true",
        "ZITADEL_ACCESS_LOG_STDOUT_ENABLED=true",
        "ZITADEL_INSTRUMENTATION_SERVICENAME=zitadel-api",
        "ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_TYPE=none",
        "ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_ENDPOINT=",
        "ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_INSECURE=true",
        "LOGIN_OTEL_SERVICE_NAME=zitadel-login",
        "LOGIN_OTEL_EXPORTER_OTLP_ENDPOINT=",
        "LOGIN_OTEL_EXPORTER_OTLP_PROTOCOL=grpc"
    ) -join "`n"
    Write-PrivateFile -Path $zitadelEnvFile -Content ($content + "`n")
}

function Persist-EnvironmentMarker {
    $composeEnv = Read-KeyValueFile -Path $composeEnvFile
    $marker = $composeEnv["LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER"]
    $password = $composeEnv["LISTINGKIT_ACCEPTANCE_DB_PASSWORD"]
    if ([string]::IsNullOrWhiteSpace($marker) -or [string]::IsNullOrWhiteSpace($password)) { throw "acceptance runtime state is incomplete" }
    $env:PGPASSWORD = $password
    try {
        $sql = "CREATE TABLE IF NOT EXISTS listingkit_acceptance_environment (id bigserial PRIMARY KEY, marker text NOT NULL); DELETE FROM listingkit_acceptance_environment; INSERT INTO listingkit_acceptance_environment (marker) VALUES ('$marker');"
        & $DockerCommand compose --project-name $composeProject --file $composeFile --env-file $composeEnvFile exec -T acceptance-postgres psql -U acceptance -d $databaseName -v ON_ERROR_STOP=1 -c $sql 2>$null | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "persist acceptance environment marker failed" }
    } finally {
        Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
    }
}

function Start-Acceptance {
    Initialize-ZitadelRuntime
    Invoke-ZitadelCompose -Arguments @("up", "-d", "--wait")
    Initialize-AcceptanceRuntime
    Invoke-DockerCompose -Arguments @("up", "-d", "--wait", "acceptance-postgres", "temporal", "acceptance-redis", "acceptance-minio")
    Invoke-DockerCompose -Arguments @("run", "--rm", "acceptance-minio-init")
    Set-LocalAcceptanceEnvironment
    Invoke-GoCommand -Arguments @("run", "./cmd/listingkit-schema-migrate", "-config", "config/config-dev.yaml", "-scope", "all")
    Invoke-GoCommand -Arguments @("run", "./cmd/product-listing-api-schema-migrate", "-config", "config/config-dev.yaml")
    Persist-EnvironmentMarker
    Start-LocalApi
    Write-Host "status=ok phase=start project=$composeProject database=$databaseName api=http://localhost:$apiPort ui=http://localhost:$uiPort worker=pending-authorize"
}

function Invoke-Provision {
    $path = if ([string]::IsNullOrWhiteSpace($ManagementTokenFile)) { Join-Path $acceptanceRoot "management-admin-token.txt" } else { $ManagementTokenFile }
    if (-not (Test-Path -LiteralPath $path)) { throw "management token file is required" }
    Invoke-GoCommand -Arguments @("run", "./internal/zitadelprovision/cmd", "provision", "-issuer-url", "http://localhost:8080", "-management-token-file", $path, "-runtime-file", $runtimeFile)
    Import-ProvisionedRuntime
    Set-LocalAcceptanceEnvironment
    Start-LocalApi -RequireReadiness
    Start-LocalUi
}

function Invoke-Authorize {
    $path = if ([string]::IsNullOrWhiteSpace($TokenFile)) { Join-Path $acceptanceRoot "user-token.txt" } else { $TokenFile }
    if (-not (Test-Path -LiteralPath $path)) { throw "browser token file is required" }
    Invoke-GoCommand -Arguments @("run", "./internal/zitadelprovision/cmd", "authorize", "-token-file", $path, "-runtime-file", $runtimeFile)
    Import-ProvisionedRuntime
    Start-LocalWorker
}

function Invoke-Seed {
    Assert-LocalSourceUrl -Url $SourceUrl -Name "SourceUrl"
    $path = if ([string]::IsNullOrWhiteSpace($TokenFile)) { Join-Path $acceptanceRoot "user-token.txt" } else { $TokenFile }
    if (-not (Test-Path -LiteralPath $path)) { throw "browser token file is required" }
    $arguments = @("run", "./internal/app/runtime/imageagentacceptance/cmd", "-runtime-file", $runtimeFile, "-token-file", $path, "-source-url", $SourceUrl)
    if (-not [string]::IsNullOrWhiteSpace($StyleUrl)) { Assert-LocalSourceUrl -Url $StyleUrl -Name "StyleUrl"; $arguments += @("-style-url", $StyleUrl) }
    Invoke-GoCommand -Arguments $arguments
}

function Get-AcceptanceStatus {
    if (-not (Test-Path -LiteralPath $composeEnvFile)) { Write-Host "status=not-started project=$composeProject"; return }
    Invoke-DockerCompose -Arguments @("ps")
    Write-Host "health=GET http://127.0.0.1:$apiPort/readyz"
    try {
        $response = Invoke-WebRequest -Uri "http://127.0.0.1:$apiPort/readyz" -UseBasicParsing -TimeoutSec 5
        Write-Host "api_readiness=$([int]$response.StatusCode)"
    } catch { Write-Host "api_readiness=unavailable" }
    try {
        $tcp = Test-NetConnection -ComputerName 127.0.0.1 -Port $temporalPort -WarningAction SilentlyContinue
        Write-Host "temporal_frontend=$($tcp.TcpTestSucceeded)"
    } catch { Write-Host "temporal_frontend=unavailable" }
    $workerRunning = $false
    if (Test-Path -LiteralPath $workerPidFile) {
        $workerPid = 0
        $workerPidText = (Get-Content -LiteralPath $workerPidFile -Raw).Trim()
        if ([int]::TryParse($workerPidText, [ref]$workerPid) -and $null -ne (Get-Process -Id $workerPid -ErrorAction SilentlyContinue)) {
            $workerRunning = $true
        }
    }
    $workerState = if ($workerRunning) { "running" } else { "pending-or-stopped" }
    Write-Host "worker=$workerState"
}

function Stop-Acceptance {
    $action = if ($Reset) { "stop and remove acceptance containers and volumes" } else { "stop acceptance processes and containers" }
    $target = if ($Reset) { "$composeProject / acceptance-postgres-data / acceptance-s3-data" } else { "$composeProject / $zitadelProject" }
    if ($Reset) {
        Write-Host "reset_scope=project:$composeProject volumes:acceptance-postgres-data,acceptance-s3-data"
    }
    if (-not $PSCmdlet.ShouldProcess($target, $action)) { return }
    Stop-LocalProcess -PidFile $uiPidFile -ExpectedCommandContains (Join-Path $repoRoot "web\listingkit-ui") -ExpectedPort $uiPort
    Stop-LocalProcess -PidFile $workerPidFile -ExpectedExecutablePath $workerBinary
    Stop-LocalProcess -PidFile $apiPidFile -ExpectedExecutablePath (Join-Path $apiRuntimeDir "product-listing-api-local.exe") -ExpectedPort $apiPort
    if ($Reset) {
        if (Test-Path -LiteralPath $composeEnvFile) { Invoke-DockerCompose -Arguments @("down", "-v") }
    } elseif (Test-Path -LiteralPath $composeEnvFile) {
        Invoke-DockerCompose -Arguments @("stop")
    }
    if (Test-Path -LiteralPath $zitadelEnvFile) { Invoke-ZitadelCompose -Arguments @("stop") }
    Write-Host "status=ok phase=stop project=$composeProject"
}

switch ($Mode) {
    "start" { Start-Acceptance }
    "provision" { Invoke-Provision }
    "authorize" { Invoke-Authorize }
    "seed" { Invoke-Seed }
    "status" { Get-AcceptanceStatus }
    "stop" { Stop-Acceptance }
}
