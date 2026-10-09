param(
 [Parameter(Mandatory=$true)][string]$CaptureAppUrl,
 [Parameter(Mandatory=$true)][string]$OutputDirectory
)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
$extensionRoot=Join-Path $taskRoot 'extensions/1688-capture'
$destination=[System.IO.Path]::GetFullPath($OutputDirectory)
$previousCaptureUrl=$env:CAPTURE_APP_URL
try {
 $env:CAPTURE_APP_URL=$CaptureAppUrl
 Push-Location $extensionRoot
 try { npm.cmd run build; if($LASTEXITCODE -ne 0){throw 'Plugin build failed'} } finally { Pop-Location }
 New-Item -ItemType Directory -Path $destination -Force | Out-Null
 $files=@('manifest.json','background.js','popup.js','extractor.js','popup.html','popup.css') | ForEach-Object {Join-Path $extensionRoot "dist/$_"}
 $archive=Join-Path $destination 'shuomi-1688-capture.zip'
 Compress-Archive -LiteralPath $files -DestinationPath $archive -Force
 $release=[ordered]@{schemaVersion=1;captureAppUrl=([Uri]$CaptureAppUrl).AbsoluteUri;sha256=(Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant();filename='shuomi-1688-capture.zip'}
 [System.IO.File]::WriteAllText((Join-Path $destination 'release.json'),($release | ConvertTo-Json),[System.Text.UTF8Encoding]::new($false))
 Write-Output "Plugin release: $archive"
 Write-Output "Trusted build record: $(Join-Path $destination 'release.json')"
} finally {$env:CAPTURE_APP_URL=$previousCaptureUrl}
