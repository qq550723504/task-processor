import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';

export const label = 'com.shuomi.issue357.run';
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
export function runDirectory(id) { assert.match(id,uuid,'INVALID_RUN'); return join(tmpdir(),'task-processor-issue357',id); }
export function childEnvironment(source=process.env) {
  const allowed = new Set(['path','systemroot','windir','comspec','pathext','temp','tmp','userprofile','localappdata','appdata','programfiles','programfiles(x86)','programdata','number_of_processors','processor_architecture','gopath','gocache','gomodcache']);
  return Object.fromEntries(Object.entries(source).filter(([key])=>allowed.has(key.toLowerCase())));
}
export const secret = () => `Z9!${randomBytes(24).toString('hex')}`;
export function makeManifest(runId,ports,sourceSha,webSha) {
  const directory=runDirectory(runId);
  return {schemaVersion:'issue357-v1',runId,directory,project:`issue357-${runId}`,status:'creating',sourceSha,webSha,ports,
    origins:{issuer:`http://localhost:${ports.issuer}`,web:`http://localhost:${ports.web}`,go:`http://127.0.0.1:${ports.go}`},
    resources:{},organizations:{},users:{},authorizations:{},
    secrets:{identityDatabase:secret(),commercialDatabase:secret(),reader:secret(),masterkey:randomBytes(16).toString('hex'),auth:secret(),bootstrapPassword:secret()},
    createdAt:new Date().toISOString()};
}
export function validateManifest(m) {
  assert.equal(m.schemaVersion,'issue357-v1','INVALID_RUN');
  assert.equal(resolve(m.directory),resolve(runDirectory(m.runId)),'INVALID_RUN');
  assert.equal(m.project,`issue357-${m.runId}`,'INVALID_RUN');
  assert.equal(new Set(Object.values(m.ports)).size,4,'INVALID_RUN');
  for (const p of Object.values(m.ports)) assert.ok(Number.isInteger(p)&&p>1024&&p<=65535,'INVALID_RUN');
  assert.equal(m.origins.issuer,`http://localhost:${m.ports.issuer}`,'INVALID_RUN');
  assert.equal(m.origins.web,`http://localhost:${m.ports.web}`,'INVALID_RUN');
  assert.equal(m.origins.go,`http://127.0.0.1:${m.ports.go}`,'INVALID_RUN');
  return m;
}
export function ownedResource(m,r,name,id) {
  assert.equal(r.Id??r.ID??r.Name,id,'OWNERSHIP_MISMATCH');
  assert.equal(r.Name.replace(/^\//,''),name,'OWNERSHIP_MISMATCH');
  assert.equal((r.Labels??r.Config?.Labels)?.[label],m.runId,'OWNERSHIP_MISMATCH');
  return true;
}
