param(
    [string]$TaskNamespace = "task-processor",
    [string]$LoginNamespace = "yudao-cloud",
    [string]$LoginConfigSecret = "login-config",
    [string]$DatabaseSecret = "shein-listing-secret",
    [string]$PostgresNamespace = "yudao-cloud",
    [string]$PostgresTarget = "deploy/postgresql",
    [string]$ListenIp = "10.42.0.1",
    [int]$Start = 4,
    [int]$End = 254,
    [int]$PortBase = 31000,
    [switch]$AllSheinStores,
    [switch]$Reassign,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

function Decode-SecretValue {
    param(
        [Parameter(Mandatory = $true)]$Data,
        [Parameter(Mandatory = $true)][string]$Name
    )
    if (-not $Data.$Name) {
        throw "secret value missing: $Name"
    }
    [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Data.$Name))
}

$loginSecret = kubectl -n $LoginNamespace get secret $LoginConfigSecret -o json | ConvertFrom-Json
$configText = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($loginSecret.data.'config.yaml'))
$configPath = Join-Path $env:TEMP "login-config-shein-proxies.yaml"
[IO.File]::WriteAllText($configPath, $configText, [Text.UTF8Encoding]::new($false))

$dbSecret = kubectl -n $TaskNamespace get secret $DatabaseSecret -o json | ConvertFrom-Json
$dbData = $dbSecret.data
$dbName = Decode-SecretValue $dbData "TASK_PROCESSOR_DATABASE_NAME"
$dbUser = Decode-SecretValue $dbData "TASK_PROCESSOR_DATABASE_USER"
$dbPassword = Decode-SecretValue $dbData "TASK_PROCESSOR_DATABASE_PASSWORD"

$where = "lower(platform) = 'shein'"
if (-not $AllSheinStores) {
    $where += " and enable_auto_listing = true"
}

$storeQuery = @"
select coalesce(json_agg(row_to_json(t))::text, '[]')
from (
  select id, tenant_id, name, proxy
  from listing_store
  where $where
  order by tenant_id, id
) t;
"@

$storesJson = kubectl -n $PostgresNamespace exec $PostgresTarget -- env PGPASSWORD=$dbPassword psql -U $dbUser -d $dbName -At -c $storeQuery
if ($LASTEXITCODE -ne 0) {
    throw "failed to read SHEIN stores from listing_store"
}

$storesPath = Join-Path $env:TEMP "shein-stores-for-proxy-sync.json"
($storesJson -join "`n") | Set-Content -Path $storesPath -Encoding utf8

$planScript = @'
import json
import sys
from pathlib import Path

import yaml

config_path = Path(sys.argv[1])
stores_path = Path(sys.argv[2])
listen_ip = sys.argv[3]
start = int(sys.argv[4])
end = int(sys.argv[5])
port_base = int(sys.argv[6])
reassign = sys.argv[7].lower() == "true"

cfg = yaml.safe_load(config_path.read_text(encoding="utf-8-sig")) or {}
stores = json.loads(stores_path.read_text(encoding="utf-8-sig"))
shein = cfg.setdefault("platforms", {}).setdefault("shein", {})
shop_map = shein.setdefault("shop_proxy_map", {})
if reassign:
    shop_map.clear()

pool = [f"http://{listen_ip}:{port_base + last}" for last in range(start, end + 1)]
pool_set = set(pool)
used = {proxy for proxy in shop_map.values() if proxy in pool_set}
available = [proxy for proxy in pool if proxy not in used]

updates = []
assigned = []
kept = []

for store in stores:
    tenant_id = str(store.get("tenant_id") or "").strip()
    store_id = str(store.get("id") or "").strip()
    if not tenant_id or not store_id:
        continue
    key = f"{tenant_id}:{store_id}"
    proxy = shop_map.get(key)
    if not proxy:
        if not available:
            raise RuntimeError("proxy pool exhausted")
        proxy = available.pop(0)
        shop_map[key] = proxy
        assigned.append({"key": key, "name": store.get("name") or "", "proxy": proxy})
    else:
        kept.append({"key": key, "name": store.get("name") or "", "proxy": proxy})
    if (store.get("proxy") or "").strip() != proxy:
        updates.append({"id": int(store_id), "tenant_id": int(tenant_id), "name": store.get("name") or "", "proxy": proxy})

def sql_quote(value):
    return "'" + str(value).replace("'", "''") + "'"

if updates:
    values = ",\n".join(
        f"({item['id']}, {item['tenant_id']}, {sql_quote(item['proxy'])})"
        for item in updates
    )
    sql = f"""update listing_store as s
set proxy = v.proxy
from (values
{values}
) as v(id, tenant_id, proxy)
where s.id = v.id
  and s.tenant_id = v.tenant_id
  and lower(s.platform) = 'shein';"""
else:
    sql = "-- no listing_store proxy changes required"

config_path.write_text(yaml.safe_dump(cfg, allow_unicode=True, sort_keys=False), encoding="utf-8")
print(json.dumps({
    "store_count": len(stores),
    "kept_count": len(kept),
    "assigned_count": len(assigned),
    "update_count": len(updates),
    "kept": kept,
    "assigned": assigned,
    "updates": updates,
    "sql": sql,
}, ensure_ascii=False))
'@

$planJson = $planScript | python - $configPath $storesPath $ListenIp $Start $End $PortBase $Reassign.IsPresent.ToString().ToLower()
if ($LASTEXITCODE -ne 0) {
    throw "failed to build proxy sync plan"
}
$plan = $planJson | ConvertFrom-Json

Write-Host "SHEIN proxy sync plan" -ForegroundColor Cyan
Write-Host "  stores:   $($plan.store_count)"
Write-Host "  kept:     $($plan.kept_count)"
Write-Host "  assigned: $($plan.assigned_count)"
Write-Host "  updates:  $($plan.update_count)"

if ($plan.assigned_count -gt 0) {
    Write-Host ""
    Write-Host "New assignments:" -ForegroundColor Yellow
    $plan.assigned | Format-Table key, name, proxy -AutoSize
}

if ($DryRun) {
    Write-Host ""
    Write-Host "Dry run only; no Kubernetes secret or database changes were applied." -ForegroundColor Yellow
    exit 0
}

$sqlPath = Join-Path $env:TEMP "sync-shein-store-proxies.sql"
[IO.File]::WriteAllText($sqlPath, $plan.sql, [Text.UTF8Encoding]::new($false))

kubectl -n $LoginNamespace create secret generic $LoginConfigSecret --from-file=config.yaml=$configPath --dry-run=client -o yaml | kubectl apply -f -
if ($LASTEXITCODE -ne 0) {
    throw "failed to apply $LoginConfigSecret"
}

Get-Content $sqlPath | kubectl -n $PostgresNamespace exec -i $PostgresTarget -- env PGPASSWORD=$dbPassword psql -U $dbUser -d $dbName
if ($LASTEXITCODE -ne 0) {
    throw "failed to update listing_store.proxy"
}

kubectl -n $LoginNamespace rollout restart deploy/login
kubectl -n $LoginNamespace rollout status deploy/login --timeout=300s

Write-Host ""
Write-Host "SHEIN proxy sync finished." -ForegroundColor Green
