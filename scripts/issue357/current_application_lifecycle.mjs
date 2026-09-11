import assert from 'node:assert/strict';

export async function startCurrentApplications(m,operations) {
  assert.ok(['stopped','start-failed'].includes(m.status),'RUN_NOT_STOPPED');
  await operations.assertFingerprint(m);
  m.status='starting-applications';
  delete m.startFailure;
  delete m.stopFailure;
  await operations.save(m);
  try {
    await operations.startApplications(m);
    await operations.health(m);
    m.status='ready';
    await operations.save(m);
  } catch(error) {
    const failure={code:String(error.message).split(':',1)[0],at:new Date().toISOString()};
    let stopError;
    try { await operations.stopApplications(m); }
    catch(cause) { stopError=cause; }
    m.status=stopError?'stop-failed':'start-failed';
    m.startFailure=failure;
    if(stopError)m.stopFailure={code:String(stopError.message).split(':',1)[0],at:new Date().toISOString()};
    let saveError;
    try { await operations.save(m); }
    catch(cause) { saveError=cause; }
    if(stopError||saveError)throw new AggregateError([error,...(stopError?[stopError]:[]),...(saveError?[saveError]:[])],'CURRENT_APPLICATION_START_RECOVERY_FAILED');
    throw error;
  }
}
