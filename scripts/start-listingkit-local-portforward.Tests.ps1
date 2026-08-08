$scriptPath = Join-Path $PSScriptRoot "start-listingkit-local-portforward.ps1"

function Get-ParameterDefaultText {
    param(
        [System.Management.Automation.Language.ParamBlockAst]$ParamBlock,
        [string]$Name
    )

    $parameter = $ParamBlock.Parameters |
        Where-Object { $_.Name.VariablePath.UserPath -eq $Name } |
        Select-Object -First 1

    if ($null -eq $parameter -or $null -eq $parameter.DefaultValue) {
        return $null
    }

    return $parameter.DefaultValue.Extent.Text.Trim('"')
}

Describe "start-listingkit-local-portforward defaults" {
    BeforeAll {
        $tokens = $null
        $errors = $null
        $scriptAst = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    }

    It "has no PowerShell parser errors" {
        $errors.Count | Should Be 0
    }

    It "targets the current shared database and Redis services" {
        (Get-ParameterDefaultText -ParamBlock $scriptAst.ParamBlock -Name "DbNamespace") | Should Be "platform-data"
        (Get-ParameterDefaultText -ParamBlock $scriptAst.ParamBlock -Name "DbService") | Should Be "shared-postgresql"
        (Get-ParameterDefaultText -ParamBlock $scriptAst.ParamBlock -Name "RedisNamespace") | Should Be "platform-data"
        (Get-ParameterDefaultText -ParamBlock $scriptAst.ParamBlock -Name "RedisService") | Should Be "redis"
    }
}
