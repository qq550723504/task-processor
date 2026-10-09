import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { proxyEcoservices,ecoservicesFailure } from "./ecoservices-proxy";
const authenticated=serverAuth(async(request:NextRequest & {auth?:unknown})=>{const identity=readZitadelIdentityFromSession(request.auth as never);return proxyEcoservices(request,readZitadelServerAccessToken(request.auth as never),String(identity?.userId??""))});
export async function handleEcoservices(request:NextRequest){
 const controller=new AbortController();const abort=()=>controller.abort();request.signal.addEventListener("abort",abort,{once:true});if(request.signal.aborted)abort();const timeout=()=>ecoservicesFailure(504,request.method==="GET"?"DEADLINE_EXCEEDED":"OUTCOME_UNKNOWN");const timer=setTimeout(abort,40000);let finish=()=>{};
 const ended=new Promise<Response>(resolve=>{finish=()=>resolve(timeout());controller.signal.addEventListener("abort",finish,{once:true});if(controller.signal.aborted)finish()});
 try{const response=await Promise.race([authenticated(new NextRequest(request,{signal:controller.signal}),{params:Promise.resolve({})}),ended]);return controller.signal.aborted?timeout():response??ecoservicesFailure(503,"ECOSERVICES_UNAVAILABLE")}catch{return controller.signal.aborted?timeout():ecoservicesFailure(503,"ECOSERVICES_UNAVAILABLE")}finally{clearTimeout(timer);request.signal.removeEventListener("abort",abort);controller.signal.removeEventListener("abort",finish)}
}
export const rejectEcoservices=()=>ecoservicesFailure(405,"ECOSERVICES_INVALID");
