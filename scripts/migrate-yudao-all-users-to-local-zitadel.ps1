param(
    [Parameter(Mandatory = $true)]
    [string] $AccessToken,

    [string] $ZitadelBaseUrl = "http://localhost:8080",
    [string] $KubeNamespace = "yudao-cloud",
    [string] $PostgresDeployment = "deploy/postgresql",
    [string] $Database = "ruoyi-vue-pro",
    [string] $DbUser = "postgres",
    [string] $RunId = (Get-Date -Format "yyyyMMddHHmmss"),
    [int] $MaxTenants = 0,
    [int] $UserLimitPerTenant = 0,
    [string] $OutputPath = ".tmp/yudao-zitadel-local-full-import.json",
    [switch] $DryRun
)

$ErrorActionPreference = "Stop"

function ConvertTo-Base64Utf8 {
    param([AllowNull()][string] $Value)
    return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$Value))
}

function ConvertTo-SafeName {
    param([string] $Value)
    $safe = $Value -replace "[^a-zA-Z0-9_-]+", "-"
    $safe = $safe.Trim("-")
    if ([string]::IsNullOrWhiteSpace($safe)) {
        return "tenant"
    }
    return $safe
}

function ConvertTo-LoginSlug {
    param([string] $Value)
    $slug = ($Value -replace "[^a-zA-Z0-9._-]+", "-").Trim("-")
    if ([string]::IsNullOrWhiteSpace($slug)) {
        return "user"
    }
    return $slug.ToLowerInvariant()
}

function Invoke-YudaoSql {
    param([string] $Sql)
    $result = kubectl -n $KubeNamespace exec $PostgresDeployment -- `
        psql -U $DbUser -d $Database -tA -c $Sql
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
$tenants = ConvertFrom-JsonLines (Invoke-YudaoSql @"
select row_to_json(t)::text
from (
  select id, name, contact_name, contact_mobile, status, package_id, expire_time
  from system_tenant
  where status = 0 and deleted = 0
  order by id
  $tenantLimitSql
) t
"@)

$allUsers = ConvertFrom-JsonLines (Invoke-YudaoSql @"
select row_to_json(u)::text
from (
  select id, username, password, nickname, email, mobile, dept_id, tenant_id
  from system_users
  where status = 0
    and deleted = 0
  order by tenant_id, id
) u
"@)

$usernameCounts = @{}
foreach ($user in $allUsers) {
    $key = "$($user.tenant_id)|$($user.username)"
    if ($usernameCounts.ContainsKey($key)) {
        $usernameCounts[$key]++
    } else {
        $usernameCounts[$key] = 1
    }
}

$report = [ordered]@{
    mode = if ($DryRun) { "dry-run" } else { "apply" }
    runId = $RunId
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    source = [ordered]@{
        kubeNamespace = $KubeNamespace
        postgresDeployment = $PostgresDeployment
        database = $Database
    }
    target = [ordered]@{
        baseUrl = $ZitadelBaseUrl
        system = "local-zitadel"
    }
    totals = [ordered]@{
        tenantsPlanned = $tenants.Count
        tenantsCreated = 0
        tenantsFailed = 0
        usersPlanned = 0
        usersCreated = 0
        usersFailed = 0
        syntheticEmails = 0
    }
    tenants = @()
}

foreach ($tenant in $tenants) {
    $users = @($allUsers | Where-Object { $_.tenant_id -eq $tenant.id } | Sort-Object id)
    if ($UserLimitPerTenant -gt 0) {
        $users = @($users | Select-Object -First $UserLimitPerTenant)
    }

    $orgName = "yudao-$($tenant.id)-$(ConvertTo-SafeName $tenant.name)-local-$RunId"
    $tenantReport = [ordered]@{
        yudaoTenantId = $tenant.id
        yudaoTenantName = $tenant.name
        targetOrgName = $orgName
        targetOrgId = $null
        usersPlanned = $users.Count
        usersCreated = 0
        usersFailed = 0
        users = @()
        status = "pending"
    }
    $report.totals.usersPlanned += $users.Count

    foreach ($user in $users) {
        if ([string]::IsNullOrWhiteSpace($user.email)) {
            $report.totals.syntheticEmails++
        }
    }

    if ($DryRun) {
        $tenantReport.status = "planned"
        foreach ($user in $users) {
            $baseLoginName = "$($tenant.id)-$(ConvertTo-LoginSlug $user.username)"
            $targetLoginName = if ($usernameCounts["$($user.tenant_id)|$($user.username)"] -gt 1) {
                "$baseLoginName-$($user.id)"
            } else {
                $baseLoginName
            }
            $tenantReport.users += [ordered]@{
                yudaoUserId = $user.id
                yudaoUsername = $user.username
                targetLoginName = $targetLoginName
                targetEmail = if ([string]::IsNullOrWhiteSpace($user.email)) {
                    "$targetLoginName@migration.localhost"
                } else {
                    $user.email
                }
                status = "planned"
            }
        }
        $report.tenants += $tenantReport
        continue
    }

    try {
        $org = Invoke-Zitadel -Method Post -Path "/v2/organizations" -Body @{ name = $orgName }
        $orgId = $org.organizationId
        $tenantReport.targetOrgId = $orgId
        $report.totals.tenantsCreated++

        Invoke-Zitadel -Method Post -Path "/v2/organizations/$orgId/metadata" -Body @{
            metadata = @(
                @{ key = "yudao_tenant_id"; value = ConvertTo-Base64Utf8 $tenant.id },
                @{ key = "yudao_tenant_name"; value = ConvertTo-Base64Utf8 $tenant.name },
                @{ key = "yudao_package_id"; value = ConvertTo-Base64Utf8 $tenant.package_id },
                @{ key = "migration_run_id"; value = ConvertTo-Base64Utf8 $RunId },
                @{ key = "migration_source"; value = ConvertTo-Base64Utf8 "yudao-cloud-k8s" }
            )
        } | Out-Null

        foreach ($user in $users) {
            $baseLoginName = "$($tenant.id)-$(ConvertTo-LoginSlug $user.username)"
            $loginName = if ($usernameCounts["$($user.tenant_id)|$($user.username)"] -gt 1) {
                "$baseLoginName-$($user.id)"
            } else {
                $baseLoginName
            }
            $email = if ([string]::IsNullOrWhiteSpace($user.email)) {
                "$loginName@migration.localhost"
            } else {
                $user.email
            }
            $displayName = if ([string]::IsNullOrWhiteSpace($user.nickname)) { $user.username } else { $user.nickname }
            $userReport = [ordered]@{
                yudaoUserId = $user.id
                yudaoUsername = $user.username
                targetLoginName = $loginName
                targetEmail = $email
                targetUserId = $null
                status = "pending"
            }

            try {
                $created = Invoke-Zitadel -Method Post -Path "/management/v1/users/human/_import" -OrgId $orgId -Body @{
                    userName = $loginName
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

                $userReport.targetUserId = $created.userId

                Invoke-Zitadel -Method Post -Path "/v2/users/$($created.userId)/metadata" -OrgId $orgId -Body @{
                    metadata = @(
                        @{ key = "yudao_user_id"; value = ConvertTo-Base64Utf8 $user.id },
                        @{ key = "yudao_tenant_id"; value = ConvertTo-Base64Utf8 $tenant.id },
                        @{ key = "yudao_username"; value = ConvertTo-Base64Utf8 $user.username },
                        @{ key = "yudao_dept_id"; value = ConvertTo-Base64Utf8 $user.dept_id },
                        @{ key = "migration_run_id"; value = ConvertTo-Base64Utf8 $RunId }
                    )
                } | Out-Null

                $userReport.status = "created"
                $tenantReport.usersCreated++
                $report.totals.usersCreated++
            } catch {
                $userReport.status = "failed"
                $userReport.error = $_.ErrorDetails.Message
                $tenantReport.usersFailed++
                $report.totals.usersFailed++
            }

            $tenantReport.users += $userReport
        }

        $tenantReport.status = if ($tenantReport.usersFailed -eq 0) { "created" } else { "created_with_user_failures" }
    } catch {
        $tenantReport.status = "failed"
        $tenantReport.error = $_.ErrorDetails.Message
        $report.totals.tenantsFailed++
        $report.totals.usersFailed += $users.Count
    }

    $report.tenants += $tenantReport
}

$dir = Split-Path -Parent $OutputPath
if ($dir) {
    New-Item -ItemType Directory -Force $dir | Out-Null
}

$json = $report | ConvertTo-Json -Depth 50
Set-Content -Path $OutputPath -Value $json -Encoding UTF8
$json
