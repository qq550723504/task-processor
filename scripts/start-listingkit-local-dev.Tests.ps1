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

Describe "start-listingkit-local-dev auth defaults" {
    It "defaults the local dev stack to bypass ZITADEL auth" {
        $ast = Get-StartDevScriptAst
        $paramBlock = $ast.ParamBlock

        $bypassParameter = $paramBlock.Parameters |
            Where-Object { $_.Name.VariablePath.UserPath -eq "BypassAuthGate" } |
            Select-Object -First 1
        $zitadelModeParameter = $paramBlock.Parameters |
            Where-Object { $_.Name.VariablePath.UserPath -eq "ZitadelAuthMode" } |
            Select-Object -First 1

        $bypassParameter | Should Not Be $null
        $bypassParameter.DefaultValue.Extent.Text | Should Be '"1"'
        $zitadelModeParameter | Should Not Be $null
        $zitadelModeParameter.DefaultValue.Extent.Text | Should Be '"Disabled"'
    }

    It "passes the selected ZITADEL auth mode to the local API starter" {
        $content = Get-Content -LiteralPath $scriptPath -Raw

        $content | Should Match 'ZitadelAuthMode'
        $content | Should Match '-ZitadelAuthMode'
    }
}
