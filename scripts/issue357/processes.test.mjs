import test from 'node:test';
import assert from 'node:assert/strict';
import {startChildren,terminateChildren,discoverRunProcesses} from './processes.mjs';
import {processIdentity,json} from './io.mjs';
import {randomUUID} from 'node:crypto';
import {mkdir,writeFile,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {runDirectory} from './contract.mjs';

for(const failure of ['second spawn','registry write'])test(`${failure} failure retains handles for complete cleanup`,{skip:process.platform!=='win32'},async()=>{
 const children=[],spec={name:'first',command:process.execPath,args:['-e','setInterval(()=>{},1000)']};
 let registrations=0;
 try {
  await assert.rejects(()=>startChildren([spec,{...spec,name:'second',command:'issue357-no-such-executable'}],async()=>{registrations++;if(failure==='registry write')throw new Error('registry failed')},children));
  assert.equal(registrations,1);
 }finally{await terminateChildren(children)}
 for(const child of children)if(child.pid)assert.equal(await processIdentity(child.pid),null);
});

test('lost registry recovery selects only exact run arguments',{skip:process.platform!=='win32'},async()=>{
 const directory=runDirectory(randomUUID()),children=[];
 await mkdir(directory,{recursive:true});
 const script=join(directory,'next.mjs'),processSpec=join(directory,'process-spec.json');
 try {
  await writeFile(script,'setInterval(()=>{},1000)');
  await json(processSpec,{binary:join(directory,'runtime.test.exe'),node:process.execPath,directory,supervisorScript:join(directory,'serve.mjs'),nextScript:script,createdAt:new Date().toISOString()});
  await startChildren([{name:'owned',command:process.execPath,args:[script,directory]},{name:'foreign',command:process.execPath,args:[script,directory+'-different']}],async()=>{},children);
  const found=await discoverRunProcesses({processSpec});
  assert.deepEqual(found.map(p=>p.pid),[children[0].pid]);
  assert.deepEqual(found[0],await processIdentity(children[0].pid));
 }finally{await terminateChildren(children);await rm(directory,{recursive:true,force:true})}
});
