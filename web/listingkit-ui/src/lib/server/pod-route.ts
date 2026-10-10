import {NextRequest} from "next/server";
import {serverAuth} from "@/auth";
import {readZitadelIdentityFromSession} from "./zitadel-auth";
import {readZitadelServerAccessToken} from "./zitadel-server-token";
import {proxyPOD,podFailure} from "./pod-proxy";
const authenticated=serverAuth(async(request:NextRequest&{auth?:unknown})=>{const identity=readZitadelIdentityFromSession(request.auth as never);return proxyPOD(request,readZitadelServerAccessToken(request.auth as never),String(identity?.userId??""))});
export async function handlePOD(request:NextRequest){const controller=new AbortController(),abort=()=>controller.abort();request.signal.addEventListener("abort",abort,{once:true});if(request.signal.aborted)abort();const timer=setTimeout(abort,35000);let finish=()=>{};const failure=()=>podFailure(504,request.method==="GET"?"DEPENDENCY_UNAVAILABLE":"OUTCOME_UNKNOWN"),ended=new Promise<Response>(resolve=>{finish=()=>resolve(failure());controller.signal.addEventListener("abort",finish,{once:true});if(controller.signal.aborted)finish()});try{return(await Promise.race([authenticated(new NextRequest(request,{signal:controller.signal}),{params:Promise.resolve({})}),ended]))??podFailure(503,"DEPENDENCY_UNAVAILABLE")}catch{return failure()}finally{clearTimeout(timer);request.signal.removeEventListener("abort",abort);controller.signal.removeEventListener("abort",finish)}}
export const rejectPOD=()=>podFailure(405,"INVALID_REQUEST");
