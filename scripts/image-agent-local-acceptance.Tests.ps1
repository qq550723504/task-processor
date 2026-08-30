$scriptPath = Join-Path $PSScriptRoot "image-agent-local-acceptance.ps1"

function Import-SeedFunction {
    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        throw "Unable to parse ${scriptPath}: $($errors[0].Message)"
    }
    $function = $ast.FindAll({
        param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq "Invoke-Seed"
    }, $true) | Select-Object -First 1
    if ($null -eq $function) {
        throw "Invoke-Seed was not found in ${scriptPath}"
    }
    $definition = $function.Extent.Text -replace '^function\s+Invoke-Seed', 'function global:Invoke-Seed'
    Invoke-Expression $definition
}

Describe "image-agent-local-acceptance seed routing" {
    BeforeEach {
        Import-SeedFunction
        $global:SourceUrl = "https://example.com/source.png"
        $global:StyleUrl = ""
        $global:TokenFile = Join-Path $TestDrive "token.txt"
        $global:runtimeFile = Join-Path $TestDrive "runtime.env"
        Set-Content -LiteralPath $global:TokenFile -Value "test-token"
        $global:capturedGoArguments = @()
        function global:Assert-LocalSourceUrl { param([string]$Url, [string]$Name) }
        function global:Invoke-GoCommand { param([string[]]$Arguments) $global:capturedGoArguments = $Arguments }
    }

    AfterEach {
        Remove-Item Function:\global:Invoke-Seed,Function:\global:Assert-LocalSourceUrl,Function:\global:Invoke-GoCommand -ErrorAction SilentlyContinue
        Remove-Variable SourceUrl,StyleUrl,TokenFile,runtimeFile,capturedGoArguments -Scope Global -ErrorAction SilentlyContinue
    }

    It "invokes the application-owned Image Agent seed command" {
        Invoke-Seed

        $global:capturedGoArguments[0] | Should Be "run"
        $global:capturedGoArguments[1] | Should Be "./internal/app/runtime/imageagentacceptance/cmd"
    }
}
