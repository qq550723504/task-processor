[CmdletBinding()] param([string]$Namespace = "zitadel", [string]$TargetVersion = "v4.17.1")
function Get-SnapshotFromJson($d) {
  $c = @($d.spec.template.spec.containers | Where-Object name -eq $d.metadata.name)[0]
  [pscustomobject]@{ image=[string]$c.image; ready=("{0}/{1}" -f $d.status.readyReplicas,$d.status.replicas); generation=[int64]$d.metadata.generation }
}
$core = Get-SnapshotFromJson (kubectl -n $Namespace get deployment zitadel -o json | ConvertFrom-Json)
$login = Get-SnapshotFromJson (kubectl -n $Namespace get deployment zitadel-login -o json | ConvertFrom-Json)
[pscustomobject]@{ coreImage=$core.image; loginImage=$login.image; coreReady=$core.ready; loginReady=$login.ready; coreGeneration=$core.generation; loginGeneration=$login.generation; targetVersion=$TargetVersion; upgradeRequired=(($core.image -notmatch [regex]::Escape($TargetVersion)) -or ($login.image -notmatch [regex]::Escape($TargetVersion))) } | ConvertTo-Json -Compress
