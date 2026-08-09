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

Describe "start-listingkit-local-ui authentication" {
    It "does not expose a local auth gate bypass" {
        $ast = Get-StartUiScriptAst
        $paramBlock = $ast.ParamBlock
        $parameter = $paramBlock.Parameters |
            Where-Object { $_.Name.VariablePath.UserPath -eq "BypassAuthGate" } |
            Select-Object -First 1

        $parameter | Should Be $null
    }
}
