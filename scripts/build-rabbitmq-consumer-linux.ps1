# RabbitMQ Consumer Linux 编译脚本

$ErrorActionPreference = "Stop"

# 版本信息
$version = git describe --tags --always --dirty 2>$null
if (-not $version) { $version = "v1.0.0" }
$buildTime = Get-Date -Format 'yyyy-MM-dd_HH:mm:ss'
$ldflags = "-X main.appVersion=$version -X main.buildTime=$buildTime"

$binDir = "bin"
$output = "$binDir/rabbitmq-consumer"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "RabbitMQ Consumer Linux Build" -ForegroundColor Cyan
Write-Host "Version: $version" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 创建输出目录
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

# 编译
Write-Host "`nBuilding..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"

go build -ldflags $ldflags -o $output cmd/rabbitmq-consumer/main.go

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

$fileSize = [math]::Round((Get-Item $output).Length / 1KB, 1)
Write-Host "`nBuild succeeded!" -ForegroundColor Green
Write-Host "Output: $output" -ForegroundColor Green
Write-Host "Size: $fileSize KB" -ForegroundColor Green
