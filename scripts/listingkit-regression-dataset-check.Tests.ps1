$scriptPath = Join-Path $PSScriptRoot "listingkit-regression-dataset-check.ps1"
$powershellPath = (Get-Command powershell.exe -ErrorAction Stop).Source

function Invoke-RegressionManifestValidation {
    param([string]$Manifest)

    $output = & $powershellPath -NoProfile -ExecutionPolicy Bypass -File $scriptPath -DatasetPath $Manifest -ValidateOnly 2>&1
    return @{
        ExitCode = $LASTEXITCODE
        Output = ($output | Out-String)
    }
}

Describe "listingkit regression dataset check" {
    It "validates a controlled SHEIN preview manifest without a token or network request" {
        $manifestPath = New-TemporaryFile
        try {
            Set-Content -LiteralPath $manifestPath -Encoding UTF8 -Value @'
{
  "schema_version": 1,
  "cases": [
    {
      "id": "shein-ready-draft",
      "task_id": "controlled-task-42",
      "platform": "shein",
      "expected_preview_ready": true,
      "expected_submit_mode": "save_draft"
    }
  ]
}
'@

            $result = Invoke-RegressionManifestValidation -Manifest $manifestPath

            $result.ExitCode | Should Be 0
            $result.Output | Should Match "manifest is valid: 1 case"
        } finally {
            Remove-Item -LiteralPath $manifestPath -Force -ErrorAction SilentlyContinue
        }
    }

    It "rejects placeholder task IDs before a production request is possible" {
        $manifestPath = New-TemporaryFile
        try {
            Set-Content -LiteralPath $manifestPath -Encoding UTF8 -Value @'
{
  "schema_version": 1,
  "cases": [
    {
      "id": "template-case",
      "task_id": "REPLACE_WITH_CONTROLLED_TASK_ID",
      "platform": "shein",
      "expected_preview_ready": true,
      "expected_submit_mode": "save_draft"
    }
  ]
}
'@

            $result = Invoke-RegressionManifestValidation -Manifest $manifestPath

            $result.ExitCode | Should Be 1
            $result.Output | Should Match "placeholder task_id"
        } finally {
            Remove-Item -LiteralPath $manifestPath -Force -ErrorAction SilentlyContinue
        }
    }
}
