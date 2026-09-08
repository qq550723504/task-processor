const stages={
 stopping:'stopping-applications',
 containers:'restarting-containers',
 provider:'provider-readiness',
 applications:'starting-applications',
 health:'final-health',
 ready:'marking-ready',
};

function safeCode(error) {
 const value=String(error?.message??'RESTART_STAGE_FAILED').split(/[\r\n:]/,1)[0];
 return /^[A-Z][A-Z0-9_]{2,80}$/.test(value)?value:'RESTART_STAGE_FAILED';
}

export async function restartRuntime(m,ops) {
 await ops.assertFingerprint(m);
 const startedAt=new Date().toISOString();
 let stage=stages.stopping;
 m.status='restarting';m.restart={stage,startedAt};delete m.restartFailure;
 try{await ops.save(m,'marking-non-ready')}
 catch{throw new Error('RESTART_STATE_SAVE_FAILED:marking-non-ready')}

 async function mark(next) {
  stage=next;m.status='restarting';m.restart={stage,startedAt};
  await ops.save(m,`before-${stage}`);
 }
 try {
  await ops.stopApplications(m);
  await mark(stages.containers);await ops.restartContainers(m);
  await mark(stages.provider);await ops.waitProvider(m);
  await mark(stages.applications);await ops.startApplications(m);
  await mark(stages.health);await ops.health(m);
  await mark(stages.ready);
  m.status='ready';delete m.restart;delete m.restartFailure;
  await ops.save(m,'marking-ready');
 } catch(error) {
  m.status='restart-failed';delete m.restart;
  m.restartFailure={stage,code:safeCode(error),failedAt:new Date().toISOString()};
  try{await ops.save(m,'marking-failed')}
  catch{throw new Error(`RESTART_STATE_SAVE_FAILED:${stage}`)}
  throw new Error(`RESTART_FAILED:${stage}:${m.restartFailure.code}`);
 }
}
