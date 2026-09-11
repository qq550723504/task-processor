import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { childEnvironment, runDirectory, validateManifest, ownedResource, makeManifest } from './contract.mjs';
import { composeConfiguration } from './compose.mjs';

test('child processes cannot inherit IAM, DB, proxy, NODE_OPTIONS or mock settings', () => {
  const env = childEnvironment({ Path: 'tools', SystemRoot: 'Windows', TEMP: 'temp', ZITADEL_CLIENT_SECRET: 'foreign', DATABASE_URL: 'foreign', HTTP_PROXY: 'foreign', NODE_OPTIONS: '--require foreign', LISTINGKIT_UI_USE_MOCK: 'true' });
  assert.deepEqual(env, { Path: 'tools', SystemRoot: 'Windows', TEMP: 'temp' });
});
test('run identifiers cannot escape the fixed private root', () => {
  const id = randomUUID();
  assert.equal(runDirectory(id), join(tmpdir(), 'task-processor-issue357', id));
  for (const bad of ['', '..', '../foreign', 'local-zitadel', 'C:\\shared', id.toUpperCase()]) assert.throws(() => runDirectory(bad));
});
function manifest() { return makeManifest(randomUUID(), { issuer: 42101, web: 42102, go: 42103, database: 42104 }, 'a'.repeat(40), 'b'.repeat(40)); }
test('runtime configuration is bound to manifest-assigned loopback ports and project', () => {
  const m = manifest();
  assert.equal(validateManifest(m).runId, m.runId);
  assert.equal(validateManifest({...m,runtimeMode:'current-application'}).runtimeMode,'current-application');
  for (const patch of [ {runId:'local-zitadel'}, {project:'local-zitadel'}, {runtimeMode:'legacy-compatibility'}, {origins:{...m.origins,issuer:'https://real.example'}}, {origins:{...m.origins,web:'http://localhost:3000'}}, {ports:{...m.ports,database:m.ports.go}}, {directory:tmpdir()}, {schemaVersion:'fixture-v0'} ]) assert.throws(() => validateManifest({...m,...patch}));
});
test('resource deletion needs run label, exact expected name and recorded ID', () => {
  const m = manifest(); const name = `${m.project}-identity-db`;
  const r = {Id:'actual-id',Name:name,Labels:{'com.shuomi.issue357.run':m.runId}};
  assert.equal(ownedResource(m, r, name, 'actual-id'), true);
  for (const bad of [{...r,Id:'reused-id'}, {...r,Name:'local-zitadel'}, {...r,Labels:{}}, {...r,Labels:{'com.shuomi.issue357.run':randomUUID()}}]) assert.throws(() => ownedResource(m,bad,name,'actual-id'));
});
test('generated Compose has no host discovery, external storage or public bind', () => {
  const m = manifest(); const c = composeConfiguration(m);
  assert.equal(c.name,m.project);
  assert.deepEqual(Object.keys(c.services).sort(), ['commercial-db','identity-db','proxy','zitadel-api','zitadel-login']);
  for (const s of Object.values(c.services)) {
    assert.equal(s.labels['com.shuomi.issue357.run'],m.runId);
    assert.equal(s.restart,'no');
    for (const p of s.ports ?? []) assert.equal(p.host_ip,'127.0.0.1');
    for (const v of s.volumes ?? []) assert.ok(!JSON.stringify(v).includes('docker.sock'));
  }
  assert.equal(c.services['identity-db'].ports,undefined);
  assert.equal(c.services['zitadel-api'].ports,undefined);
  assert.equal(c.services['zitadel-login'].ports,undefined);
  assert.ok(!JSON.stringify(c).includes('local-zitadel'));
  assert.ok(!JSON.stringify(c.services['zitadel-login']).includes('setup-secret'));
  for (const v of Object.values(c.volumes)) { assert.equal(v.external,undefined); assert.equal(v.labels['com.shuomi.issue357.run'],m.runId); }
});
