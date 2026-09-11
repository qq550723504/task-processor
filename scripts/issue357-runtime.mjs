import assert from 'node:assert/strict';
import {randomUUID,createHash} from 'node:crypto';
import {spawn} from 'node:child_process';
import {readFile,writeFile,mkdir,copyFile,symlink,unlink,rm,lstat} from 'node:fs/promises';
import {dirname,join,resolve,relative} from 'node:path';
import {fileURLToPath} from 'node:url';
import {makeManifest,validateManifest,childEnvironment,ownedResource,label,runDirectory} from './issue357/contract.mjs';
import {composeConfiguration,proxyConfiguration,images} from './issue357/compose.mjs';
import {run,json,readJSON,save,load,privateDirectory,port,until,processIdentity,sameProcess,lock,pause,dockerEndpoint} from './issue357/io.mjs';
import {provider,createSubjects,grantSubjects,authorizationControl} from './issue357/provider.mjs';
import {discoverRunProcesses} from './issue357/processes.mjs';
import {restartRuntime} from './issue357/restart.mjs';
import {waitForProvider} from './issue357/readiness.mjs';

const repo=resolve(dirname(fileURLToPath(import.meta.url)),'..');
const argv=process.argv.slice(2),action=argv[0];
function arg(name){const i=argv.indexOf(name);return i<0?undefined:argv[i+1]}
function currentMode(m){return m.runtimeMode==='current-application'}
const services=['proxy','zitadel-api','zitadel-login','identity-db','commercial-db'];
const volumes=['identity','commercial','login','setup'];
function binary(m){return join(m.directory,'runtime.test.exe')}
function applicationBinary(m){return join(m.directory,'current-application.exe')}
function schemaBinary(m){return join(m.directory,'bin','source-account-registry-schema-init.exe')}
function baseConfig(m){return {runId:m.runId,runtimeMode:m.runtimeMode,issuer:m.origins.issuer,issuerPort:m.ports.issuer,webOrigin:m.origins.web,goPort:m.ports.go,databasePort:m.ports.database,databaseHost:'127.0.0.1',databaseName:'issue357',databaseUser:'commercial_reader',databasePassword:m.secrets.reader,projectId:m.projectId,organizationA:m.organizations.A?.id,organizationB:m.organizations.B?.id,organizationC:m.organizations.C?.id}}
async function go(m,mode,input='runtime.json') {
 return run(binary(m),[`-test.run=^TestIssue357${mode}$`,'-test.v','-test.timeout=120s'],{cwd:m.directory,env:{...childEnvironment(),ISSUE357_INPUT_FILE:join(m.directory,input)}});
}
async function inspect(kind,name) {
 try {return JSON.parse(await run('docker',[kind,'inspect',name]))[0]}catch(e){
  // Distinguish absence from daemon failure before claiming cleanup.
  await run('docker',['info','--format','{{.ServerVersion}}']);
  if(/No such|not found/i.test(e.privateOutput??''))return null;throw e;
 }
}
async function inventory(m,required=false) {
 assert.equal(m.dockerEndpoint??dockerEndpoint,dockerEndpoint,'DOCKER_ENDPOINT_MISMATCH');
 const expected=[...services.map(s=>['container',`${m.project}-${s}`]),...volumes.map(v=>['volume',`${m.project}-${v}`]),['network',`${m.project}-network`]];
 for(const [kind,name]of expected){const r=await inspect(kind,name);if(!r){assert.ok(!required,'MISSING_RESOURCE');continue}
  const id=r.Id??r.ID??r.Name;ownedResource(m,r,name,m.resources[name]?.id??id);
  m.resources[name]={kind,id,name};
 }
 await save(m);
}
async function assertFingerprint(m){
 assert.equal(m.sourceDirty,false,'SOURCE_WAS_DIRTY');assert.equal(m.webDirty,false,'SOURCE_WAS_DIRTY');
 assert.equal(await run('git',['rev-parse','HEAD'],{cwd:repo}),m.sourceSha,'SOURCE_CHANGED');assert.equal(await run('git',['rev-parse','HEAD'],{cwd:m.webDirectory}),m.webSha,'SOURCE_CHANGED');
 assert.equal(await run('git',['status','--porcelain'],{cwd:repo}),'','SOURCE_CHANGED');assert.equal(await run('git',['status','--porcelain','--','.'],{cwd:m.webDirectory}),'','SOURCE_CHANGED');
}
async function copyUI(m,web) {
 const ui=join(m.directory,'ui');await mkdir(ui,{recursive:true});
 const files=(await run('git',['ls-files','-z','--','.'],{cwd:web})).split('\0').filter(Boolean);
 for(const file of files){
  assert.ok(!relative(web,resolve(web,file)).startsWith('..'),'INVALID_SOURCE');
  if(file.split('/').some(p=>p.startsWith('.env')||p==='node_modules'||p==='.next'))continue;
  const source=join(web,file);assert.ok(!(await lstat(source)).isSymbolicLink(),'SOURCE_SYMLINK');
  const dest=join(ui,file);await mkdir(dirname(dest),{recursive:true});await copyFile(source,dest);
 }
 await symlink(join(web,'node_modules'),join(ui,'node_modules'),'junction');
 return ui;
}
async function startApplications(m) {
 const apps=await readJSON(join(m.directory,'applications.json'));
 await json(join(m.directory,'runtime.json'),{...baseConfig(m),apiClientId:apps.APIClientID,apiClientSecret:apps.APIClientSecret});
 const ui=join(m.directory,'ui');
 const nextEnvironment={NODE_ENV:'development',NEXT_TELEMETRY_DISABLED:'1',AUTH_SECRET:m.secrets.auth,AUTH_URL:m.origins.web,LISTINGKIT_PUBLIC_BASE_URL:m.origins.web,
  ZITADEL_ISSUER_URL:m.origins.issuer,ZITADEL_CLIENT_ID:apps.OIDCClientID,ZITADEL_CLIENT_SECRET:apps.OIDCClientSecret,ZITADEL_REDIRECT_URI:`${m.origins.web}/api/auth/callback/zitadel`,ZITADEL_POST_LOGOUT_REDIRECT_URI:m.origins.web,ZITADEL_SCOPES:[...apps.RecommendedScopes,'offline_access'].join(' '),
  LISTINGKIT_SERVICE_API_BASE:`${m.origins.go}/api/v1`,COMMERCIAL_API_ORIGIN:m.origins.go};
 let goService={binary:binary(m),goArgs:['-test.run=^TestIssue357Serve$','-test.timeout=24h'],goEnvironment:{ISSUE357_INPUT_FILE:join(m.directory,'runtime.json')}};
 if(currentMode(m)) {
  const currentConfig=join(m.directory,'current-application.json');
  await json(currentConfig,{schemaVersion:1,listen:{host:'127.0.0.1',port:m.ports.go},identity:{issuerURL:m.origins.issuer,authorizationAPIURL:m.origins.issuer,clientID:apps.APIClientID,clientSecret:apps.APIClientSecret,projectID:m.projectId},sourceAccountDatabase:{host:'127.0.0.1',port:m.ports.database,user:'source_account_runtime',password:m.secrets.sourceRuntime,database:'issue357',maxConnections:4},commercialDatabase:{host:'127.0.0.1',port:m.ports.database,user:'commercial_reader',password:m.secrets.reader,database:'issue357',maxConnections:4}});
  goService={binary:applicationBinary(m),goArgs:['-config',currentConfig,'-shutdown-file',join(m.directory,'stop-go')],goEnvironment:{},goReadyFromPort:true,goPort:m.ports.go};
 }
 await json(join(m.directory,'services.json'),{...goService,uiDirectory:ui,webPort:m.ports.web,nextEnvironment});
 m.processSpec=join(m.directory,'process-spec.json');
 await json(m.processSpec,{binary:goService.binary,node:process.execPath,directory:m.directory,supervisorScript:join(repo,'scripts/issue357/serve.mjs'),nextScript:join(repo,'scripts/issue357/next.mjs'),createdAt:m.createdAt});
 await save(m);
 for(const file of ['stop-go','stop-next','stop-services','go-ready.json','next-ready.json','services-stopped.json'])await unlink(join(m.directory,file)).catch(e=>{if(e.code!=='ENOENT')throw e});
 const p=spawn(process.execPath,[join(repo,'scripts/issue357/serve.mjs'),m.directory],{env:childEnvironment(),cwd:m.directory,detached:true,windowsHide:true,stdio:'ignore'});
 await new Promise((r,reject)=>{p.once('spawn',r);p.once('error',()=>reject(new Error('SUPERVISOR_START_FAILED')))});p.unref();
 m.supervisor=await processIdentity(p.pid);await save(m);
 const started=await until(async()=>{const stopped=await readJSON(join(m.directory,'services-stopped.json')).catch(e=>{if(e.code!=='ENOENT')throw e});if(stopped)return {failed:true};try{await readJSON(join(m.directory,'go-ready.json'));await readJSON(join(m.directory,'next-ready.json'));return {ready:true}}catch(e){if(e.code!=='ENOENT')throw e;return false}},'APPLICATION_START');
 if(started.failed)throw new Error('APPLICATION_START_FAILED');
}
async function health(m) {
 await inventory(m,true);
 const discovery=await (await fetch(`${m.origins.issuer}/.well-known/openid-configuration`,{signal:AbortSignal.timeout(10000)})).json();
 assert.equal(discovery.issuer,m.origins.issuer,'ISSUER_MISMATCH');
 for(const key of ['authorization_endpoint','token_endpoint','userinfo_endpoint','introspection_endpoint','end_session_endpoint'])assert.equal(new URL(discovery[key]).origin,m.origins.issuer,'ENDPOINT_MISMATCH');
 const ready=await fetch(`${m.origins.issuer}/debug/ready`,{signal:AbortSignal.timeout(10000)});assert.equal(ready.status,200,'PROVIDER_NOT_READY');
 const login=await fetch(`${m.origins.issuer}/ui/v2/login/healthy`,{signal:AbortSignal.timeout(10000)});assert.equal(login.status,200,'LOGIN_NOT_READY');
 for(const path of ['/api/v1/account/profile','/api/v1/workbench/commercial/overview',...(currentMode(m)?['/api/v1/workbench/source-accounts']:[])]){const r=await fetch(m.origins.go+path,{signal:AbortSignal.timeout(10000)});assert.equal(r.status,401,'UNAUTHENTICATED_NOT_DENIED')}
 const providers=await (await fetch(`${m.origins.web}/api/auth/providers`,{signal:AbortSignal.timeout(90000)})).json();assert.ok(providers.zitadel,'NEXT_PROVIDER_MISSING');
 if(currentMode(m))await json(join(m.directory,'snapshot.json'),{...baseConfig(m),databaseUser:'issue357',databasePassword:m.secrets.commercialDatabase});
 await go(m,'Snapshot',currentMode(m)?'snapshot.json':'runtime.json');
 const report={schemaVersion:'issue357-check-v1',runId:m.runId,sourceSha:m.sourceSha,webSha:m.webSha,healthPassed:true,zeroWrite:await readJSON(join(m.directory,'zero-write.json')),realBrowser:'NOT_RUN_BY_CHECK',at:new Date().toISOString()};await json(join(m.directory,'check.json'),report);return report;
}
async function stopApplications(m) {
 await writeFile(join(m.directory,'stop-services'),'stop');
 const records=await readJSON(join(m.directory,'processes.json')).catch(e=>{if(e.code!=='ENOENT')throw e;return {supervisor:m.supervisor}});
 if(m.processSpec)for(const r of await discoverRunProcesses(m))records[`recovered-${r.pid}`]=r;
 if(!await sameProcess(records.supervisor)){await writeFile(join(m.directory,'stop-next'),'stop');await writeFile(join(m.directory,'stop-go'),'stop')}
 await until(async()=>Object.values(records).every(r=>!r)||!(await Promise.all(Object.values(records).map(sameProcess))).some(Boolean),'APPLICATION_STOP',35000).catch(async()=>{
  // Only exact live process instances recorded for this run may be terminated.
  for(const r of Object.values(records))if(await sameProcess(r))await run('taskkill.exe',['/PID',String(r.pid),'/T','/F']);
 });
 for(const r of Object.values(records))assert.equal(await sameProcess(r),false,'PROCESS_STILL_RUNNING');
 for(const p of [m.ports.web,m.ports.go])assert.equal(await port(p),p,'PORT_NOT_RELEASED');
 return await readJSON(join(m.directory,'services-stopped.json')).catch(()=>({passed:false,reason:'abrupt_application_exit'}));
}
async function cleanup(m) {
 validateManifest(m);await inventory(m);m.status='stopping';await save(m);
 let audit=await readJSON(join(m.directory,'cleanup-audit.json')).catch(e=>{if(e.code!=='ENOENT')throw e;return null});
 // Every retry must recheck processes even after an earlier successful snapshot.
 let applications={passed:true};
 if(m.supervisor||m.processSpec)applications=await stopApplications(m);
 if(!audit) {
  let zeroWrite={passed:false,reason:'setup_incomplete'};
  if(m.seeded)try{if(currentMode(m))await json(join(m.directory,'snapshot.json'),{...baseConfig(m),databaseUser:'issue357',databasePassword:m.secrets.commercialDatabase});await go(m,'Snapshot',currentMode(m)?'snapshot.json':'runtime.json');zeroWrite=await readJSON(join(m.directory,'zero-write.json'))}catch{zeroWrite={passed:false,reason:'snapshot_failed'}}
  audit={applications,zeroWrite};await json(join(m.directory,'cleanup-audit.json'),audit);
 }
 const {zeroWrite}=audit;
 applications={...applications,passed:applications.passed&&audit.applications.passed};
 // Cleanup continues after audit failure. Ownership is rechecked for every delete.
 for(const name of [...services.map(s=>`${m.project}-${s}`),...volumes.map(v=>`${m.project}-${v}`),`${m.project}-network`]){
  const record=m.resources[name];if(!record)continue;const r=await inspect(record.kind,name);if(!r)continue;ownedResource(m,r,name,record.id);
  if(record.kind==='container') {await run('docker',['container','stop','--time','10',record.id]);await run('docker',['container','rm',record.id])}
  else await run('docker',[record.kind,'rm',record.id]);
 }
 for(const record of Object.values(m.resources))assert.equal(await inspect(record.kind,record.name),null,'RESOURCE_NOT_RELEASED');
 for(const p of Object.values(m.ports))await port(p);
 const passed=applications.passed&&(!m.seeded||zeroWrite.passed);
 const evidence={schemaVersion:'issue357-cleanup-v1',runId:m.runId,sourceSha:m.sourceSha,webSha:m.webSha,passed,resourcesReleased:true,portsReleased:true,applications,zeroWrite,at:new Date().toISOString()};
 // Remove secrets only after all owned resources are confirmed absent. Do not
 // recursively remove the UI dependency junction (it points to the checkout).
 const ui=join(m.directory,'ui');await unlink(join(ui,'node_modules')).catch(e=>{if(e.code!=='ENOENT')throw e});
 assert.equal(resolve(ui),resolve(runDirectory(m.runId),'ui'));
 await rm(ui,{recursive:true,force:true});
 await unlink(join(m.directory,'schema-init','source-account-schema.yaml')).catch(e=>{if(e.code!=='ENOENT')throw e});
 const privateFiles=['bootstrap.pat','applications.json','provision.json','seed.json','runtime.json','snapshot.json','services.json','compose.json','admin.credentials.json','viewer.credentials.json','no-org.credentials.json','owner-secrets.json','current-application.json','source-account-schema.yaml','grant-source.json','source-snapshot-input.json','source-permission-input.json','issue390-chain.json'];
 for(const name of privateFiles.flatMap(name=>[name,`${name}.tmp`]))await unlink(join(m.directory,name)).catch(e=>{if(e.code!=='ENOENT')throw e});
 delete m.secrets;await json(join(m.directory,'cleanup.json'),evidence);m.status=currentMode(m)?'destroyed':'stopped';await save(m);return evidence;
}
async function initializeCurrentApplication(m) {
 const schemaWork=join(m.directory,'schema-init'),schemaConfig=join(schemaWork,'source-account-schema.yaml');await mkdir(schemaWork,{recursive:true});
 for(const candidate of [join(schemaWork,'.env'),join(schemaWork,'.env.source-account-schema'),join(schemaWork,'source-account-schema.env'),join(m.directory,'bin','.env'),join(m.directory,'.env')])try{await lstat(candidate);throw new Error('SCHEMA_ENV_FILE_PRESENT')}catch(error){if(error.code!=='ENOENT')throw error}
 await writeFile(schemaConfig,["database:","  host: 127.0.0.1",`  port: ${m.ports.database}`,"  user: issue357",`  password: \"${m.secrets.commercialDatabase}\"`,"  database: issue357","  max_connections: 2","  max_idle_connections: 1","  connection_max_lifetime: 1h",""].join('\n'),{mode:0o600});
 await run(schemaBinary(m),['-config',schemaConfig],{cwd:schemaWork,env:childEnvironment()});await unlink(schemaConfig);
 await json(join(m.directory,'grant-source.json'),{...baseConfig(m),databaseUser:'issue357',databasePassword:m.secrets.commercialDatabase,sourceRuntimePassword:m.secrets.sourceRuntime});
 await go(m,'GrantSourceAccountRuntime','grant-source.json');await unlink(join(m.directory,'grant-source.json'));
 m.sourceAccountInitialized=true;await save(m);
}
async function sourceAccountSnapshot(m) {
 await json(join(m.directory,'source-snapshot-input.json'),{...baseConfig(m),databaseUser:'issue357',databasePassword:m.secrets.commercialDatabase});
 await go(m,'SourceAccountSnapshot','source-snapshot-input.json');await unlink(join(m.directory,'source-snapshot-input.json'));
 return readJSON(join(m.directory,'source-account-snapshot.json'));
}
async function sourceAccountPermission(m,permissionAction) {
 assert.ok(['revoke','restore'].includes(permissionAction),'INVALID_PERMISSION_ACTION');
 await json(join(m.directory,'source-permission-input.json'),{...baseConfig(m),databaseUser:'issue357',databasePassword:m.secrets.commercialDatabase,permissionAction});
 await go(m,'SourceAccountPermission','source-permission-input.json');await unlink(join(m.directory,'source-permission-input.json'));
}
async function stopCurrentApplications(m) {
 assert.ok(['ready','stop-failed'].includes(m.status),'RUN_NOT_READY');m.status='stopping-applications';await save(m);
 try {
  const applications=await stopApplications(m);await inventory(m,true);const factsRetained=await sourceAccountSnapshot(m);
  const evidence={schemaVersion:'issue390-stop-v1',runId:m.runId,passed:applications.passed,resourcesRetained:true,factsRetained,applications,at:new Date().toISOString()};
  await json(join(m.directory,'stop.json'),evidence);m.status=applications.passed?'stopped':'stop-failed';await save(m);if(!applications.passed)throw new Error('APPLICATION_STOP_FAILED');return evidence;
 } catch(error) {m.status='stop-failed';m.stopFailure={code:String(error.message).split(':',1)[0],at:new Date().toISOString()};await save(m);throw error}
}
async function startCurrentApplications(m) {
 assert.ok(['stopped','start-failed'].includes(m.status),'RUN_NOT_STOPPED');await assertFingerprint(m);m.status='starting-applications';delete m.startFailure;await save(m);
 try {await startApplications(m);await health(m);m.status='ready';await save(m)}
 catch(error){m.status='start-failed';m.startFailure={code:String(error.message).split(':',1)[0],at:new Date().toISOString()};await save(m);try{await stopApplications(m)}catch{}throw error}
}
async function fresh() {
 assert.equal(process.platform,'win32','NOT_SUPPORTED: Windows first');
 await run('docker',['version','--format','{{.Server.Version}}']);await run('docker',['compose','version']);
 const web=resolve(arg('--web-dir')??join(repo,'web/listingkit-ui'));await lstat(join(web,'node_modules/next/package.json'));
 const ports={};for(const key of ['issuer','web','go','database']){let p;do{p=await port()}while(Object.values(ports).includes(p));ports[key]=p}
 const m=makeManifest(randomUUID(),ports,await run('git',['rev-parse','HEAD'],{cwd:repo}),await run('git',['rev-parse','HEAD'],{cwd:web}));m.webDirectory=web;m.sourceDirectory=repo;m.dockerEndpoint=dockerEndpoint;if(argv.includes('--current-application'))m.runtimeMode='current-application';
 m.sourceDirty=Boolean(await run('git',['status','--porcelain'],{cwd:repo}));m.webDirty=Boolean(await run('git',['status','--porcelain','--','.'],{cwd:web}));
 await privateDirectory(m.directory);await save(m);console.log(`runId=${m.runId} manifest=${join(m.directory,'manifest.json')}`);
 return lock(m,async()=>{
  try {
   // Before create, every expected name must be absent. No adopt-by-name.
   for(const [kind,names]of [['container',services],['volume',volumes],['network',['network']]])for(const suffix of names)assert.equal(await inspect(kind,`${m.project}-${suffix}`),null,'RESOURCE_CONFLICT');
   await copyUI(m,web);
   await run('go',['test','-tags','issue357','-c','-o',binary(m),'./internal/app/httpapi'],{cwd:repo});
   if(currentMode(m)){await mkdir(dirname(schemaBinary(m)),{recursive:true});await run('go',['build','-o',applicationBinary(m),'./cmd/current-application'],{cwd:repo});await run('go',['build','-o',schemaBinary(m),'./cmd/source-account-registry-schema-init'],{cwd:repo})}
   await json(join(m.directory,'compose.json'),composeConfiguration(m));await json(join(m.directory,'proxy.json'),proxyConfiguration());await writeFile(join(m.directory,'empty.env'),'');
   await run('docker',['compose','--env-file',join(m.directory,'empty.env'),'-f',join(m.directory,'compose.json'),'-p',m.project,'up','-d','--wait','--wait-timeout','300'],{cwd:m.directory});
   await inventory(m,true);m.status='provider-ready';await save(m);
   m.imageDigests={};for(const [key,image]of Object.entries(images)){const info=JSON.parse(await run('docker',['image','inspect',image]))[0];m.imageDigests[key]={image,id:info.Id,digests:info.RepoDigests}}await save(m);
   await run('docker',['cp',`${m.project}-zitadel-api:/setup/bootstrap.pat`,join(m.directory,'bootstrap.pat')]);
   await createSubjects(m);
   await json(join(m.directory,'provision.json'),{...baseConfig(m),managementToken:(await readFile(join(m.directory,'bootstrap.pat'),'utf8')).trim()});await go(m,'Provision','provision.json');await unlink(join(m.directory,'provision.json'));
   const apps=await readJSON(join(m.directory,'applications.json'));m.projectId=apps.ProjectID;m.apiClientId=apps.APIClientID;m.oidcClientId=apps.OIDCClientID;await save(m);await grantSubjects(m);
   await json(join(m.directory,'seed.json'),{...baseConfig(m),databaseUser:'issue357',databasePassword:m.secrets.commercialDatabase,readerPassword:m.secrets.reader});await go(m,'Seed','seed.json');await unlink(join(m.directory,'seed.json'));m.seeded=true;m.status='seeded';await save(m);
   if(currentMode(m))await initializeCurrentApplication(m);
   await startApplications(m);await health(m);m.status='ready';await save(m);console.log(`READY ${m.origins.web} manifest=${join(m.directory,'manifest.json')}`);return m;
  } catch(e) {
   m.status='failed';m.failure=e.message;await save(m);console.error(`FAILED ${e.message}; run=${m.runId}`);
   try{await cleanup(m)}catch{console.error(`CLEANUP_INCOMPLETE: retry ${currentMode(m)?'destroy':'stop'} --run ${m.runId}`)}throw e;
  }
 });
}

try {
 if(action==='start'&&!arg('--run'))await fresh();
 else {
  const m=await load(arg('--run'));
  await lock(m,async()=>{
   if(action==='stop'){
    if(currentMode(m)){
     const evidence=await stopCurrentApplications(m);console.log(JSON.stringify(evidence));return;
    }
    const evidence=m.status==='stopped'&&!m.secrets?await readJSON(join(m.directory,'cleanup.json')):await cleanup(m);
    console.log(JSON.stringify(evidence));if(!evidence.passed)process.exitCode=1;return;
   }
   if(action==='destroy'){const evidence=await cleanup(m);console.log(JSON.stringify(evidence));if(!evidence.passed)process.exitCode=1;return}
   if(currentMode(m)&&(action==='source-permission-revoke'||action==='source-permission-restore')){assert.ok(['stopped','start-failed'].includes(m.status),'APPLICATIONS_MUST_BE_STOPPED');await sourceAccountPermission(m,action.endsWith('revoke')?'revoke':'restore');console.log('CONTROL_OK source-permission');return}
   if(currentMode(m)&&(action==='provider-stop'||action==='provider-start')){assert.ok(['ready','stopped','start-failed'].includes(m.status),'RUN_NOT_CONTROLLABLE');await inventory(m,true);await run('docker',['container',action==='provider-stop'?'stop':'start',m.resources[`${m.project}-zitadel-api`].id]);console.log(`CONTROL_OK ${action}`);return}
   if(action==='start'&&currentMode(m)&&['stopped','start-failed'].includes(m.status)){await startCurrentApplications(m);console.log(`READY ${m.origins.web}`);return}
   assert.equal(m.status,'ready','RUN_NOT_READY');
   if(action==='check'){console.log(JSON.stringify(await health(m)));return}
   if(action==='start'){await assertFingerprint(m);await health(m);console.log(`READY ${m.origins.web}`);return}
   if(action==='revoke'||action==='restore'){await inventory(m,true);await authorizationControl(m,arg('--user'),arg('--org'),action);console.log(`CONTROL_OK ${action}`);return}
   if(action==='provider-stop'||action==='provider-start') {await inventory(m,true);await run('docker',['container',action==='provider-stop'?'stop':'start',m.resources[`${m.project}-zitadel-api`].id]);console.log(`CONTROL_OK ${action}`);return}
   if(action==='restart'){
    let operations;
    if(process.env.ISSUE357_TEST_RESTART_PLAN){const test=await import('./issue357/restart_test_control.mjs');operations=await test.restartTestOperations(m,process.env.ISSUE357_TEST_RESTART_PLAN)}
    else {
     await inventory(m,true);
     operations={assertFingerprint,save,stopApplications,
      restartContainers:currentMode(m)?async()=>{}:async()=>run('docker',['container','restart',...['identity-db','commercial-db','zitadel-api','zitadel-login','proxy'].map(s=>m.resources[`${m.project}-${s}`].id)]),
      waitProvider:async()=>waitForProvider(m.origins.issuer),
      startApplications,health};
    }
    await restartRuntime(m,operations);console.log(`READY ${m.origins.web}`);return;
   }
   throw new Error('INVALID_COMMAND');
  });
 }
} catch(e){console.error(e.message);process.exitCode=1}
