# 数据库查询工具 - PowerShell 包装脚本
param(
    [string]$Table = "",
    [string]$Where = "",
    [string]$Fields = "*",
    [int]$Limit = 100,
    [string]$Format = "table"
)

if ([string]::IsNullOrWhiteSpace($Table)) {
    Write-Host "错误: 必须指定表名" -ForegroundColor Red
    Write-Host ""
    Write-Host "用法:" -ForegroundColor Cyan
    Write-Host "  .\scripts\db-query.ps1 -Table <表名> [-Where <条件>] [-Fields <字段>] [-Limit <数量>] [-Format <格式>]" -ForegroundColor White
    Write-Host ""
    Write-Host "示例:" -ForegroundColor Cyan
    Write-Host '  .\scripts\db-query.ps1 -Table shein_studio_sessions -Where "id=''batch-id''"' -ForegroundColor Gray
    Write-Host '  .\scripts\db-query.ps1 -Table listing_kit_tasks -Where "status=''pending''" -Fields task_id,status' -ForegroundColor Gray
    Write-Host '  .\scripts\db-query.ps1 -Table listing_kit_tasks -Format json' -ForegroundColor Gray
    exit 1
}

# 构建命令
$cmdArgs = @("--table", $Table)

if (-not [string]::IsNullOrWhiteSpace($Where)) {
    $cmdArgs += "--where", $Where
}

if ($Fields -ne "*") {
    $cmdArgs += "--fields", $Fields
}

if ($Limit -ne 100) {
    $cmdArgs += "--limit", $Limit.ToString()
}

if ($Format -ne "table") {
    $cmdArgs += "--format", $Format
}

# 执行 Go 程序
$scriptPath = Join-Path $PSScriptRoot "..\cmd\db-query\main.go"
go run $scriptPath @cmdArgs
