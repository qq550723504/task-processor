param(
    [string] $KubeNamespace = "yudao-cloud",
    [string] $PostgresDeployment = "deploy/postgresql",
    [string] $Database = "ruoyi-vue-pro",
    [string] $DbUser = "postgres",
    [string] $OutputPath = ".tmp/yudao-zitadel-migration-dry-run.json"
)

$ErrorActionPreference = "Stop"

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

$summaryJson = Invoke-YudaoSql @"
select row_to_json(s)::text
from (
  select
    (select count(*) from system_tenant where status = 0 and deleted = 0) as active_tenants,
    (select count(*) from system_tenant where status <> 0 or deleted <> 0) as inactive_or_deleted_tenants,
    (select count(*) from system_users where status = 0 and deleted = 0) as active_users,
    (select count(*) from system_users where status <> 0 or deleted <> 0) as inactive_or_deleted_users,
    (select count(*) from system_users where status = 0 and deleted = 0 and coalesce(email, '') = '') as active_users_empty_email,
    (select count(*) from (
      select tenant_id, username
      from system_users
      where status = 0 and deleted = 0
      group by tenant_id, username
      having count(*) > 1
    ) d) as duplicate_username_same_tenant_groups,
    (select count(*) from (
      select id
      from system_users
      where status = 0 and deleted = 0
      group by id
      having count(*) > 1
    ) d) as duplicate_generated_login_groups
) s
"@
$summary = $summaryJson | ConvertFrom-Json

$tenants = ConvertFrom-JsonLines (Invoke-YudaoSql @"
select row_to_json(t)::text
from (
  select
    t.id,
    t.name,
    t.status,
    t.package_id,
    t.expire_time,
    count(u.id) as active_users,
    count(u.id) filter (where coalesce(u.email, '') = '') as empty_email_users,
    'yudao-' || t.id || '-' || regexp_replace(t.name, '[^a-zA-Z0-9_-]+', '-', 'g') as target_org_name
  from system_tenant t
  left join system_users u on u.tenant_id = t.id
    and u.status = 0
    and u.deleted = 0
  where t.status = 0
    and t.deleted = 0
  group by t.id, t.name, t.status, t.package_id, t.expire_time
  order by active_users desc, t.id
) t
"@)

$duplicateUsernames = ConvertFrom-JsonLines (Invoke-YudaoSql @"
select row_to_json(d)::text
from (
  select tenant_id, username, count(*) as users
  from system_users
  where status = 0 and deleted = 0
  group by tenant_id, username
  having count(*) > 1
  order by users desc, tenant_id, username
) d
"@)

$users = ConvertFrom-JsonLines (Invoke-YudaoSql @"
select row_to_json(u)::text
from (
  select
    u.id as yudao_user_id,
    u.tenant_id as yudao_tenant_id,
    u.username as yudao_username,
    u.nickname,
    u.email as source_email,
    coalesce(u.email, '') = '' as needs_synthetic_email,
    case
      when coalesce(u.email, '') = '' then 'yudao-u' || u.id || '-t' || u.tenant_id || '@migration.localhost'
      else u.email
    end as target_email,
    'yudao-u' || u.id || '-t' || u.tenant_id as target_login_name,
    u.dept_id
  from system_users u
  where u.status = 0
    and u.deleted = 0
  order by u.tenant_id, u.id
) u
"@)

$report = [ordered]@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    source = [ordered]@{
        kubeNamespace = $KubeNamespace
        postgresDeployment = $PostgresDeployment
        database = $Database
    }
    target = [ordered]@{
        system = "local-zitadel"
        defaultBaseUrl = "http://localhost:8080"
    }
    strategy = [ordered]@{
        organization = "One active Yudao tenant becomes one ZITADEL organization."
        user = "One active non-deleted Yudao admin user becomes one ZITADEL human user."
        loginName = "yudao-u<system_users.id>-t<system_users.tenant_id>"
        emptyEmail = "yudao-u<system_users.id>-t<tenant_id>@migration.localhost"
        password = "Import existing BCrypt hash with algorithm=bcrypt."
        metadata = @("yudao_tenant_id", "yudao_user_id", "yudao_username", "yudao_dept_id")
    }
    summary = $summary
    tenants = $tenants
    duplicateUsernames = $duplicateUsernames
    users = $users
}

$dir = Split-Path -Parent $OutputPath
if ($dir) {
    New-Item -ItemType Directory -Force $dir | Out-Null
}

$json = $report | ConvertTo-Json -Depth 30
Set-Content -Path $OutputPath -Value $json -Encoding UTF8
$json
