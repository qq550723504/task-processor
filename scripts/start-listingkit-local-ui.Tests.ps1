$scriptPath = Join-Path $PSScriptRoot "start-listingkit-local-ui.ps1"

function Get-StartUiScriptAst {
    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        throw "Unable to parse ${scriptPath}: $($errors[0].Message)"
    }
    return $ast
}

Describe "start-listingkit-local-ui auth defaults" {
    It "enables the local auth gate bypass by default" {
        $ast = Get-StartUiScriptAst
        $paramBlock = $ast.ParamBlock
        $parameter = $paramBlock.Parameters |
            Where-Object { $_.Name.VariablePath.UserPath -eq "BypassAuthGate" } |
            Select-Object -First 1

        $parameter | Should Not Be $null
        $parameter.DefaultValue.Extent.Text | Should Be '"1"'
    }
}
