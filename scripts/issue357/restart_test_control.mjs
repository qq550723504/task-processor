import assert from 'node:assert/strict';
import {resolve,join} from 'node:path';
import {json,readJSON,save} from './io.mjs';

export async function restartTestOperations(m,planPath) {
 assert.equal(process.env.NODE_TEST_CONTEXT,'child-v8','TEST_CONTROL_FORBIDDEN');
 assert.equal(resolve(planPath),resolve(join(m.directory,'restart-test-plan.json')),'TEST_CONTROL_FORBIDDEN');
 assert.equal(m.sourceSha,'a'.repeat(40),'TEST_CONTROL_FORBIDDEN');
 assert.equal(m.webSha,'b'.repeat(40),'TEST_CONTROL_FORBIDDEN');
 assert.deepEqual(m.resources,{},'TEST_CONTROL_FORBIDDEN');
 assert.equal(m.instanceId,undefined,'TEST_CONTROL_FORBIDDEN');
 const plan=await readJSON(planPath),trace=[];
 async function record(value){trace.push(value);await json(join(m.directory,'restart-test-trace.json'),trace)}
 async function operation(stage){await record(stage);if(plan.failOperation===stage)throw new Error('INJECTED_RESTART_FAILURE')}
 return {
  assertFingerprint:async()=>record('assert-fingerprint'),
  save:async(state,checkpoint)=>{await record(`save:${checkpoint}`);if(plan.failSave===checkpoint)throw new Error('INJECTED_SAVE_FAILURE');await save(state)},
  stopApplications:async()=>operation('stopping-applications'),
  restartContainers:async()=>operation('restarting-containers'),
  waitProvider:async()=>operation('provider-readiness'),
  startApplications:async()=>operation('starting-applications'),
  health:async()=>operation('final-health'),
 };
}
