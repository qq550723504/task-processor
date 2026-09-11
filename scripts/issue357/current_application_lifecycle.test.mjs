import test from 'node:test';
import assert from 'node:assert/strict';

import {startCurrentApplications} from './current_application_lifecycle.mjs';

function manifest(status='stopped') { return {status}; }

test('current application start cleanup still runs when ready-state save fails', async () => {
  const m=manifest(); let saves=0,stops=0;
  await assert.rejects(startCurrentApplications(m,{
    assertFingerprint:async()=>{},startApplications:async()=>{},health:async()=>{},
    save:async()=>{saves++;if(saves===2)throw new Error('SAVE_READY_FAILED')},
    stopApplications:async()=>{stops++},
  }),/SAVE_READY_FAILED/);
  assert.equal(stops,1);
  assert.equal(m.status,'start-failed');
  assert.equal(saves,3,'failure state is persisted after cleanup');
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
