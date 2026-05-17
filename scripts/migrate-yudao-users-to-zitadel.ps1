param(
    [Parameter(Mandatory = $true)]
    [string] $AccessToken,

    [string] $ZitadelBaseUrl = "https://auth.shuomiai.com",
    [string] $SourceNamespace = "yudao-cloud",
    [string] $SourcePostgresTarget = "sts/postgresql-v18",
    [string] $SourceDatabase = "ruoyi-vue-pro",
    [string] $SourceDbUser = "postgres",
    [string] $TargetNamespace = "yudao-cloud",
    [string] $TargetPostgresTarget = "sts/postgresql-v18",
    [string] $TargetDatabase = "zitadel_auth",
    [string] $TargetDbUser = "postgres",
    [string] $RunId = (Get-Date -Format "yyyyMMddHHmmss"),
    [int] $MaxTenants = 0,
    [int] $UserLimitPerTenant = 0,
    [string] $OutputPath = ".tmp/yudao-zitadel-import-report.json",
    [switch] $DryRun
)

$ErrorActionPreference = "Stop"

function ConvertTo-Base64Utf8 {
    param([AllowNull()][string] $Value)
    return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$Value))
}

function Resolve-OrgName {
    param(
        [string] $TenantId,
        [string] $TenantName
    )

    $alnum = ($TenantName -replace '[^0-9A-Za-z]+', '')
    if ([string]::IsNullOrWhiteSpace($alnum)) {
        return "$TenantId-$TenantName"
    }
    return $TenantName
}

function New-ZitadelOrganization {
    param(
        [string] $TenantId,
        [string] $TenantName
    )

    $candidates = @(
        (Resolve-OrgName -TenantId $TenantId -TenantName $TenantName),
        "$TenantId-$TenantName",
        "tenant-$TenantId"
    ) | Select-Object -Unique

    $lastError = $null
    foreach ($candidate in $candidates) {
        try {
            $created = Invoke-Zitadel -Method Post -Path "/v2/organizations" -Body @{ name = $candidate }
            return [pscustomobject]@{
                organizationId = $created.organizationId
                organizationName = $candidate
            }
        } catch {
            $lastError = $_
        }
    }

    throw $lastError
}

function Invoke-PostgresJsonLines {
    param(
        [string] $Namespace,
        [string] $Target,
        [string] $Database,
        [string] $DbUser,
        [string] $Sql
    )

    $sqlBase64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($Sql))
    $command = "printf '%s' '$sqlBase64' | base64 -d | PGPASSWORD='postgresql2026Zone$' psql -U $DbUser -d $Database -tA"
    $result = kubectl -n $Namespace exec $Target -- bash -lc $command
    return (($result -join "`n").Trim())
}

function ConvertFrom-JsonLines {
    param([AllowNull()][string] $Text)
    $items = @()
    if (-not $Text) {
        return $items
    }
    foreach ($line in ($Text -split "`n")) {
        if (-not [string]::IsNullOrWhiteSpace($line)) {
            $items += ($line | ConvertFrom-Json)
        }
    }
    return $items
}

function Invoke-Zitadel {
    param(
        [string] $Method,
        [string] $Path,
        [object] $Body = $null,
        [string] $OrgId = ""
    )

    $headers = @{
        Authorization  = "Bearer $AccessToken"
        "Content-Type" = "application/json"
    }
    if ($OrgId) {
        $headers["x-zitadel-orgid"] = $OrgId
    }

    $uri = "$ZitadelBaseUrl$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers
    }

    $json = $Body | ConvertTo-Json -Depth 20
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers -Body $json
}

$tenantLimitSql = if ($MaxTenants -gt 0) { "limit $MaxTenants" } else { "" }
$userLimitSql = if ($UserLimitPerTenant -gt 0) { "and user_rank <= $UserLimitPerTenant" } else { "" }

$tenants = ConvertFrom-JsonLines (Invoke-PostgresJsonLines `
    -Namespace $SourceNamespace `
    -Target $SourcePostgresTarget `
    -Database $SourceDatabase `
    -DbUser $SourceDbUser `
    -Sql @"
select row_to_json(t)::text
from (
  select id, name, contact_name, contact_mobile, status, package_id, expire_time
  from system_tenant
  where deleted = 0 and status = 0
  order by id
  $tenantLimitSql
) t
"@)

$users = ConvertFrom-JsonLines (Invoke-PostgresJsonLines `
    -Namespace $SourceNamespace `
    -Target $SourcePostgresTarget `
    -Database $SourceDatabase `
    -DbUser $SourceDbUser `
    -Sql @"
with ranked_users as (
  select
    u.id,
    u.tenant_id,
    u.username,
    u.password,
    u.nickname,
    u.email,
    u.mobile,
    u.dept_id,
    row_number() over (partition by tenant_id order by id) as user_rank
  from system_users u
  join system_tenant t on t.id = u.tenant_id
  where u.deleted = 0
    and u.status = 0
    and t.deleted = 0
    and t.status = 0
)
select row_to_json(u)::text
from (
  select id, tenant_id, username, password, nickname, email, mobile, dept_id
  from ranked_users
  where 1 = 1
    $userLimitSql
  order by tenant_id, id
) u
"@)

$existingLogins = ConvertFrom-JsonLines (Invoke-PostgresJsonLines `
    -Namespace $TargetNamespace `
    -Target $TargetPostgresTarget `
    -Database $TargetDatabase `
    -DbUser $TargetDbUser `
    -Sql @"
select row_to_json(t)::text
from (
  select id, user_name
  from projections.login_names3_users
  order by user_name
) t
"@)

$existingOrgImports = ConvertFrom-JsonLines (Invoke-PostgresJsonLines `
    -Namespace $TargetNamespace `
    -Target $TargetPostgresTarget `
    -Database $TargetDatabase `
    -DbUser $TargetDbUser `
    -Sql @"
select row_to_json(t)::text
from (
  select org_id, convert_from(value, 'UTF8') as yudao_tenant_id
  from projections.org_metadata2
  where key = 'yudao_tenant_id'
) t
"@)

$existingUserImports = ConvertFrom-JsonLines (Invoke-PostgresJsonLines `
    -Namespace $TargetNamespace `
    -Target $TargetPostgresTarget `
    -Database $TargetDatabase `
    -DbUser $TargetDbUser `
    -Sql @"
select row_to_json(t)::text
from (
  select user_id, convert_from(value, 'UTF8') as yudao_user_id
  from projections.user_metadata5
  where key = 'yudao_user_id'
) t
"@)

$existingLoginSet = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
foreach ($login in $existingLogins) {
    [void]$existingLoginSet.Add($login.user_name)
}

$existingOrgByTenantId = @{}
foreach ($org in $existingOrgImports) {
    $existingOrgByTenantId[[string]$org.yudao_tenant_id] = [string]$org.org_id
}

$existingUserByYudaoId = @{}
foreach ($user in $existingUserImports) {
    $existingUserByYudaoId[[string]$user.yudao_user_id] = [string]$user.user_id
}

$duplicateUsernames = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
($users | Group-Object username | Where-Object { $_.Count -gt 1 }) | ForEach-Object {
    [void]$duplicateUsernames.Add($_.Name)
}

function Resolve-LoginName {
    param(
        [string] $TenantId,
        [string] $UserId,
        [string] $Username
    )

    $base = $Username
    if (-not $duplicateUsernames.Contains($base) -and -not $existingLoginSet.Contains($base)) {
        [void]$existingLoginSet.Add($base)
        return $base
    }

    $tenantPrefixed = "$TenantId-$Username"
    if (-not $existingLoginSet.Contains($tenantPrefixed)) {
        [void]$existingLoginSet.Add($tenantPrefixed)
        return $tenantPrefixed
    }

    $withUserId = "$TenantId-$Username-$UserId"
    [void]$existingLoginSet.Add($withUserId)
    return $withUserId
}

$report = [ordered]@{
    mode = if ($DryRun) { "dry-run" } else { "apply" }
    runId = $RunId
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    source = [ordered]@{
        namespace = $SourceNamespace
        postgresTarget = $SourcePostgresTarget
        database = $SourceDatabase
    }
    target = [ordered]@{
        baseUrl = $ZitadelBaseUrl
        namespace = $TargetNamespace
        postgresTarget = $TargetPostgresTarget
        database = $TargetDatabase
    }
    totals = [ordered]@{
        tenantsPlanned = $tenants.Count
        tenantsCreated = 0
        tenantsReused = 0
        tenantsFailed = 0
        usersPlanned = $users.Count
        usersCreated = 0
        usersSkipped = 0
        usersFailed = 0
        syntheticEmails = @($users | Where-Object { [string]::IsNullOrWhiteSpace($_.email) }).Count
    }
    tenants = @()
}

foreach ($tenant in $tenants) {
    $tenantUsers = @($users | Where-Object { [string]$_.tenant_id -eq [string]$tenant.id } | Sort-Object id)
    $targetOrgName = Resolve-OrgName -TenantId ([string]$tenant.id) -TenantName ([string]$tenant.name)
    $tenantReport = [ordered]@{
        yudaoTenantId = $tenant.id
        yudaoTenantName = $tenant.name
        targetOrgName = $targetOrgName
        targetOrgId = $null
        status = "pending"
        usersPlanned = $tenantUsers.Count
        usersCreated = 0
        usersSkipped = 0
        usersFailed = 0
        users = @()
    }

    $existingOrgId = $existingOrgByTenantId[[string]$tenant.id]
    if ($existingOrgId) {
        $tenantReport.targetOrgId = $existingOrgId
        $tenantReport.status = "reused"
        $report.totals.tenantsReused++
    } elseif ($DryRun) {
        $tenantReport.status = "planned"
    } else {
        try {
            $createdOrg = New-ZitadelOrganization -TenantId ([string]$tenant.id) -TenantName ([string]$tenant.name)
            $tenantReport.targetOrgId = $createdOrg.organizationId
            $tenantReport.targetOrgName = $createdOrg.organizationName
            $tenantReport.status = "created"
            $report.totals.tenantsCreated++
            $existingOrgByTenantId[[string]$tenant.id] = [string]$createdOrg.organizationId

            Invoke-Zitadel -Method Post -Path "/v2/organizations/$($createdOrg.organizationId)/metadata" -Body @{
                metadata = @(
                    @{ key = "yudao_tenant_id"; value = ConvertTo-Base64Utf8 $tenant.id },
                    @{ key = "yudao_tenant_name"; value = ConvertTo-Base64Utf8 $tenant.name },
                    @{ key = "yudao_package_id"; value = ConvertTo-Base64Utf8 $tenant.package_id },
                    @{ key = "migration_run_id"; value = ConvertTo-Base64Utf8 $RunId },
                    @{ key = "migration_source"; value = ConvertTo-Base64Utf8 "yudao-cloud-k8s" }
                )
            } | Out-Null
        } catch {
            $tenantReport.status = "failed"
            $tenantReport.error = $_.ErrorDetails.Message
            $tenantReport.usersFailed = $tenantUsers.Count
            $report.totals.tenantsFailed++
            $report.totals.usersFailed += $tenantUsers.Count
            $report.tenants += $tenantReport
            continue
        }
    }

    foreach ($user in $tenantUsers) {
        $finalLogin = Resolve-LoginName -TenantId ([string]$tenant.id) -UserId ([string]$user.id) -Username ([string]$user.username)
        $email = if ([string]::IsNullOrWhiteSpace($user.email)) {
            "$finalLogin@migration.localhost"
        } else {
            [string]$user.email
        }

        $userReport = [ordered]@{
            yudaoUserId = $user.id
            yudaoUsername = $user.username
            targetLoginName = $finalLogin
            targetEmail = $email
            targetUserId = $null
            status = "pending"
        }

        $existingImportedUserId = $existingUserByYudaoId[[string]$user.id]
        if ($existingImportedUserId) {
            $userReport.targetUserId = $existingImportedUserId
            $userReport.status = "skipped_existing"
            $tenantReport.usersSkipped++
            $report.totals.usersSkipped++
            $tenantReport.users += $userReport
            continue
        }

        if ($DryRun) {
            $userReport.status = "planned"
            $tenantReport.users += $userReport
            continue
        }

        $displayName = if ([string]::IsNullOrWhiteSpace($user.nickname)) { [string]$user.username } else { [string]$user.nickname }
        try {
            $createdUser = Invoke-Zitadel -Method Post -Path "/management/v1/users/human/_import" -OrgId $tenantReport.targetOrgId -Body @{
                userName = $finalLogin
                profile = @{
                    firstName = $displayName
                    lastName = "Yudao"
                    displayName = $displayName
                    preferredLanguage = "zh"
                }
                email = @{
                    email = $email
                    isEmailVerified = $true
                }
                hashedPassword = @{
                    value = $user.password
                    algorithm = "bcrypt"
                }
                passwordChangeRequired = $false
                requestPasswordlessRegistration = $false
            }

            $userReport.targetUserId = $createdUser.userId
            $userReport.status = "created"
            $tenantReport.usersCreated++
            $report.totals.usersCreated++
            $existingUserByYudaoId[[string]$user.id] = [string]$createdUser.userId

            Invoke-Zitadel -Method Post -Path "/v2/users/$($createdUser.userId)/metadata" -OrgId $tenantReport.targetOrgId -Body @{
                metadata = @(
                    @{ key = "yudao_user_id"; value = ConvertTo-Base64Utf8 $user.id },
                    @{ key = "yudao_tenant_id"; value = ConvertTo-Base64Utf8 $tenant.id },
                    @{ key = "yudao_username"; value = ConvertTo-Base64Utf8 $user.username },
                    @{ key = "yudao_dept_id"; value = ConvertTo-Base64Utf8 $user.dept_id },
                    @{ key = "migration_run_id"; value = ConvertTo-Base64Utf8 $RunId }
                )
            } | Out-Null
        } catch {
            $userReport.status = "failed"
            $userReport.error = $_.ErrorDetails.Message
            $tenantReport.usersFailed++
            $report.totals.usersFailed++
        }

        $tenantReport.users += $userReport
    }

    $report.tenants += $tenantReport
}

$outputDir = Split-Path -Parent $OutputPath
if ($outputDir) {
    New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
}

$json = $report | ConvertTo-Json -Depth 100
Set-Content -Path $OutputPath -Value $json -Encoding UTF8
$json
