import test from 'node:test';
import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
import {mkdir,readFile,rm} from 'node:fs/promises';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {runDirectory} from './contract.mjs';
import {json,processIdentity} from './io.mjs';

const supervisor=fileURLToPath(new URL('./serve.mjs',import.meta.url));

for(const currentApplication of [true,false])test(`early startup exit confirms cleanup without masking failure (current=${currentApplication})`,{skip:process.platform!=='win32',timeout:20000},async()=>{
 const directory=runDirectory(randomUUID());await mkdir(directory,{recursive:true});
 let child;
 try {
  await json(join(directory,'services.json'),{binary:process.execPath,goArgs:['-e','process.exit(2)'],goEnvironment:{},uiDirectory:directory,webPort:0,nextEnvironment:{},goReadyFromPort:currentApplication,goPort:0});
  child=spawn(process.execPath,[supervisor,directory],{windowsHide:true,stdio:'ignore'});
  const [exit]=await once(child,'close');
  assert.equal(exit,1,'the failed application must still fail supervisor startup');
  assert.equal(await processIdentity(child.pid),null);
  const report=JSON.parse(await readFile(join(directory,'services-stopped.json'),'utf8'));
  assert.equal(report.passed,currentApplication,'only confirmed cleanup of a failed current startup permits the existing start-failed retry path');
  if(currentApplication){assert.equal(report.startupFailed,true);assert.equal(report.goExit,2)}
 }finally{
  if(child&&child.exitCode===null&&child.signalCode===null){child.kill();await once(child,'close')}
  await rm(directory,{recursive:true,force:true});
 }
});

test('startup cleanup requiring forced termination cannot confirm a successful stop',{skip:process.platform!=='win32',timeout:40000},async()=>{
 const directory=runDirectory(randomUUID());await mkdir(directory,{recursive:true});
 let child;
 try {
  await json(join(directory,'services.json'),{binary:process.execPath,goArgs:['-e','setInterval(()=>{},1000)'],goEnvironment:{},uiDirectory:directory,webPort:0,nextEnvironment:{},goReadyFromPort:true,goPort:0});
  child=spawn(process.execPath,[supervisor,directory],{windowsHide:true,stdio:'ignore'});
  const [exit]=await once(child,'close');assert.equal(exit,1);
  const report=JSON.parse(await readFile(join(directory,'services-stopped.json'),'utf8'));
  assert.equal(report.startupFailed,true);assert.equal(report.passed,false);
  const records=JSON.parse(await readFile(join(directory,'processes.json'),'utf8'));
  for(const record of Object.values(records))assert.equal(await processIdentity(record.pid),null);
 }finally{
  if(child&&child.exitCode===null&&child.signalCode===null){child.kill();await once(child,'close')}
  await rm(directory,{recursive:true,force:true});
 }
});
