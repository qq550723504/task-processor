import test from 'node:test';
import assert from 'node:assert/strict';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {randomUUID} from 'node:crypto';
import {mkdir,rm} from 'node:fs/promises';
import {dirname,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {childEnvironment,makeManifest} from './contract.mjs';
import {json,readJSON,port} from './io.mjs';

const execute=promisify(execFile);
const repo=dirname(dirname(dirname(fileURLToPath(import.meta.url))));
const cli=join(repo,'scripts','issue357-runtime.mjs');
const subprocessTempRoot=(await execute(process.execPath,['--input-type=module','--eval',"import {tmpdir} from 'node:os';process.stdout.write(tmpdir())"],{env:childEnvironment(),windowsHide:true})).stdout;

async function fixture(plan) {
 const ports={issuer:0,web:0,go:0,database:0};
 for(const key of Object.keys(ports)){let selected;do{selected=await port()}while(Object.values(ports).includes(selected));ports[key]=selected}
 const id=randomUUID(),m=makeManifest(id,ports,'a'.repeat(40),'b'.repeat(40));
 m.directory=join(subprocessTempRoot,'task-processor-issue357',id);
 m.status='ready';m.sourceDirectory=repo;m.webDirectory=repo;m.resources={};
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
 try{const result=await invoke(m,planPath);await verify({m,result,manifest:await readJSON(join(m.directory,'manifest.json')),trace:await readJSON(join(m.directory,'restart-test-trace.json')).catch(()=>[]),planPath})}
 finally{await rm(m.directory,{recursive:true,force:true})}
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
