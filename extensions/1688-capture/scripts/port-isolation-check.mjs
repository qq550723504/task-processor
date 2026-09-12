// Executable regression: the occupied debugger port is another task-owned
// server, never a real user's debugger. The harness must not even query it.
import { createServer } from 'node:http';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';

let requests = 0;
const occupied = createServer((_request, response) => {
  requests++;
  response.writeHead(200, { 'Content-Type': 'application/json' });
  response.end('[]');
});
await new Promise((done, reject) => { occupied.once('error', reject); occupied.listen(4398, '127.0.0.1', done); });
try {
  let childError;
  try {
    const result = await promisify(execFile)(process.execPath,
      [fileURLToPath(new URL('browser-smoke.mjs', import.meta.url)), '--cdp', ...process.argv.slice(2)],
      { windowsHide: true, timeout: 60000, maxBuffer: 1024 * 1024 });
    console.log(result.stdout);
  } catch (error) { childError = error; }
  if (requests !== 0) throw Error(`HARNESS_QUERIED_OTHER_DEBUGGER: ${requests} requests`);
  if (childError) throw childError;
  console.log('PASS: occupied unrelated debugger received zero requests; only the task endpoint was used.');
} finally { await new Promise(done => occupied.close(done)); }
