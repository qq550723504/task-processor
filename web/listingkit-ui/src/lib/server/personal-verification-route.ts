import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { accountFailure } from "./account-proxy";
import { proxyPersonalVerification } from "./personal-verification-proxy";

async function handle(request: NextRequest): Promise<Response> {
  const dispatch={forwarded:false};const controller=new AbortController();const abort=()=>controller.abort();
  request.signal.addEventListener("abort",abort,{once:true});const timer=setTimeout(abort,22000);
  const failure=()=>accountFailure(504,dispatch.forwarded?"RESULT_UNVERIFIED":"VERIFICATION_UNAVAILABLE",dispatch.forwarded?"unknown":undefined);
  let finish=()=>{};const ended=new Promise<Response>(resolve=>{finish=()=>resolve(failure());controller.signal.addEventListener("abort",finish,{once:true});});
  try {
    if(request.signal.aborted)abort();if(controller.signal.aborted)return failure();
    const authenticated=serverAuth(async(req: NextRequest & {auth?:unknown})=>{
      const identity=readZitadelIdentityFromSession(req.auth as never);if(!identity?.userId)return accountFailure(401,"AUTHENTICATION_REQUIRED");
      return proxyPersonalVerification(req,readZitadelServerAccessToken(req.auth as never),String(identity.userId),dispatch);
    });
    return await Promise.race([authenticated(new NextRequest(request,{signal:controller.signal}),{params:Promise.resolve({})}),ended])??accountFailure(503,"VERIFICATION_UNAVAILABLE");
  } catch {return dispatch.forwarded?accountFailure(502,"RESULT_UNVERIFIED","unknown"):accountFailure(503,"VERIFICATION_UNAVAILABLE");}
  finally {clearTimeout(timer);request.signal.removeEventListener("abort",abort);controller.signal.removeEventListener("abort",finish);}
}
export const GET=handle;
export const POST=handle;
