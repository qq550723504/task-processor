import type { z } from "zod";
import { projectEndpoint } from "@/lib/contracts/project-center";
import { readBoundedStrictJSON } from "./strict-json-response";
export type ProjectScope={userId:string;organizationId:string};
export type Intent={path:string;method:"POST"|"PATCH";body:unknown;key:string;revision?:number};
export class ProjectError extends Error{constructor(public code:string){super(code);}}
export async function projectRequest<T>(scope:ProjectScope,path:string,schema:z.ZodType<T>,signal:AbortSignal,intent?:Intent):Promise<T>{
 const url="/api/workbench/projects"+path;if(!projectEndpoint(new URL(url,window.location.origin),intent?.method??"GET"))throw new ProjectError("INVALID_REQUEST");
 const headers:Record<string,string>={Accept:"application/json","X-Expected-User-ID":scope.userId,"X-Expected-Organization-ID":scope.organizationId};
 if(intent){headers["Content-Type"]="application/json";headers["Idempotency-Key"]=intent.key;if(intent.revision)headers["If-Match"]=String(intent.revision);}
 try{const response=await fetch(url,{method:intent?.method??"GET",headers,body:intent?JSON.stringify(intent.body):undefined,signal,credentials:"same-origin",cache:"no-store",redirect:"error"});const payload=await readBoundedStrictJSON(response,256<<10,signal);
  if(!response.ok){const code=(payload as {code?:unknown})?.code;throw new ProjectError(typeof code==="string"?code:intent?"OUTCOME_UNKNOWN":"DEPENDENCY_UNAVAILABLE");}
  const parsed=schema.safeParse(payload);if(!parsed.success)throw new ProjectError(intent?"OUTCOME_UNKNOWN":"INVALID_UPSTREAM_RESPONSE");return parsed.data;
 }catch(e){if(e instanceof ProjectError)throw e;throw new ProjectError(intent?"OUTCOME_UNKNOWN":"DEPENDENCY_UNAVAILABLE");}
}
