import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';

const repo = fileURLToPath(new URL('../../', import.meta.url));
const preload = new URL('fixtures/current_application_acceptance_preload.mjs', import.meta.url).href;
const cli = fileURLToPath(new URL('../../web/listingkit-ui/scripts/current-application-final-acceptance.mjs', import.meta.url));
const id = '12345678-1234-4234-8234-123456789abc';
function run(scenario) {
  return new Promise(resolve => execFile(process.execPath, ['--import', preload, cli, 'a'.repeat(40)], {
    cwd: repo, windowsHide: true, timeout: 15000, env: { ...process.env, ACCEPTANCE_FAULT: scenario },
  }, (error, stdout, stderr) => resolve({ code: error?.code ?? 0, stdout, stderr })));
}

for (const scenario of ['stdout-id', 'stderr-id', 'destroy-fails-start', 'no-id', 'invalid-id', 'ambiguous-id', 'split-id', 'truncated-id', 'wrong-path', 'git-run-id', 'missing-manifest', 'stale-manifest', 'wrong-owner', 'wrong-sha', 'wrong-id', 'success', 'destroy-fails-after-success', 'body-fails']) {
  test(`acceptance CLI: ${scenario}`, async () => {
    const result = await run(scenario);
    const output = result.stdout + result.stderr;
    assert.match(output, /FIXTURE_LOADED/);
    const owned = ['stdout-id', 'stderr-id', 'destroy-fails-start', 'success', 'destroy-fails-after-success', 'body-fails'].includes(scenario);
    const cleanup = owned && !scenario.startsWith('destroy-fails');
    assert.equal(result.code, scenario === 'success' ? 0 : 1, output);
    assert.deepEqual([...result.stdout.matchAll(/^FIXTURE_DESTROY (.+)$/gm)].map(match => match[1]), owned ? [id] : [], output);
    if (scenario !== 'success' && scenario !== 'destroy-fails-after-success') assert.match(result.stderr, /ACCEPTANCE_FAILED run=/);
    assert.equal(result.stdout.includes('DESTROYED owned run'), cleanup, output);
    assert.equal(result.stdout.includes('PASS RUN-1'), scenario === 'success', output);
    if (scenario.startsWith('destroy-fails')) assert.match(output, new RegExp(`DESTROY_INCOMPLETE run=${id}`));
    if (scenario === 'success') assert.ok(result.stdout.indexOf('DESTROYED') < result.stdout.indexOf('PASS RUN-1'), output);
    assert.doesNotMatch(output, /SYNTHETIC_SECRET_MUST_NOT_ESCAPE/);
  });
}
