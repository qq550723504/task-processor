// Run-private adapter to Next's documented custom-server API, for graceful
// Windows stop. All routing/authentication remains the copied current Console.
import {createServer} from 'node:http';
import {createRequire} from 'node:module';
import {existsSync} from 'node:fs';
import {join} from 'node:path';
import {readJSON,json} from './io.mjs';

const dir=process.argv[2];const cfg=await readJSON(join(dir,'services.json'));
const require=createRequire(join(cfg.uiDirectory,'package.json'));
const next=require('next');
const app=next({dev:true,dir:cfg.uiDirectory,hostname:'localhost',port:cfg.webPort,webpack:true,quiet:true});
await app.prepare();
const handler=app.getRequestHandler();
const server=createServer((req,res)=>handler(req,res));
await new Promise((r,reject)=>{server.once('error',reject);server.listen(cfg.webPort,'127.0.0.1',r)});
await json(join(dir,'next-ready.json'),{port:cfg.webPort});
let closing=false;
async function stop(){if(closing)return;closing=true;clearInterval(timer);server.closeAllConnections();await new Promise(r=>server.close(r));await app.close();process.exit(0)}
const timer=setInterval(()=>{if(existsSync(join(dir,'stop-next')))void stop()},200);
process.on('SIGINT',()=>void stop());process.on('SIGTERM',()=>void stop());
