import {spawn} from 'node:child_process';
import {readFile,writeFile,rename,mkdir,lstat,realpath,unlink} from 'node:fs/promises';
import {dirname,join} from 'node:path';
import {createServer} from 'node:net';
import assert from 'node:assert/strict';
import {childEnvironment,runDirectory,validateManifest} from './contract.mjs';

export const pause=ms=>new Promise(r=>setTimeout(r,ms));
export const dockerEndpoint='npipe:////./pipe/dockerDesktopLinuxEngine';
export function run(command,args,options={}) {
 if(command==='docker')args=['--host',dockerEndpoint,...args];
 return new Promise((resolveRun,reject)=>{
  const p=spawn(command,args,{windowsHide:true,env:childEnvironment(),stdio:['pipe','pipe','pipe'],...options});let out='',err='';
  p.stdout?.on('data',d=>{out+=d;if(out.length>8*1024*1024)p.kill()});p.stderr?.on('data',d=>{err+=d;if(err.length>1024*1024)p.kill()});
  p.on('error',()=>reject(new Error('PROCESS_START_FAILED')));
  p.on('close',code=>code===0?resolveRun(out.trim()):reject(Object.assign(new Error(`PROCESS_FAILED:${command}:${code}`),{privateOutput:err||out})));
  p.stdin?.end(options.input??'');
 });
}
export async function json(path,value,renameFile=rename) {
 try{
  await writeFile(`${path}.tmp`,JSON.stringify(value,null,2),{mode:0o600});
  for(let attempt=0;;attempt++)try{await renameFile(`${path}.tmp`,path);break}catch(error){
   if(process.platform!=='win32'||!['EPERM','EACCES','EBUSY'].includes(error.code)||attempt===4)throw error;
   await pause(25*2**attempt);
  }
 }
 catch(error){await unlink(`${path}.tmp`).catch(()=>{});throw error}
}
export async function readJSON(path) {return JSON.parse(await readFile(path,'utf8'))}
export async function save(m){
 const {secrets,...publicManifest}=m;
 if(secrets)await json(join(m.directory,'owner-secrets.json'),secrets);
 await json(join(m.directory,'manifest.json'),publicManifest);
}
export async function load(id) {
 const dir=runDirectory(id),root=dirname(dir);
 assert.ok(!(await lstat(root)).isSymbolicLink(),'INVALID_RUN');assert.ok(!(await lstat(dir)).isSymbolicLink(),'INVALID_RUN');
 const physicalRoot=(await realpath(root)).toLowerCase(),physicalDirectory=(await realpath(dir)).toLowerCase();
 assert.equal(dirname(physicalDirectory),physicalRoot,'INVALID_RUN');
 const path=join(dir,'manifest.json');assert.ok(!(await lstat(path)).isSymbolicLink(),'INVALID_RUN');
 const manifest=await readJSON(path);assert.equal(typeof manifest.directory,'string','INVALID_RUN');assert.ok(!(await lstat(manifest.directory)).isSymbolicLink(),'INVALID_RUN');
 assert.equal((await realpath(manifest.directory)).toLowerCase(),physicalDirectory,'INVALID_RUN');
 manifest.directory=dir;const m=validateManifest(manifest);
 try{m.secrets=await readJSON(join(dir,'owner-secrets.json'))}catch(e){if(e.code!=='ENOENT')throw e}
 return m;
}
export async function privateDirectory(dir) {
 await mkdir(dir,{recursive:true,mode:0o700});
 if(process.platform==='win32') {
  const sid=await run('powershell.exe',['-NoProfile','-NonInteractive','-Command','[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value']);
  await run('icacls.exe',[dir,'/inheritance:r','/grant:r',`*${sid}:(OI)(CI)F`,'*S-1-5-18:(OI)(CI)F']);
 }
}
export async function port(preferred=0) {
 const s=createServer();await new Promise((r,reject)=>{s.once('error',reject);s.listen(preferred,'127.0.0.1',r)});const p=s.address().port;await new Promise(r=>s.close(r));return p;
}
export async function until(fn,name,timeout=300000) {
 const end=Date.now()+timeout;while(Date.now()<end){try {const v=await fn();if(v)return v}catch{}await pause(250)}throw new Error(`TIMEOUT:${name}`);
}
export async function processIdentity(pid,inspectProcess=run) {
 assert.ok(Number.isInteger(pid)&&pid>0,'INVALID_PROCESS');
 if(process.platform==='win32') {
  const output=await inspectProcess('powershell.exe',['-NoProfile','-NonInteractive','-Command',`$ErrorActionPreference='Stop'; try { $p=[Diagnostics.Process]::GetProcessById(${pid}) } catch [ArgumentException] { [Console]::WriteLine('null'); exit 0 }; @{pid=$p.Id;started=$p.StartTime.ToUniversalTime().Ticks.ToString();path=$p.Path} | ConvertTo-Json -Compress`]);
  return JSON.parse(output);
 }
 throw new Error('NOT_SUPPORTED: Windows process ownership validation required');
}
export async function sameProcess(record) {if(!record)return false;const actual=await processIdentity(record.pid);return actual?.started===record.started&&actual?.path===record.path}
export async function lock(m,fn) {
 assert.equal(process.platform,'win32','NOT_SUPPORTED: native Windows file lock');
 // A native exclusive file handle is released by the OS when this helper's
 // stdin closes, including controller crashes. No stale-file unlink race.
 const script="$ErrorActionPreference='Stop'; try { $f=[IO.File]::Open($env:ISSUE357_LOCK_FILE,[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None) } catch [IO.IOException] { exit 73 }; [Console]::WriteLine('LOCKED'); try { [Console]::In.ReadToEnd() | Out-Null } finally { $f.Dispose() }";
 const p=spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',script],{windowsHide:true,env:{...childEnvironment(),ISSUE357_LOCK_FILE:join(m.directory,'owner.lock')},stdio:['pipe','pipe','ignore']});
 let acquired=false;const done=new Promise(r=>p.once('close',r));
 await new Promise((r,reject)=>{p.once('error',()=>reject(new Error('LOCK_INSPECTION_FAILED')));p.stdout.on('data',d=>{if(String(d).includes('LOCKED')){acquired=true;r()}});p.once('close',code=>{if(!acquired)reject(new Error(code===73?'RUN_BUSY':'LOCK_INSPECTION_FAILED'))})});
 try{return await fn()}finally{p.stdin.end();await done}
}
