import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { proxyKnowledge,knowledgeFailure } from "./knowledge-proxy";
const authenticated=serverAuth(async(request:NextRequest & {auth?:unknown})=>{
 const identity=readZitadelIdentityFromSession(request.auth as never);
 return proxyKnowledge(request,readZitadelServerAccessToken(request.auth as never),String(identity?.userId??""));
});
export async function handleKnowledge(request:NextRequest){
 const controller=new AbortController();const abort=()=>controller.abort();
 request.signal.addEventListener("abort",abort,{once:true});if(request.signal.aborted)abort();
 const timeout=()=>knowledgeFailure(504,request.method==="GET"?"DEADLINE_EXCEEDED":"OUTCOME_UNKNOWN");
 const timer=setTimeout(abort,45000);let finish=()=>{};
 const ended=new Promise<Response>(resolve=>{finish=()=>resolve(timeout());controller.signal.addEventListener("abort",finish,{once:true});if(controller.signal.aborted)finish();});
 try {const response=await Promise.race([authenticated(new NextRequest(request,{signal:controller.signal}),{params:Promise.resolve({})}),ended]);return controller.signal.aborted?timeout():response??knowledgeFailure(503,"KNOWLEDGE_UNAVAILABLE");}
 catch{return controller.signal.aborted?timeout():knowledgeFailure(503,"KNOWLEDGE_UNAVAILABLE");}
 finally{clearTimeout(timer);request.signal.removeEventListener("abort",abort);controller.signal.removeEventListener("abort",finish);}
}
export const rejectKnowledge=()=>knowledgeFailure(405,"KNOWLEDGE_INVALID_REQUEST");
