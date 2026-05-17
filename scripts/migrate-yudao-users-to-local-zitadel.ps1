param(
    [Parameter(Mandatory = $true)]
    [string] $AccessToken,

    [string] $ZitadelBaseUrl = "http://localhost:8080",
    [long] $TenantId = 246,
    [int] $UserLimit = 3,
    [string] $KubeNamespace = "yudao-cloud",
    [string] $PostgresDeployment = "deploy/postgresql",
    [string] $Database = "ruoyi-vue-pro",
    [string] $DbUser = "postgres",
    [switch] $DryRun
)

$ErrorActionPreference = "Stop"

function ConvertTo-Base64Utf8 {
    param([AllowNull()][string] $Value)
    return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$Value))
}

function Invoke-YudaoSql {
    param([string] $Sql)
    $result = kubectl -n $KubeNamespace exec $PostgresDeployment -- `
        psql -U $DbUser -d $Database -tA -c $Sql
    return (($result -join "`n").Trim())
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

$tenantJson = Invoke-YudaoSql @"
select row_to_json(t)::text
from (
  select id, name, contact_name, contact_mobile, status, package_id, expire_time
  from system_tenant
  where id = $TenantId and deleted = 0
) t
"@

if (-not $tenantJson) {
    throw "Tenant $TenantId was not found or has been deleted."
}

$tenant = $tenantJson | ConvertFrom-Json
$usersRows = Invoke-YudaoSql @"
select row_to_json(u)::text
from (
  select id, username, password, nickname, email, mobile, dept_id, tenant_id
  from system_users
  where tenant_id = $TenantId
    and status = 0
    and deleted = 0
  order by id
  limit $UserLimit
) u
"@
$users = @()
if ($usersRows) {
    foreach ($line in ($usersRows -split "`n")) {
        if (-not [string]::IsNullOrWhiteSpace($line)) {
            $users += ($line | ConvertFrom-Json)
        }
    }
}

$orgName = "yudao-$($tenant.id)-$($tenant.name)-test-$(Get-Date -Format 'yyyyMMddHHmmss')"
$report = [ordered]@{
    mode = if ($DryRun) { "dry-run" } else { "apply" }
    sourceTenant = $tenant
    targetOrgName = $orgName
    targetOrgId = $null
    users = @()
}

if ($DryRun) {
    foreach ($user in $users) {
        $report.users += [ordered]@{
            yudaoUserId = $user.id
            yudaoUsername = $user.username
            targetLoginName = "yudao-u$($user.id)-t$TenantId"
            targetEmail = if ([string]::IsNullOrWhiteSpace($user.email)) {
                "yudao-u$($user.id)-t$TenantId@migration.localhost"
            } else {
                $user.email
            }
            status = "planned"
        }
    }
    $report | ConvertTo-Json -Depth 20
    exit 0
}

$org = Invoke-Zitadel -Method Post -Path "/v2/organizations" -Body @{ name = $orgName }
$orgId = $org.organizationId
$report.targetOrgId = $orgId

Invoke-Zitadel -Method Post -Path "/v2/organizations/$orgId/metadata" -Body @{
    metadata = @(
        @{ key = "yudao_tenant_id"; value = ConvertTo-Base64Utf8 $tenant.id },
        @{ key = "yudao_tenant_name"; value = ConvertTo-Base64Utf8 $tenant.name },
        @{ key = "yudao_package_id"; value = ConvertTo-Base64Utf8 $tenant.package_id },
        @{ key = "migration_source"; value = ConvertTo-Base64Utf8 "yudao-cloud-k8s" }
    )
} | Out-Null

foreach ($user in $users) {
    $loginName = "yudao-u$($user.id)-t$TenantId"
    $email = if ([string]::IsNullOrWhiteSpace($user.email)) {
        "yudao-u$($user.id)-t$TenantId@migration.localhost"
    } else {
        $user.email
    }
    $displayName = if ([string]::IsNullOrWhiteSpace($user.nickname)) { $user.username } else { $user.nickname }

    $entry = [ordered]@{
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

        $entry.targetUserId = $created.userId

        Invoke-Zitadel -Method Post -Path "/v2/users/$($created.userId)/metadata" -OrgId $orgId -Body @{
            metadata = @(
                @{ key = "yudao_user_id"; value = ConvertTo-Base64Utf8 $user.id },
                @{ key = "yudao_tenant_id"; value = ConvertTo-Base64Utf8 $TenantId },
                @{ key = "yudao_username"; value = ConvertTo-Base64Utf8 $user.username },
                @{ key = "yudao_dept_id"; value = ConvertTo-Base64Utf8 $user.dept_id }
            )
        } | Out-Null

        $entry.status = "created"
    } catch {
        $entry.status = "failed"
        $entry.error = $_.ErrorDetails.Message
    }

    $report.users += $entry
}

$report | ConvertTo-Json -Depth 20
