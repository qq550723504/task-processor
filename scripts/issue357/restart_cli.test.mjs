import test from 'node:test';
import assert from 'node:assert/strict';
import {execFile,spawn} from 'node:child_process';
import {promisify} from 'node:util';
import {randomUUID} from 'node:crypto';
import {mkdir,rm,writeFile} from 'node:fs/promises';
import {dirname,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {childEnvironment,makeManifest} from './contract.mjs';
import {json,readJSON,port,pause,sameProcess,run} from './io.mjs';
import {startCurrentApplications} from './current_application_lifecycle.mjs';

const execute=promisify(execFile);
const repo=dirname(dirname(dirname(fileURLToPath(import.meta.url))));
const cli=join(repo,'scripts','issue357-runtime.mjs');

async function fixture(plan) {
 const ports={issuer:0,web:0,go:0,database:0};
 for(const key of Object.keys(ports)){let selected;do{selected=await port()}while(Object.values(ports).includes(selected));ports[key]=selected}
 const id=randomUUID(),m=makeManifest(id,ports,'a'.repeat(40),'b'.repeat(40));
 m.status='ready';m.sourceDirectory=repo;m.webDirectory=repo;m.resources={};
 if(plan.currentApplication)m.runtimeMode='current-application';
 await mkdir(m.directory,{recursive:true});
 await json(join(m.directory,'manifest.json'),{...m,secrets:undefined});
 await json(join(m.directory,'owner-secrets.json'),m.secrets);
 await json(join(m.directory,'check.json'),{healthPassed:true,stale:true});
 const planPath=join(m.directory,'restart-test-plan.json');await json(planPath,plan);
 return {m,planPath};
}
async function invoke(m,planPath,action='restart') {
 const env=childEnvironment();
 try {
  const result=await execute(process.execPath,[cli,action,'--run',m.runId],{cwd:repo,windowsHide:true,env:{...env,NODE_TEST_CONTEXT:process.env.NODE_TEST_CONTEXT,ISSUE357_TEST_RESTART_PLAN:planPath}});
  return {code:0,stdout:result.stdout,stderr:result.stderr};
 }catch(error){return {code:error.code,stdout:error.stdout??'',stderr:error.stderr??''}}
}
async function scenario(plan,verify) {
 const {m,planPath}=await fixture(plan);
 try{
  let stopReport;
  if(plan.ownedChildExit!==undefined){stopReport=await ownedSupervisorStop(m,plan.ownedChildExit);await json(planPath,{...plan,stopResult:stopReport})}
  const result=await invoke(m,planPath);await verify({m,result,manifest:await readJSON(join(m.directory,'manifest.json')),trace:await readJSON(join(m.directory,'restart-test-trace.json')).catch(()=>[]),planPath,stopReport});
 }
 finally{await rm(m.directory,{recursive:true,force:true})}
}

// Real production supervisor/next launcher, OS children, stop files, exit report
// and released loopback ports. The Go child and Next app are deliberately tiny
// fixtures: this is process-result propagation, not Go/Next/IAM acceptance.
async function ownedSupervisorStop(m,exitCode) {
 const ui=join(m.directory,'fixture-ui'),nextModule=join(ui,'node_modules','next');
 await mkdir(nextModule,{recursive:true});
 await json(join(ui,'package.json'),{private:true});
 await json(join(nextModule,'package.json'),{name:'next',main:'index.cjs'});
 await writeFile(join(nextModule,'index.cjs'),`module.exports=()=>({prepare:async()=>{},getRequestHandler:()=>((_req,res)=>res.end('fixture')),close:async()=>{}});`);
 const child=join(m.directory,'fixture-go.cjs');
 await writeFile(child,`const {existsSync}=require('node:fs');const {join}=require('node:path');const server=require('node:http').createServer((_req,res)=>{res.statusCode=401;res.end()});server.listen(Number(process.argv[3]),'127.0.0.1');const timer=setInterval(()=>{if(existsSync(join(process.argv[2],'stop-go'))){clearInterval(timer);server.close(()=>process.exit(Number(process.argv[4]))) }},25);`);
 await json(join(m.directory,'services.json'),{binary:process.execPath,goArgs:[child,m.directory,String(m.ports.go),String(exitCode)],goEnvironment:{},uiDirectory:ui,webPort:m.ports.web,nextEnvironment:{}});
 const supervisor=spawn(process.execPath,[join(repo,'scripts','issue357','serve.mjs'),m.directory],{cwd:m.directory,env:childEnvironment(),windowsHide:true,stdio:['ignore','ignore','pipe']});
 let stderr='';supervisor.stderr.on('data',chunk=>{stderr=(stderr+String(chunk)).slice(-4096)});
 const finished=new Promise((resolve,reject)=>{supervisor.once('error',reject);supervisor.once('close',(code,signal)=>resolve({code,signal}))});
 const timeout=setTimeout(()=>supervisor.kill(),45000);
 try {
  const deadline=Date.now()+20000;
  for(;;){
   const records=await readJSON(join(m.directory,'processes.json')).catch(error=>{if(error.code==='ENOENT')return {};throw error});
   const ready=await readJSON(join(m.directory,'next-ready.json')).catch(error=>{if(error.code==='ENOENT')return null;throw error});
   const stopped=await readJSON(join(m.directory,'services-stopped.json')).catch(error=>{if(error.code==='ENOENT')return null;throw error});
   const stage=!records.supervisor?'supervisor-register':!records.go?'go-register':!records.next?'next-register':'next-ready';
   if(supervisor.exitCode!==null||supervisor.signalCode!==null||stopped||Date.now()>=deadline){
    const safeStderr=stderr.replaceAll(m.directory,'<fixture>').replaceAll(repo,'<repo>');
    throw new Error(`OWNED_CHILDREN_NOT_READY stage=${stage} exit=${supervisor.exitCode} signal=${supervisor.signalCode} stopped=${Boolean(stopped)} stderr=${safeStderr}`);
   }
   if(ready&&records.go&&records.next)break;
   await pause(100);
  }
  await writeFile(join(m.directory,'stop-services'),'stop');
  const ended=await finished;
  assert.equal(ended.signal,null);assert.equal(ended.code,exitCode===0?0:1);
  const report=await readJSON(join(m.directory,'services-stopped.json'));
  assert.equal(report.goExit,exitCode);assert.equal(report.nextExit,0);assert.equal(report.passed,exitCode===0);
  for(const record of Object.values(await readJSON(join(m.directory,'processes.json'))))assert.equal(await sameProcess(record),false);
  for(const selected of [m.ports.go,m.ports.web])assert.equal(await port(selected),selected);
  return report;
 } finally {
  clearTimeout(timeout);
  // Cleanup only exact recorded process instances from this fresh fixture.
  const records=await readJSON(join(m.directory,'processes.json')).catch(()=>({}));
  for(const record of Object.values(records))if(await sameProcess(record))await run('taskkill.exe',['/PID',String(record.pid),'/T','/F']);
  if(supervisor.exitCode===null&&supervisor.signalCode===null)supervisor.kill();
  await finished;
 }
}

test('CLI persists non-ready before stopping and does not stop if that save fails',{skip:process.platform!=='win32'},()=>scenario({failSave:'marking-non-ready'},({result,manifest,trace})=>{
 assert.notEqual(result.code,0);assert.match(result.stderr,/RESTART_STATE_SAVE_FAILED:marking-non-ready/);assert.equal(manifest.status,'ready');assert.ok(!trace.includes('stopping-applications'));assert.doesNotMatch(result.stdout,/READY/);
}));

for(const stage of ['stopping-applications','restarting-containers','provider-readiness','starting-applications','final-health'])test(`CLI restart ${stage} failure remains non-ready`,{skip:process.platform!=='win32'},()=>scenario({failOperation:stage},async({m,result,manifest,planPath})=>{
 assert.notEqual(result.code,0);assert.match(result.stderr,new RegExp(`RESTART_FAILED:${stage}:INJECTED_RESTART_FAILURE`));assert.equal(manifest.status,'restart-failed');assert.equal(manifest.restartFailure.stage,stage);assert.equal(manifest.restartFailure.code,'INJECTED_RESTART_FAILURE');assert.doesNotMatch(result.stdout,/READY/);
 for(const action of ['check','start']){const rejected=await invoke(m,planPath,action);assert.notEqual(rejected.code,0);assert.match(rejected.stderr,/RUN_NOT_READY/);assert.doesNotMatch(rejected.stdout,/READY/)}
}));

test('CLI final ready-save failure persists restart failure',{skip:process.platform!=='win32'},()=>scenario({failSave:'marking-ready'},({result,manifest})=>{
 assert.notEqual(result.code,0);assert.match(result.stderr,/RESTART_FAILED:marking-ready:INJECTED_SAVE_FAILURE/);assert.equal(manifest.status,'restart-failed');assert.equal(manifest.restartFailure.stage,'marking-ready');assert.doesNotMatch(result.stdout,/READY/);
}));

test('CLI failure-report save failure leaves prior transitional state observable',{skip:process.platform!=='win32'},()=>scenario({failOperation:'restarting-containers',failSave:'marking-failed'},({result,manifest})=>{
 assert.notEqual(result.code,0);assert.match(result.stderr,/RESTART_STATE_SAVE_FAILED:restarting-containers/);assert.equal(manifest.status,'restarting');assert.equal(manifest.restart.stage,'restarting-containers');assert.doesNotMatch(result.stdout,/READY/);
}));

test('CLI successful restart restores ready only after final health',{skip:process.platform!=='win32'},()=>scenario({},({result,manifest,trace})=>{
 assert.equal(result.code,0);assert.equal(manifest.status,'ready');assert.equal(manifest.restart,undefined);assert.equal(manifest.restartFailure,undefined);assert.match(result.stdout,/READY/);
 assert.ok(trace.indexOf('final-health')<trace.indexOf('save:marking-ready'));
}));

for(const stopResult of [{passed:false,reason:'SYNTHETIC_STOP_SECRET'},null,{}, {passed:1}])test(`current CLI restart rejects unsuccessful stop result ${JSON.stringify(stopResult)}`,{skip:process.platform!=='win32'},()=>scenario({currentApplication:true,stopResult},async({m,result,manifest,trace,planPath})=>{
 assert.notEqual(result.code,0);
 assert.match(result.stderr,/RESTART_FAILED:stopping-applications:APPLICATION_STOP_FAILED/);
 assert.equal(manifest.status,'restart-failed');
 assert.equal(manifest.restartFailure.stage,'stopping-applications');
 assert.equal(manifest.restartFailure.code,'APPLICATION_STOP_FAILED');
 for(const stage of ['restarting-containers','provider-readiness','starting-applications','final-health','save:marking-ready'])assert.ok(!trace.includes(stage),stage);
 assert.doesNotMatch(result.stdout,/READY/);
 assert.doesNotMatch(result.stdout+result.stderr,/SYNTHETIC_STOP_SECRET/);
 for(const action of ['check','start']){const rejected=await invoke(m,planPath,action);assert.notEqual(rejected.code,0);assert.match(rejected.stderr,/RUN_NOT_READY/);assert.doesNotMatch(rejected.stdout,/READY/)}
}));

test('current CLI successful stop result reaches final health and ready',{skip:process.platform!=='win32'},()=>scenario({currentApplication:true,stopResult:{passed:true}},({result,manifest,trace})=>{
 assert.equal(result.code,0);assert.equal(manifest.status,'ready');assert.match(result.stdout,/READY/);
 assert.ok(trace.indexOf('stopping-applications')<trace.indexOf('starting-applications'));
 assert.ok(trace.indexOf('final-health')<trace.indexOf('save:marking-ready'));
}));

test('current CLI thrown stop failure retains existing failure stage',{skip:process.platform!=='win32'},()=>scenario({currentApplication:true,failOperation:'stopping-applications'},({result,manifest,trace})=>{
 assert.notEqual(result.code,0);assert.equal(manifest.status,'restart-failed');assert.equal(manifest.restartFailure.code,'INJECTED_RESTART_FAILURE');
 assert.equal(manifest.restartFailure.stage,'stopping-applications');assert.ok(!trace.includes('starting-applications'));assert.doesNotMatch(result.stdout,/READY/);
}));

for(const exitCode of [0,7])test(`current lifecycle consumes real supervisor stop receipt for child exit ${exitCode}`,{skip:process.platform!=='win32'},async t=>scenario({currentApplication:true,ownedChildExit:exitCode},async({result,manifest,trace,stopReport})=>{
 t.diagnostic(`real supervisor receipt: ${JSON.stringify(stopReport)}; all three recorded processes exited and both ports released`);
 t.diagnostic(`restart CLI exit=${result.code}, state=${manifest.status}, READY=${result.stdout.includes('READY')}`);
 const recovering={status:'stopped'};
 await assert.rejects(startCurrentApplications(recovering,{
  assertFingerprint:async()=>{},save:async()=>{},startApplications:async()=>{throw new Error('START_FAILURE')},health:async()=>{},stopApplications:async()=>stopReport,
 }),exitCode===0?/START_FAILURE/:/CURRENT_APPLICATION_START_RECOVERY_FAILED/);
 assert.equal(recovering.status,exitCode===0?'start-failed':'stop-failed');
 if(exitCode===0){assert.equal(result.code,0);assert.equal(manifest.status,'ready');assert.match(result.stdout,/READY/)}
 else {
  assert.notEqual(result.code,0);assert.equal(manifest.status,'restart-failed');assert.equal(manifest.restartFailure.stage,'stopping-applications');
  assert.equal(manifest.restartFailure.code,'APPLICATION_STOP_FAILED');assert.ok(!trace.includes('starting-applications'));assert.ok(!trace.includes('final-health'));assert.doesNotMatch(result.stdout,/READY/);
 }
}));
