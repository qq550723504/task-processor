// Test-only process boundary doubles. Never imported by the runtime or acceptance CLI.
import childProcess from 'node:child_process';
import fs from 'node:fs/promises';
import { registerHooks, syncBuiltinESMExports } from 'node:module';
import { promisify } from 'node:util';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { makeManifest } from '../contract.mjs';

const scenario = process.env.ACCEPTANCE_FAULT;
process.stdout.write('FIXTURE_LOADED\n');
const sha = 'a'.repeat(40);
const id = '12345678-1234-4234-8234-123456789abc';
const directory = path.join(tmpdir(), 'task-processor-issue357', id);
const manifest = makeManifest(id, { issuer: 12001, web: 12002, go: 12003, database: 12004 }, sha, sha);
Object.assign(manifest, {
  runtimeMode: 'current-application', sourceDirectory: process.cwd(),
  webDirectory: path.join(process.cwd(), 'web/listingkit-ui'),
  users: { admin: { id: 'admin', credentialFile: path.join(directory, 'admin.json') }, viewer: { id: 'viewer', credentialFile: path.join(directory, 'viewer.json') } },
  organizations: { B: { id: 'B' }, C: { id: 'C' } },
});
const secret = 'SYNTHETIC_SECRET_MUST_NOT_ESCAPE';
let destroyed = false;
let rejectStart = false;
let dependencyUnavailable = false;
const line = () => `runId=${id} manifest=${path.join(directory, 'manifest.json')}\n`;
function fail(stdout = '', stderr = secret) {
  throw Object.assign(new Error(`fixture command failed ${secret}`), { stdout, stderr, code: 1 });
}
childProcess.execFile = function () { throw new Error('unexpected callback invocation'); };
childProcess.execFile[promisify.custom] = async (_executable, args) => {
  const action = args[1];
  if (args[0] === 'rev-parse') {
    if (scenario === 'git-run-id') fail(line());
    return { stdout: sha, stderr: '' };
  }
  if (args[0] === 'status') return { stdout: '', stderr: '' };
  if (action === 'start' && !args.includes('--run')) {
    manifest.createdAt = new Date().toISOString();
    if (scenario === 'stale-manifest') manifest.createdAt = '2000-01-01T00:00:00Z';
    if (scenario === 'wrong-owner') manifest.sourceDirectory = path.join(process.cwd(), 'another-owner');
    if (scenario === 'wrong-sha') manifest.sourceSha = 'b'.repeat(40);
    if (scenario === 'wrong-id') manifest.runId = '22345678-1234-4234-8234-123456789abc';
    if (scenario === 'no-id') fail();
    if (scenario === 'invalid-id') fail(`runId=${'-'.repeat(36)} manifest=${directory}\n`);
    if (scenario === 'ambiguous-id') fail(line() + line());
    if (scenario === 'split-id') fail(line().slice(0, 20), line().slice(20));
    if (scenario === 'truncated-id') fail(`runId=${id}\n`);
    if (scenario === 'wrong-path') fail(`runId=${id} manifest=${path.join(directory, 'other.json')}\n`);
    if (scenario === 'stderr-id') fail('', line() + secret);
    if (!['success', 'destroy-fails-after-success', 'body-fails'].includes(scenario)) fail(line());
    return { stdout: line(), stderr: '' };
  }
  if (action === 'destroy') {
    process.stdout.write(`FIXTURE_DESTROY ${args.at(-1)}\n`);
    if (scenario.startsWith('destroy-fails')) fail();
    destroyed = true;
    return { stdout: JSON.stringify({ resourcesReleased: true }), stderr: '' };
  }
  if (scenario === 'body-fails') fail();
  if (['source-permission-revoke', 'source-cross-grant', 'commercial-cross-grant', 'provider-stop'].includes(action)) rejectStart = true;
  if (action === 'provider-stop') dependencyUnavailable = true;
  if (action === 'provider-start') dependencyUnavailable = false;
  if (action === 'start' && rejectStart) { rejectStart = false; fail(); }
  return { stdout: JSON.stringify({ passed: true, resourcesRetained: true, factsRetained: { resources: 3, operations: 6 } }), stderr: '' };
};
const originalRead = fs.readFile;
fs.realpath = async value => value;
fs.writeFile = async () => {};
fs.readFile = async (file, encoding) => {
  const name = path.basename(file);
  if (!String(file).startsWith(directory)) return originalRead(file, encoding);
  if ((destroyed && ['issue390-chain.json', 'user-token.txt'].includes(name)) || (scenario === 'missing-manifest' && name === 'manifest.json')) throw Object.assign(new Error('missing'), { code: 'ENOENT' });
  const values = { 'manifest.json': { ...manifest, status: 'start-failed' }, 'services.json': { binary: 'current-application.exe', goArgs: ['-config'] }, 'admin.json': { username: 'admin', password: secret }, 'viewer.json': { username: 'viewer', password: secret }, 'issue390-evidence.json': { passed: true } };
  const value = name === 'user-token.txt' ? secret : JSON.stringify(values[name] ?? {});
  return encoding ? value : Buffer.from(value);
};
globalThis.fetch = async (url, options) => ({ status: String(url).includes('/api/v1/') ? (options?.method === 'POST' ? 503 : 401) : dependencyUnavailable ? 503 : 200, text: async () => 'DEPENDENCY_UNAVAILABLE' });
syncBuiltinESMExports();

registerHooks({
  resolve(specifier, context, next) {
    return specifier === '@playwright/test' ? { url: 'fixture:playwright', shortCircuit: true } : next(specifier, context);
  },
  load(url, context, next) {
    if (url !== 'fixture:playwright') return next(url, context);
    return { format: 'module', shortCircuit: true, source: `
      export const chromium = { launch: async () => ({ close: async () => {}, newContext: async () => {
        let user; let request;
        return { close: async () => {}, cookies: async () => [{ name: 'authjs.session-token', value: 'fixture' }],
          request: { get: async () => ({ json: async () => ({ identity: { userId: user }, expiresAt: 0 }) }) },
          newPage: async () => ({ on: (_event, handler) => request = handler,
            goto: async () => { request({ url: () => 'http://localhost:12001/ui/v2/login' }); request({ url: () => 'http://localhost:12001/oauth/v2/authorize?response_type=code&code_challenge_method=S256' }); },
            getByTestId: id => ({ waitFor: async () => {}, fill: async value => { if (id === 'username-text-input') user = value; }, click: async () => {} }), waitForURL: async () => {} }) };
      } }) };` };
  },
});
