$scriptPath = Join-Path $PSScriptRoot 'source-account-ownership-rehearsal.ps1'

Describe 'Source Account ownership rehearsal wrapper' {
    BeforeAll {
        $scriptText = Get-Content -LiteralPath $scriptPath -Raw
    }

    It 'builds only the dedicated command into ignored local state' {
        $scriptText | Should Match ([regex]::Escape("'.local/bin'"))
        $scriptText | Should Match ([regex]::Escape('./cmd/source-account-ownership-rehearsal'))
    }

    It 'runs only the explicit fixed rehearsal action' {
        $scriptText | Should Match '& \$binaryPath rehearsal --yes'
        $scriptText | Should Not Match 'Read-Host|SourceDSN|MetadataDSN|ProfileRoot|OrganizationId|tenant'
    }

    It 'propagates the command exit class' {
        $scriptText | Should Match 'exit \$rehearsalExitCode'
    }
}
