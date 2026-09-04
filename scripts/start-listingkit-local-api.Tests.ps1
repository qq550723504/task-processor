$scriptPath = Join-Path $PSScriptRoot "start-listingkit-local-api.ps1"
$objectStorageEnvNames = @(
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_PROVIDER",
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_PUBLIC_BASE",
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_BUCKET",
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_REGION",
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_ENDPOINT",
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_ACCESS_KEY_ID",
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_SECRET_ACCESS_KEY",
    "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_USE_PATH_STYLE"
)
$zitadelEnvNames = @(
    "ZITADEL_ISSUER_URL",
    "ZITADEL_CLIENT_ID",
    "ZITADEL_CLIENT_SECRET",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES",
    "LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS",
    "LISTINGKIT_ZITADEL_ALLOWED_USER_IDS",
    "LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
    "LISTINGKIT_ZITADEL_ALLOWED_ROLES"
)

function Import-StartScriptFunctions {
    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        throw "Unable to parse ${scriptPath}: $($errors[0].Message)"
    }

    $functionNames = @(
        "Set-EnvValue",
        "Import-DotEnvFile",
        "Initialize-ListingKitObjectStorageEnvFromK8s",
        "Initialize-ApiLaunchEnvironment",
        "Get-ListeningProcessIds",
        "Get-ListeningConnections",
        "Assert-PortAvailable",
        "Assert-LoopbackListener",
        "Wait-ForApiReady",
        "Resolve-IsolatedRuntimeDirectory"
    )
    $functions = $ast.FindAll({
        param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
            $functionNames -contains $node.Name
    }, $true)

    foreach ($function in $functions) {
        $definition = $function.Extent.Text -replace "^function\s+$([regex]::Escape($function.Name))", "function global:$($function.Name)"
        Invoke-Expression $definition
    }
}

Describe "start-listingkit-local-api env loading" {
    $previousValues = @{}

    BeforeEach {
        Import-StartScriptFunctions
        $previousValues = @{}
        foreach ($name in @($objectStorageEnvNames + $zitadelEnvNames)) {
            $previousValues[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
            [Environment]::SetEnvironmentVariable($name, $null, "Process")
        }
    }

    AfterEach {
        foreach ($name in @($objectStorageEnvNames + $zitadelEnvNames)) {
            [Environment]::SetEnvironmentVariable($name, $previousValues[$name], "Process")
        }
    }

    It "keeps local .env object storage values ahead of k8s fallback values" {
        $envPath = Join-Path $TestDrive ".env"
        Set-Content -LiteralPath $envPath -Value @(
            "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_PROVIDER=s3",
            "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_PUBLIC_BASE=https://cos.example.com",
            "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_BUCKET=cos-bucket",
            "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_ENDPOINT=https://cos.endpoint.example.com",
            "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_ACCESS_KEY_ID=local-access",
            "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_SECRET_ACCESS_KEY=local-secret"
        )

        Import-DotEnvFile -Path $envPath
        Set-EnvValue -Name "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_PUBLIC_BASE" -Value "https://oss.example.com"
        Set-EnvValue -Name "TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_BUCKET" -Value "oss-bucket"

        [Environment]::GetEnvironmentVariable("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_PUBLIC_BASE", "Process") | Should Be "https://cos.example.com"
        [Environment]::GetEnvironmentVariable("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_BUCKET", "Process") | Should Be "cos-bucket"
    }

    It "does not expose a ZITADEL authentication disable mode" {
        $content = Get-Content -LiteralPath $scriptPath -Raw

        $content | Should Not Match 'ZitadelAuthMode'
        $content | Should Not Match 'Configure-ListingKitLocalZitadelAuth'
    }

    It "does not query deployed object storage when kubeconfig is empty" {
        $content = Get-Content -LiteralPath $scriptPath -Raw

        $content | Should Match 'KUBECONFIG is empty; skipping deployed object storage lookup'
        $content | Should Match 'if \(\[string\]::IsNullOrWhiteSpace\(\$env:KUBECONFIG\)\)'
    }

    It "keeps hostile repository env values out of isolated acceptance" {
        $repoRoot = Join-Path $TestDrive "repo"
        New-Item -ItemType Directory -Path $repoRoot -Force | Out-Null
        Set-Content -LiteralPath (Join-Path $repoRoot ".env") -Value @(
            "TASK_PROCESSOR_DATABASE_HOST=production.example.invalid",
            "TASK_PROCESSOR_DATABASE_NAME=production"
        )
        $previousKubeconfig = $env:KUBECONFIG
        $previousHost = $env:TASK_PROCESSOR_DATABASE_HOST
        $previousDatabase = $env:TASK_PROCESSOR_DATABASE_NAME
        try {
            $env:KUBECONFIG = "C:\hostile\kubeconfig"
            $env:TASK_PROCESSOR_DATABASE_HOST = "127.0.0.1"
            $env:TASK_PROCESSOR_DATABASE_NAME = "image_agent_acceptance"

            Initialize-ApiLaunchEnvironment -RepoRoot $repoRoot -IsolatedAcceptance

            [string]$env:KUBECONFIG | Should Be ""
            $env:TASK_PROCESSOR_DATABASE_HOST | Should Be "127.0.0.1"
            $env:TASK_PROCESSOR_DATABASE_NAME | Should Be "image_agent_acceptance"
        } finally {
            $env:KUBECONFIG = $previousKubeconfig
            $env:TASK_PROCESSOR_DATABASE_HOST = $previousHost
            $env:TASK_PROCESSOR_DATABASE_NAME = $previousDatabase
        }
    }

    It "refuses an occupied port in isolated acceptance" {
        function global:Get-ListeningProcessIds { @(4242) }
        $message = ""
        try {
            Assert-PortAvailable -ListenPort 18085
        } catch {
            $message = $_.Exception.Message
        }
        $message | Should Match "4242"
    }

    It "keeps isolated runtime state below the acceptance root" {
        $repoRoot = Join-Path $TestDrive "repo"
        $allowed = Join-Path $repoRoot ".local\image-agent-acceptance\api"
        $outside = Join-Path $repoRoot ".local\tmp\api"

        Resolve-IsolatedRuntimeDirectory -RepoRoot $repoRoot -RequestedPath $allowed | Should Be ([System.IO.Path]::GetFullPath($allowed))
        $message = ""
        try {
            Resolve-IsolatedRuntimeDirectory -RepoRoot $repoRoot -RequestedPath $outside
        } catch {
            $message = $_.Exception.Message
        }
        $message | Should Match "must be below"
    }

    It "requires the real readiness endpoint even when startup was logged" {
        $script:requestCount = 0
        function global:Get-Process { [pscustomobject]@{ Id = 1234 } }
        function global:Invoke-WebRequest {
            $script:requestCount++
            return [pscustomobject]@{ StatusCode = 200 }
        }
        try {
            Wait-ForApiReady -HealthURL "http://127.0.0.1:18085/health" -ReadinessURL "http://127.0.0.1:18085/readyz" -RequireReadiness -ProcessId 1234 -TimeoutSeconds 1
            $script:requestCount | Should Be 2
        } finally {
            Remove-Item Function:\global:Get-Process -ErrorAction SilentlyContinue
            Remove-Item Function:\global:Invoke-WebRequest -ErrorAction SilentlyContinue
        }
    }

    It "rejects a wildcard listener in isolated acceptance" {
        function global:Get-ListeningConnections { @([pscustomobject]@{ LocalAddress = "0.0.0.0"; OwningProcess = 1234 }) }
        $message = ""
        try {
            Assert-LoopbackListener -ListenPort 18085
        } catch {
            $message = $_.Exception.Message
        }
        $message | Should Match "not bound exclusively to loopback"
    }

    It "passes the loopback bind address only in isolated acceptance" {
        $content = Get-Content -LiteralPath $scriptPath -Raw
        $content | Should Match 'if \(\$IsolatedAcceptance\) \{\s*\$apiArguments \+= @\("-bind-address", "127\.0\.0\.1"\)'
    }
}
