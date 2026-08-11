param(
    [string] $KubeNamespace = "yudao-cloud",
    [string] $PostgresDeployment = "deploy/postgresql",
    [string] $Database = "ruoyi-vue-pro",
    [string] $DbUser = "postgres",
    [string] $MetadataNamespace = $KubeNamespace,
    [string] $MetadataPostgresDeployment = $PostgresDeployment,
    [string] $MetadataDatabase = "zitadel_auth",
    [string] $MetadataDbUser = $DbUser,
    [string] $OutputPath = ".local/tmp/yudao-zitadel-owner-recovery-dry-run.json"
)

$ErrorActionPreference = "Stop"

function Invoke-PostgresSql {
    param(
        [Parameter(Mandatory = $true)][string] $Namespace,
        [Parameter(Mandatory = $true)][string] $Target,
        [Parameter(Mandatory = $true)][string] $TargetDatabase,
        [Parameter(Mandatory = $true)][string] $TargetDbUser,
        [Parameter(Mandatory = $true)][string] $Sql
    )
    $result = kubectl -n $Namespace exec $Target -- `
        psql -U $TargetDbUser -d $TargetDatabase -tA -c $Sql
    if ($LASTEXITCODE -ne 0) {
        throw "PostgreSQL recovery query failed."
    }
    return (($result -join "`n").Trim())
}

function ConvertFrom-JsonLines {
    param([AllowNull()][string] $Text)
    $items = @()
    if ([string]::IsNullOrWhiteSpace($Text)) {
        return $items
    }
    foreach ($line in ($Text -split "`n")) {
        if (-not [string]::IsNullOrWhiteSpace($line)) {
            $items += ($line | ConvertFrom-Json)
        }
    }
    return $items
}

function Get-Fingerprint {
    param([AllowNull()][string] $Value)
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes(([string]$Value).Trim())
        return "sha256:" + (([BitConverter]::ToString($sha.ComputeHash($bytes)) -replace '-', '').Substring(0, 12).ToLowerInvariant())
    } finally {
        $sha.Dispose()
    }
}

# The migration writes these keys for active, non-deleted users. This report
# widens the source query to inactive/deleted users but never creates a user or
# invents a subject; operators must resolve the returned legacy IDs explicitly.
$users = ConvertFrom-JsonLines (Invoke-PostgresSql -Namespace $KubeNamespace -Target $PostgresDeployment -TargetDatabase $Database -TargetDbUser $DbUser -Sql @"
select row_to_json(u)::text
from (
  select tenant_id::text as yudao_tenant_id, id::text as yudao_user_id, username, status, deleted
  from system_users
  order by tenant_id, id
) u
"@)
$metadata = ConvertFrom-JsonLines (Invoke-PostgresSql -Namespace $MetadataNamespace -Target $MetadataPostgresDeployment -TargetDatabase $MetadataDatabase -TargetDbUser $MetadataDbUser -Sql @"
select row_to_json(m)::text
from (
  select
    user_id::text as zitadel_user_id,
    max(convert_from(value, 'UTF8')) filter (where key = 'yudao_tenant_id') as yudao_tenant_id,
    max(convert_from(value, 'UTF8')) filter (where key = 'yudao_user_id') as yudao_user_id
  from projections.user_metadata5
  where key in ('yudao_tenant_id', 'yudao_user_id')
  group by user_id
  having count(*) filter (where key = 'yudao_tenant_id') > 0
     and count(*) filter (where key = 'yudao_user_id') > 0
) m
"@)
$metadataByLegacyKey = @{}
foreach ($item in $metadata) {
    $metadataByLegacyKey["$($item.yudao_tenant_id):$($item.yudao_user_id)"] = $item.zitadel_user_id
}
$rows = @($users | ForEach-Object {
    $key = "$($_.yudao_tenant_id):$($_.yudao_user_id)"
    [pscustomobject]@{
        yudao_tenant_id = $_.yudao_tenant_id
        yudao_user_id = $_.yudao_user_id
        status = $_.status
        deleted = $_.deleted
        mapped_subject_count = if ($metadataByLegacyKey.ContainsKey($key)) { 1 } else { 0 }
    }
})

$missing = @($rows | Where-Object { [int]$_.mapped_subject_count -eq 0 })
$mapped = @($rows | Where-Object { [int]$_.mapped_subject_count -gt 0 })
$inactiveOrDeleted = @($rows | Where-Object { [int]$_.status -ne 0 -or [int]$_.deleted -ne 0 })

$report = [ordered]@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    source = [ordered]@{
        kubeNamespace = $KubeNamespace
        postgresDeployment = $PostgresDeployment
        database = $Database
        metadataKeys = @("yudao_tenant_id", "yudao_user_id")
    }
    policy = [ordered]@{
        subjectSource = "existing ZITADEL user metadata only"
        missingSubjectAction = "block and require explicit migration recovery"
        inactiveOrDeletedAction = "report only; do not recreate or assign automatically"
    }
    summary = [ordered]@{
        candidateRows = $rows.Count
        missingSubjectRows = $missing.Count
        mappedSubjectRows = $mapped.Count
        inactiveOrDeletedRows = $inactiveOrDeleted.Count
    }
    missingSubjects = @($missing | ForEach-Object {
        [ordered]@{
            yudaoTenantFingerprint = Get-Fingerprint $_.yudao_tenant_id
            yudaoUserFingerprint = Get-Fingerprint $_.yudao_user_id
            status = $_.status
            deleted = $_.deleted
        }
    })
}

$dir = Split-Path -Parent $OutputPath
if ($dir) {
    New-Item -ItemType Directory -Force $dir | Out-Null
}
$json = $report | ConvertTo-Json -Depth 20
Set-Content -Path $OutputPath -Value $json -Encoding UTF8
$json
