import test from 'node:test';
import assert from 'node:assert/strict';

import {startCurrentApplications} from './current_application_lifecycle.mjs';

function manifest(status='stopped') { return {status}; }

test('current application start cleanup still runs when ready-state save fails', async () => {
  const m=manifest(); let saves=0,stops=0;
  await assert.rejects(startCurrentApplications(m,{
    assertFingerprint:async()=>{},startApplications:async()=>{},health:async()=>{},
    save:async()=>{saves++;if(saves===2)throw new Error('SAVE_READY_FAILED')},
    stopApplications:async()=>{stops++;return {passed:true}},
  }),/SAVE_READY_FAILED/);
  assert.equal(stops,1);
  assert.equal(m.status,'start-failed');
  assert.equal(saves,3,'failure state is persisted after cleanup');
});

for(const point of ['start','health','ready-save'])test(`current start ${point} failure records returned stop failure and blocks retry`,async()=>{
  const m=manifest(),trace=[];let saves=0;
  const failure=new Error('ORIGINAL_START_FAILURE');
  let caught;
  try{await startCurrentApplications(m,{
    assertFingerprint:async()=>trace.push('fingerprint'),
    startApplications:async()=>{trace.push('start');if(point==='start')throw failure},
    health:async()=>{trace.push('health');if(point==='health')throw failure},
    save:async()=>{trace.push(`save:${m.status}`);if(++saves===2&&point==='ready-save')throw failure},
    stopApplications:async()=>{trace.push('stop');return {passed:false,reason:'SYNTHETIC_STOP_SECRET'}},
  })}catch(error){caught=error}
  assert.ok(caught instanceof AggregateError);
  assert.equal(caught.message,'CURRENT_APPLICATION_START_RECOVERY_FAILED');
  assert.equal(caught.errors[0],failure);
  assert.equal(caught.errors[1].message,'APPLICATION_STOP_FAILED');
  assert.equal(m.status,'stop-failed');assert.equal(m.startFailure.code,'ORIGINAL_START_FAILURE');assert.equal(m.stopFailure.code,'APPLICATION_STOP_FAILED');
  assert.equal(trace.filter(stage=>stage==='stop').length,1);assert.equal(trace.at(-1),'save:stop-failed');
  assert.doesNotMatch(JSON.stringify(m)+String(caught),/SYNTHETIC_STOP_SECRET/);
  await assert.rejects(startCurrentApplications(m,{}),/RUN_NOT_STOPPED/);
});

for(const stopResult of [undefined,null,{}, {passed:'true'}])test(`current start rejects unconfirmed cleanup ${JSON.stringify(stopResult)}`,async()=>{
  const m=manifest();
  await assert.rejects(startCurrentApplications(m,{
    assertFingerprint:async()=>{},save:async()=>{},startApplications:async()=>{throw new Error('START_FAILED')},health:async()=>{},stopApplications:async()=>stopResult,
  }),/CURRENT_APPLICATION_START_RECOVERY_FAILED/);
  assert.equal(m.status,'stop-failed');assert.equal(m.stopFailure.code,'APPLICATION_STOP_FAILED');
});

test('current start retains start, returned stop, and persistence failures together',async()=>{
  const m=manifest();let saves=0;
  await assert.rejects(startCurrentApplications(m,{
    assertFingerprint:async()=>{},save:async()=>{if(++saves===2)throw new Error('SAVE_FAILURE')},
    startApplications:async()=>{throw new Error('START_FAILURE')},health:async()=>{},stopApplications:async()=>({passed:false}),
  }),error=>{
    assert.ok(error instanceof AggregateError);
    assert.deepEqual(error.errors.map(item=>item.message),['START_FAILURE','APPLICATION_STOP_FAILED','SAVE_FAILURE']);return true;
  });
  assert.equal(m.status,'stop-failed');
});

test('current start with successful cleanup retains start-failed and permits retry',async()=>{
  const m=manifest();
  await assert.rejects(startCurrentApplications(m,{
    assertFingerprint:async()=>{},save:async()=>{},startApplications:async()=>{throw new Error('START_FAILURE')},health:async()=>{},stopApplications:async()=>({passed:true}),
  }),/START_FAILURE/);
  assert.equal(m.status,'start-failed');assert.equal(m.stopFailure,undefined);
  await startCurrentApplications(m,{
    assertFingerprint:async()=>{},save:async()=>{},startApplications:async()=>{},health:async()=>{},stopApplications:async()=>{throw new Error('UNEXPECTED_STOP')},
  });
  assert.equal(m.status,'ready');assert.equal(m.startFailure,undefined);
});

test('current application start cleanup failure blocks another start', async () => {
  const m=manifest();
  await assert.rejects(startCurrentApplications(m,{
    assertFingerprint:async()=>{},save:async()=>{},startApplications:async()=>{throw new Error('HEALTH_FAILED')},health:async()=>{},
    stopApplications:async()=>{throw new Error('PROCESS_STILL_RUNNING')},
  }));
  assert.equal(m.status,'stop-failed');
  await assert.rejects(startCurrentApplications(m,{
    assertFingerprint:async()=>{},save:async()=>{},startApplications:async()=>{},health:async()=>{},stopApplications:async()=>{},
  }),/RUN_NOT_STOPPED/);
});
