[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string[]]$Hosts,

    [string]$User = "root",

    [Parameter(Mandatory = $true)]
    [string]$Password,

    [string]$Server = "https://101.33.34.102:6443",

    [Parameter(Mandatory = $true)]
    [string]$Token,

    [string]$Version = "v1.33.4+k3s1",

    [switch]$LabelAgent
)

$ErrorActionPreference = "Stop"

$pythonScript = @'
import argparse
from pathlib import Path
import paramiko

parser = argparse.ArgumentParser()
parser.add_argument("--user", required=True)
parser.add_argument("--password", required=True)
parser.add_argument("--server", required=True)
parser.add_argument("--token", required=True)
parser.add_argument("--version", required=True)
parser.add_argument("--hosts", nargs="+", required=True)
args = parser.parse_args()

pubkey = Path.home().joinpath(".ssh", "id_ed25519.pub").read_text(encoding="utf-8").strip()

for host in args.hosts:
    print(f"=== {host} ===")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(
        hostname=host,
        username=args.user,
        password=args.password,
        timeout=20,
        banner_timeout=30,
        auth_timeout=30,
        look_for_keys=False,
        allow_agent=False,
    )

    install_key = (
        "umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; "
        f"grep -qxF '{pubkey}' ~/.ssh/authorized_keys || echo '{pubkey}' >> ~/.ssh/authorized_keys; "
        "chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys"
    )
    stdin, stdout, stderr = client.exec_command(install_key, get_pty=True)
    stdout.channel.recv_exit_status()

    join_cmd = f"""
set -e
(test -x /usr/local/bin/k3s-uninstall.sh && /usr/local/bin/k3s-uninstall.sh) || true
(test -x /usr/local/bin/k3s-agent-uninstall.sh && /usr/local/bin/k3s-agent-uninstall.sh) || true
systemctl disable k3s 2>/dev/null || true
systemctl disable k3s-agent 2>/dev/null || true
systemctl reset-failed 2>/dev/null || true
rm -f /etc/systemd/system/k3s.service /etc/systemd/system/k3s.service.env /etc/systemd/system/k3s-agent.service /etc/systemd/system/k3s-agent.service.env
rm -rf /etc/rancher /var/lib/rancher /var/lib/kubelet /etc/cni /var/lib/cni /run/flannel
mkdir -p /etc/rancher/k3s
cat > /etc/rancher/k3s/config.yaml <<EOF
server: {args.server}
token: {args.token}
node-external-ip: {host}
with-node-id: true
EOF
curl -sfL https://get.k3s.io -o /tmp/get-k3s.sh
INSTALL_K3S_VERSION='{args.version}' INSTALL_K3S_EXEC='agent' sh /tmp/get-k3s.sh > /tmp/codex-k3s-install.log 2>&1
systemctl is-active k3s-agent
hostname
"""

    stdin, stdout, stderr = client.exec_command(join_cmd, get_pty=True)
    code = stdout.channel.recv_exit_status()
    out = stdout.read().decode(errors="ignore")
    err = stderr.read().decode(errors="ignore")
    print(out.strip())
    if err.strip():
        print(err.strip())
    if code != 0:
        raise RuntimeError(f"{host} failed to join")

    client.close()
'@

$scriptPath = Join-Path $env:TEMP "codex-join-k3s-agents.py"
Set-Content -Path $scriptPath -Value $pythonScript -Encoding ASCII

foreach ($host in $Hosts) {
    ssh-keygen -R $host | Out-Null
}

$argList = @(
    $scriptPath,
    "--user", $User,
    "--password", $Password,
    "--server", $Server,
    "--token", $Token,
    "--version", $Version,
    "--hosts"
) + $Hosts

python @argList

if ($LabelAgent) {
    Start-Sleep -Seconds 5
    foreach ($host in $Hosts) {
        $node = kubectl --context=default get nodes -o jsonpath="{range .items[?(@.status.addresses[?(@.type=='ExternalIP')].address=='$host')]}{.metadata.name}{end}"
        if ($node) {
            kubectl --context=default label node $node node-role.kubernetes.io/agent=true --overwrite | Out-Null
        }
    }
}
