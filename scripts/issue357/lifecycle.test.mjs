import test from 'node:test';
import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
import {mkdir,writeFile,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {lock,processIdentity,pause} from './io.mjs';
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
