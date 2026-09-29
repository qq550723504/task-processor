import test from 'node:test';
import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
import {mkdir,writeFile,rm,lstat,readFile,rename} from 'node:fs/promises';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {join} from 'node:path';
import {lock,processIdentity,pause,json,run} from './io.mjs';
import {runDirectory} from './contract.mjs';

test('simultaneous stale-lock recovery never admits two controllers', {skip:process.platform!=='win32'}, async()=>{
 const directory=runDirectory(randomUUID());await mkdir(directory,{recursive:true});
 try {
  await writeFile(join(directory,'owner.lock'),JSON.stringify({pid:2147483647,started:'old',path:'old'}));
  let active=0,maximum=0;
  const results=await Promise.allSettled(Array.from({length:8},()=>lock({directory},async()=>{active++;maximum=Math.max(maximum,active);await pause(350);active--})));
  assert.equal(maximum,1);
  for(const r of results)if(r.status==='rejected')assert.match(r.reason.message,/RUN_BUSY/);
 }finally{await rm(directory,{recursive:true,force:true})}
});
test('process inspection failure cannot be interpreted as confirmed absence',async()=>{
 await assert.rejects(()=>processIdentity(2147483647,async()=>{throw new Error('inspection failed')}),/inspection failed/);
});

for(const identity of [{pid:123,started:'1',path:null},{pid:123,path:'node.exe'},{pid:456,started:'1',path:'node.exe'}])test('an incomplete or wrong process snapshot cannot establish ownership',async()=>{
 await assert.rejects(()=>processIdentity(123,async()=>JSON.stringify(identity)),/PROCESS_IDENTITY_UNAVAILABLE/);
});

test('a process exiting during native path inspection is confirmed absent',{skip:process.platform!=='win32'},async()=>{
 const child=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{windowsHide:true,stdio:'ignore'});
 const done=once(child,'close');await once(child,'spawn');
 try {
  const identity=await processIdentity(child.pid,async(command,args)=>{
   const inspected=[...args];
   inspected[inspected.length-1]=inspected.at(-1).replace('path=$p.Path','path=$(& { $started=$p.StartTime.ToUniversalTime().Ticks.ToString(); $p.Kill(); $p.WaitForExit(); $p.Path })');
   return run(command,inspected);
  });
  assert.equal(identity,null);
 }finally{if(child.exitCode===null&&child.signalCode===null)child.kill();await done}
});

test('atomic rename failure erases its secret temporary file',{skip:process.platform!=='win32'},async()=>{
 const directory=runDirectory(randomUUID());await mkdir(directory,{recursive:true});
 const path=join(directory,'owner-secrets.json');await writeFile(path,'original');
 const holder=spawn('powershell.exe',['-NoProfile','-NonInteractive','-Command',"$f=[IO.File]::Open($env:ISSUE357_HELD_FILE,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::None); [Console]::WriteLine('HELD'); try { [Console]::In.ReadToEnd() | Out-Null } finally { $f.Dispose() }"],{windowsHide:true,env:{...process.env,ISSUE357_HELD_FILE:path},stdio:['pipe','pipe','ignore']});
 try {
  await once(holder.stdout,'data');
  await assert.rejects(()=>json(path,{password:'synthetic-secret'}));
  await assert.rejects(()=>lstat(path+'.tmp'),{code:'ENOENT'});
 }finally{holder.stdin.end();await once(holder,'close');assert.equal(await readFile(path,'utf8'),'original');await rm(directory,{recursive:true,force:true})}
});

test('atomic writer retries a transient Windows rename conflict',{skip:process.platform!=='win32'},async()=>{
 const directory=runDirectory(randomUUID());await mkdir(directory,{recursive:true});
 const path=join(directory,'runtime.json');let calls=0;
 try {
  await json(path,{version:2},async(...args)=>{calls++;if(calls===1)throw Object.assign(new Error('transient conflict'),{code:'EPERM'});return rename(...args)});
  assert.equal(calls,2);assert.deepEqual(JSON.parse(await readFile(path,'utf8')),{version:2});
 }finally{await rm(directory,{recursive:true,force:true})}
});
