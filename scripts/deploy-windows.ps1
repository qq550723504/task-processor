# Windows Auto Deploy Script
# Build and upload to Tencent Cloud COS

param(
    [Parameter(Mandatory=$true)]
    [string]$Version,
    
    [string]$Changelog = "Bug fixes and improvements",
    [string]$CosBucket = "auto-update-1303159911",
    [string]$CosRegion = "ap-shanghai",
    [string]$CosPath = "task-processor",
    [switch]$ForceUpdate = $false,
    [switch]$AutoUpload = $true
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Task Processor Windows Deploy Script" -ForegroundColor Cyan
Write-Host "Version: $Version" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 1. Build Windows version
Write-Host "`n[1/5] Building..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

# Create dist directory if not exists
if (-not (Test-Path "dist")) {
    New-Item -ItemType Directory -Path "dist" | Out-Null
}

# Inject version
$buildTime = Get-Date -Format 'yyyy-MM-dd_HH:mm:ss'
$ldflags = "-X main.appVersion=$Version -X main.buildTime=$buildTime"

Write-Host "Debug: Version = $Version" -ForegroundColor Magenta
Write-Host "Debug: BuildTime = $buildTime" -ForegroundColor Magenta
Write-Host "Debug: ldflags = $ldflags" -ForegroundColor Magenta

go build -ldflags $ldflags -o "dist/task-processor.exe" ./cmd/task

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

Write-Host "Build completed" -ForegroundColor Green

# 2. Calculate file hash
Write-Host "`n[2/5] Calculating hash..." -ForegroundColor Yellow
$hash = Get-FileHash -Path "dist/task-processor.exe" -Algorithm SHA256
$sha256 = $hash.Hash.ToLower()
Write-Host "SHA256: $sha256" -ForegroundColor Cyan

# 3. Generate version info
Write-Host "`n[3/5] Generating version info..." -ForegroundColor Yellow

# 处理版本号格式，避免重复前缀
$cleanVersion = $Version
if ($Version.StartsWith("task-processor-")) {
    $cleanVersion = $Version.Substring("task-processor-".Length)
}

$cosUrl = "https://$CosBucket.cos.$CosRegion.myqcloud.com"
$downloadUrl = "$cosUrl/$CosPath/task-processor-$cleanVersion.exe"

$versionInfo = @{
    version = $Version
    release_date = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ssZ")
    download_url = $downloadUrl
    sha256 = $sha256
    changelog = $Changelog
    force_update = $ForceUpdate.IsPresent
}

$json = $versionInfo | ConvertTo-Json -Depth 10
# Write without BOM
[System.IO.File]::WriteAllText("$PWD\dist\version.json", $json, [System.Text.UTF8Encoding]::new($false))
Write-Host "Version info generated" -ForegroundColor Green
Write-Host "Download URL: $downloadUrl" -ForegroundColor Cyan

# 4. Copy file
Write-Host "`n[4/5] Preparing files..." -ForegroundColor Yellow
Copy-Item "dist/task-processor.exe" "dist/task-processor-$cleanVersion.exe"

# 5. Upload to COS
Write-Host "`n[5/5] Uploading to Tencent Cloud COS..." -ForegroundColor Yellow

if ($AutoUpload) {
    if (Get-Command coscmd -ErrorAction SilentlyContinue) {
        Write-Host "Using coscmd to upload..." -ForegroundColor Cyan
        
        try {
            Write-Host "Uploading program file..." -ForegroundColor Yellow
            coscmd upload "dist/task-processor-$cleanVersion.exe" "$CosPath/task-processor-$cleanVersion.exe"
            
            Write-Host "Updating version info..." -ForegroundColor Yellow
            coscmd upload "dist/version.json" "$CosPath/version.json" -f
            
            Write-Host "Upload completed!" -ForegroundColor Green
            Write-Host "Access URL: $cosUrl/$CosPath/version.json" -ForegroundColor Cyan
        }
        catch {
            Write-Host "Upload failed: $_" -ForegroundColor Red
            Write-Host "Please check coscmd config or upload manually" -ForegroundColor Yellow
        }
    }
    else {
        Write-Host "coscmd not found, please install it first" -ForegroundColor Yellow
        Write-Host "`nInstall: pip install coscmd" -ForegroundColor Cyan
        Write-Host "Config: coscmd config -a <SecretId> -s <SecretKey> -b $CosBucket -r $CosRegion" -ForegroundColor Cyan
    }
}
else {
    Write-Host "Auto upload skipped" -ForegroundColor Yellow
}

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "Deploy completed!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "`nNext steps:" -ForegroundColor Cyan
Write-Host "1. Files uploaded to COS" -ForegroundColor White
Write-Host "2. Clients will auto-update within 1 hour" -ForegroundColor White
Write-Host "3. Use Redis to trigger immediate update (optional)" -ForegroundColor White

# # 基本用法
# .\scripts\deploy-windows.ps1 -Version "2.8.8"

# # 完整参数
# .\scripts\deploy-windows.ps1 `
#     -Version "v1.0.1" `
#     -Changelog "修复了登录问题" `
#     -ForceUpdate `
#     -CosBucket "your-bucket" `
#     -CosRegion "ap-shanghai"

# # 只构建不上传
# .\scripts\deploy-windows.ps1 -Version "v1.0.0" -AutoUpload:$false
