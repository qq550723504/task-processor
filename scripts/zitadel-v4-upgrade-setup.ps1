[CmdletBinding()]
param(
  [string]$Namespace = 'zitadel',
  [string]$JobName = 'zitadel-v4-setup',
  [switch]$Apply
)

$ErrorActionPreference = 'Stop'

$deployment = kubectl -n $Namespace get deployment zitadel -o json | ConvertFrom-Json
if ([int]$deployment.spec.replicas -ne 0) {
  throw 'Scale the ZITADEL runtime deployment to zero before running setup.'
}

$podSpec = $deployment.spec.template.spec
$podSpec.restartPolicy = 'Never'
$container = @($podSpec.containers | Where-Object { $_.name -eq 'zitadel' })[0]
if ($null -eq $container) {
  throw 'The ZITADEL runtime container was not found in the deployment template.'
}

$container.args = @('setup', '--masterkeyFromEnv', '--init-projections=true')
$container.PSObject.Properties.Remove('livenessProbe')
$container.PSObject.Properties.Remove('readinessProbe')
$container.PSObject.Properties.Remove('startupProbe')

$job = [ordered]@{
  apiVersion = 'batch/v1'
  kind = 'Job'
  metadata = [ordered]@{
    name = $JobName
    namespace = $Namespace
    labels = [ordered]@{
      'app.kubernetes.io/name' = 'zitadel-setup'
      'app.kubernetes.io/instance' = 'zitadel'
    }
  }
  spec = [ordered]@{
    backoffLimit = 0
    activeDeadlineSeconds = 900
    ttlSecondsAfterFinished = 86400
    template = [ordered]@{
      metadata = [ordered]@{
        labels = [ordered]@{
          'app.kubernetes.io/name' = 'zitadel-setup'
          'app.kubernetes.io/instance' = 'zitadel'
        }
      }
      spec = $podSpec
    }
  }
}

$manifest = $job | ConvertTo-Json -Depth 100
$manifest | kubectl apply --dry-run=server -f -
if ($LASTEXITCODE -ne 0) {
  throw 'The setup job did not pass server-side validation.'
}

if ($Apply) {
  $manifest | kubectl apply -f -
  if ($LASTEXITCODE -ne 0) {
    throw 'The setup job could not be created.'
  }
}

[pscustomobject]@{
  namespace = $Namespace
  jobName = $JobName
  image = $container.image
  args = @($container.args)
  applied = [bool]$Apply
} | ConvertTo-Json -Compress
