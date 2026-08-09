$scriptPath = Join-Path $PSScriptRoot "start-listingkit-local-dev.ps1"

function Get-StartDevScriptAst {
    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        throw "Unable to parse ${scriptPath}: $($errors[0].Message)"
    }
    return $ast
}

Describe "start-listingkit-local-dev authentication" {
    It "does not expose API or UI authentication bypass parameters" {
        $ast = Get-StartDevScriptAst
        $paramBlock = $ast.ParamBlock

        $bypassParameter = $paramBlock.Parameters |
            Where-Object { $_.Name.VariablePath.UserPath -eq "BypassAuthGate" } |
            Select-Object -First 1
        $zitadelModeParameter = $paramBlock.Parameters |
            Where-Object { $_.Name.VariablePath.UserPath -eq "ZitadelAuthMode" } |
            Select-Object -First 1

        $bypassParameter | Should Be $null
        $zitadelModeParameter | Should Be $null
    }
}
