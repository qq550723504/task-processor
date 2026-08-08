$scriptPath = Join-Path $PSScriptRoot "start-listingkit-api-local-replay.ps1"

Describe "start-listingkit-api-local-replay repository root" {
    It "resolves and uses the repository root from the script location" {
        $content = Get-Content -LiteralPath $scriptPath -Raw

        $content | Should Match '\$PSScriptRoot'
        $content | Should Match '\$repoRoot'
        $content | Should Match 'Set-Location\s+\$repoRoot'
        $content | Should Not Match 'D:\\code\\task-processor'
    }

    It "has no PowerShell parser errors" {
        $tokens = $null
        $errors = $null
        [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors) | Out-Null

        $errors.Count | Should Be 0
    }
}
