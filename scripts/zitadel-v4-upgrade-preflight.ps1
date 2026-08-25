[CmdletBinding()] param([string]$Namespace = "zitadel", [string]$TargetVersion = "v4.17.1")
function Get-SnapshotFromJson($d) {
  $c = @($d.spec.template.spec.containers | Where-Object name -eq $d.metadata.name)[0]
  [pscustomobject]@{ image=[string]$c.image; ready=("{0}/{1}" -f $d.status.readyReplicas,$d.status.replicas); generation=[int64]$d.metadata.generation }
}
# Compare the deployed tag exactly: a substring match would accept e.g.
# v4.17.10 or v4.17.1-unapproved as satisfying the approved v4.17.1 release.
function Get-ExactImageTag([string]$imageRef) {
  if ([string]::IsNullOrWhiteSpace($imageRef)) { return "" }
  $namePart = ($imageRef -split '@')[0]
  $lastSlash = $namePart.LastIndexOf('/')
  if ($lastSlash -ge 0) { $namePart = $namePart.Substring($lastSlash + 1) }
  $colon = $namePart.IndexOf(':')
  if ($colon -lt 0) { return "latest" }
  return $namePart.Substring($colon + 1)
}
$core = Get-SnapshotFromJson (kubectl -n $Namespace get deployment zitadel -o json | ConvertFrom-Json)
$login = Get-SnapshotFromJson (kubectl -n $Namespace get deployment zitadel-login -o json | ConvertFrom-Json)
$coreTag = Get-ExactImageTag $core.image
$loginTag = Get-ExactImageTag $login.image
[pscustomobject]@{ coreImage=$core.image; loginImage=$login.image; coreTag=$coreTag; loginTag=$loginTag; coreReady=$core.ready; loginReady=$login.ready; coreGeneration=$core.generation; loginGeneration=$login.generation; targetVersion=$TargetVersion; upgradeRequired=($coreTag -ne $TargetVersion) -or ($loginTag -ne $TargetVersion) } | ConvertTo-Json -Compress
