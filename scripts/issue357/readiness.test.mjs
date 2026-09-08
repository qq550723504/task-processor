import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {waitForProvider} from './readiness.mjs';

test('provider readiness bounds requests that accept connections without responding',async()=>{
 const server=createServer(()=>{});
 await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)});
 const origin=`http://127.0.0.1:${server.address().port}`;
 const started=Date.now();
 try{
  await assert.rejects(waitForProvider(origin,{requestTimeout:25,overallTimeout:100}),/TIMEOUT:PROVIDER_RESTART/);
  assert.ok(Date.now()-started<1000,'provider readiness exceeded its bounded failure window');
 }finally{
  server.closeAllConnections();
  await new Promise(resolve=>server.close(resolve));
 }
});
