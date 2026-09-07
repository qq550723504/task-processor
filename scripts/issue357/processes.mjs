import {spawn} from 'node:child_process';
import {childEnvironment} from './contract.mjs';
import {processIdentity,pause,run} from './io.mjs';

// Register each child before starting the next. Keep the actual child handle
// until shutdown, including when identity inspection or persistence fails.
export async function startChildren(specs,register,children=[]) {
 for(const spec of specs) {
  const child=spawn(spec.command,spec.args,{windowsHide:true,stdio:'ignore',cwd:spec.cwd,env:{...childEnvironment(),...spec.env}});
  children.push(child);
  await new Promise((resolve,reject)=>{child.once('spawn',resolve);child.once('error',()=>reject(new Error('SERVICE_START_FAILED')))});
  const identity=await processIdentity(child.pid);
  if(!identity)throw new Error('SERVICE_EXITED_DURING_START');
  await register(spec.name,identity);
 }
 return children;
}
export async function terminateChildren(children) {
 for(const child of children)if(child.pid&&child.exitCode===null&&child.signalCode===null)child.kill();
 const end=Date.now()+10000;
 while(children.some(c=>c.pid&&c.exitCode===null&&c.signalCode===null)&&Date.now()<end)await pause(50);
 if(children.some(c=>c.pid&&c.exitCode===null&&c.signalCode===null))throw new Error('CHILD_STOP_FAILED');
}

// Recover the narrow spawn->registry crash window. Query only exact run-owned
// executable/argument paths, then capture creation time for later PID-reuse checks.
// Command lines are inspected locally and are never returned or logged.
export async function discoverRunProcesses(m) {
 const script="$ErrorActionPreference='Stop'; $spec=Get-Content -Raw -LiteralPath $env:ISSUE357_PROCESS_SPEC | ConvertFrom-Json; function Arg($line,$arg) { return $line -match ('(?:^|\\s)\"?'+[regex]::Escape($arg)+'\"?(?:\\s|$)') }; $found=@(); Get-CimInstance Win32_Process | ForEach-Object { $p=$_; $owned=($p.ExecutablePath -eq $spec.binary) -or (($p.ExecutablePath -eq $spec.node) -and (Arg $p.CommandLine $spec.directory) -and ((Arg $p.CommandLine $spec.supervisorScript) -or (Arg $p.CommandLine $spec.nextScript))); if($owned -and $p.CreationDate.ToUniversalTime() -ge [DateTime]::Parse($spec.createdAt).ToUniversalTime()) { $found+=@{pid=[int]$p.ProcessId;started=$p.CreationDate.ToUniversalTime().Ticks.ToString();path=$p.ExecutablePath} } }; ConvertTo-Json -InputObject $found -Compress";
 const candidates=JSON.parse(await run('powershell.exe',['-NoProfile','-NonInteractive','-Command',script],{env:{...childEnvironment(),ISSUE357_PROCESS_SPEC:m.processSpec}}));
 const found=[];
 for(const candidate of candidates) {
  const actual=await processIdentity(candidate.pid);
  // CIM reports microseconds; Process.StartTime can retain finer precision.
  if(actual&&actual.path.toLowerCase()===candidate.path.toLowerCase()&&Math.abs(Number(BigInt(actual.started)-BigInt(candidate.started)))<10000)found.push(actual);
 }
 return found;
}
