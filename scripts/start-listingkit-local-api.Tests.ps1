$scriptPath = Join-Path $PSScriptRoot "start-listingkit-local-api.ps1"
$objectStorageEnvNames = @(
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PROVIDER",
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PUBLICBASE",
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_BUCKET",
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_REGION",
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ENDPOINT",
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ACCESSKEYID",
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_SECRETACCESSKEY",
    "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_USEPATHSTYLE"
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
        "Set-EnvIfMissing",
        "Import-DotEnvFile",
        "Configure-ListingKitLocalZitadelAuth"
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
            "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PROVIDER=s3",
            "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PUBLICBASE=https://cos.example.com",
            "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_BUCKET=cos-bucket",
            "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ENDPOINT=https://cos.endpoint.example.com",
            "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ACCESSKEYID=local-access",
            "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_SECRETACCESSKEY=local-secret"
        )

        Import-DotEnvFile -Path $envPath
        Set-EnvIfMissing -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PUBLICBASE" -Value "https://oss.example.com"
        Set-EnvIfMissing -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_BUCKET" -Value "oss-bucket"

        [Environment]::GetEnvironmentVariable("TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PUBLICBASE", "Process") | Should Be "https://cos.example.com"
        [Environment]::GetEnvironmentVariable("TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_BUCKET", "Process") | Should Be "cos-bucket"
    }

    It "clears ZITADEL values in disabled local debug mode after loading .env" {
        $envPath = Join-Path $TestDrive ".env"
        Set-Content -LiteralPath $envPath -Value @(
            "ZITADEL_ISSUER_URL=https://auth.example.com",
            "ZITADEL_CLIENT_ID=listingkit-client",
            "ZITADEL_CLIENT_SECRET=listingkit-secret",
            "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES=listingkit_admin",
            "LISTINGKIT_ZITADEL_ALLOWED_USERNAMES=1-admin"
        )

        Import-DotEnvFile -Path $envPath
        Configure-ListingKitLocalZitadelAuth -Mode "Disabled"

        [Environment]::GetEnvironmentVariable("ZITADEL_ISSUER_URL", "Process") | Should Be ""
        [Environment]::GetEnvironmentVariable("ZITADEL_CLIENT_ID", "Process") | Should Be ""
        [Environment]::GetEnvironmentVariable("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES", "Process") | Should Be ""
        [Environment]::GetEnvironmentVariable("LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "Process") | Should Be ""
    }

    It "keeps loaded ZITADEL values when required local debug mode is requested" {
        $envPath = Join-Path $TestDrive ".env"
        Set-Content -LiteralPath $envPath -Value @(
            "ZITADEL_ISSUER_URL=https://auth.example.com",
            "ZITADEL_CLIENT_ID=listingkit-client",
            "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES=listingkit_admin"
        )

        Import-DotEnvFile -Path $envPath
        Configure-ListingKitLocalZitadelAuth -Mode "Required"

        [Environment]::GetEnvironmentVariable("ZITADEL_ISSUER_URL", "Process") | Should Be "https://auth.example.com"
        [Environment]::GetEnvironmentVariable("ZITADEL_CLIENT_ID", "Process") | Should Be "listingkit-client"
        [Environment]::GetEnvironmentVariable("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES", "Process") | Should Be "listingkit_admin"
    }
}
