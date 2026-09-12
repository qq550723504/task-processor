import { ACQUISITION_BASE, ACQUISITION_RESPONSE_MAX_BYTES, acquisitionRequestSchema, acquisitionResultSchema, acquisitionErrorStatuses, canonical1688Source, isAcquisitionUUID, type AcquisitionResult } from "../contracts/product-acquisition";

export type AcquisitionContext = Readonly<{ userId: string; organizationId: string }>;
export type AcquisitionOperation = Readonly<AcquisitionContext & { key: string; source: string }>;
class AcquisitionAPIError extends Error {
  constructor(public readonly code: string, public readonly status: number) { super(code); }
}
export async function acquire1688(operation: AcquisitionOperation, signal?: AbortSignal): Promise<AcquisitionResult> {
  return submit(operation, false, signal);
}
export async function verify1688(operation: AcquisitionOperation, signal?: AbortSignal): Promise<AcquisitionResult> {
  return submit(operation, true, signal);
}
export async function readAcquisition(operationId: string, context: AcquisitionContext, signal?: AbortSignal): Promise<AcquisitionResult> {
  if (!isAcquisitionUUID(operationId)) throw new AcquisitionAPIError("INVALID_REQUEST",400);
  return send(`${ACQUISITION_BASE}/${operationId}`,context,{signal,operationId});
}

function submit(operation: AcquisitionOperation, verify: boolean, signal?: AbortSignal) {
  const parsed=acquisitionRequestSchema.safeParse({source:operation.source});
  if (!parsed.success || !isAcquisitionUUID(operation.key)) throw new AcquisitionAPIError("INVALID_REQUEST",400);
  const canonical=canonical1688Source(parsed.data.source)!;
  const offer=/\/offer\/([0-9]+)\.html$/.exec(canonical)![1];
  return send(`${ACQUISITION_BASE}${verify ? "/verify" : ""}`,operation,{signal,key:operation.key,body:JSON.stringify(parsed.data),productKey:`crawler:1688:${offer}`});
}

// No retry loop and no key allocation: callers retain the original intent.
async function send(path:string,context:AcquisitionContext,options:{signal?:AbortSignal;key?:string;body?:string;operationId?:string;productKey?:string}):Promise<AcquisitionResult>{
  const safe=/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
  if (!safe.test(context.userId) || !safe.test(context.organizationId)) throw new AcquisitionAPIError("INVALID_REQUEST",400);
  if (options.signal?.aborted) throw new AcquisitionAPIError("DEADLINE_EXCEEDED",504);
  const controller=new AbortController();const abort=()=>controller.abort();
  options.signal?.addEventListener("abort",abort,{once:true});const timeout=setTimeout(abort,25000);
  const unknown=()=>new AcquisitionAPIError(options.body===undefined?"ACQUISITION_UNAVAILABLE":"OUTCOME_UNKNOWN",503);
  try{
    const headers=new Headers({Accept:"application/json","X-Expected-Organization-ID":context.organizationId,"X-Expected-User-ID":context.userId});
    if(options.body!==undefined){headers.set("Content-Type","application/json");headers.set("Idempotency-Key",options.key!);}
    const response=await fetch(path,{method:options.body===undefined?"GET":"POST",headers,body:options.body,credentials:"same-origin",cache:"no-store",redirect:"error",signal:controller.signal});
    if(!/^application\/json(?:\s*;|$)/i.test(response.headers.get("content-type")??"")) throw unknown();
    const payload=await boundedJSON(response,controller.signal);
    if(!response.ok){
      const code=payload && typeof payload==="object" && "code" in payload && typeof payload.code==="string" ? payload.code : "";
      if(acquisitionErrorStatuses[code]===response.status) throw new AcquisitionAPIError(code,response.status);
      throw unknown();
    }
    const parsed=acquisitionResultSchema.safeParse(payload);
    if(response.status!==200 || !parsed.success || (options.operationId!==undefined && parsed.data.operationId!==options.operationId) || (options.productKey!==undefined && parsed.data.outcome==="published" && parsed.data.productKey!==options.productKey)) throw unknown();
    return parsed.data;
  }catch(error){if(error instanceof AcquisitionAPIError)throw error;throw unknown();}
  finally{clearTimeout(timeout);options.signal?.removeEventListener("abort",abort);}
}

async function boundedJSON(response:Response,signal:AbortSignal):Promise<unknown>{
  const declared=Number(response.headers.get("content-length")??0);
  if(declared>ACQUISITION_RESPONSE_MAX_BYTES){await response.body?.cancel();throw new Error("oversize");}
  const reader=response.body?.getReader();if(!reader)throw new Error("empty response");
  const cancel=()=>void reader.cancel().catch(()=>undefined);signal.addEventListener("abort",cancel,{once:true});
  const chunks:Uint8Array[]=[];let length=0;
  try{while(true){signal.throwIfAborted();const item=await reader.read();signal.throwIfAborted();if(item.done)break;length+=item.value.length;if(length>ACQUISITION_RESPONSE_MAX_BYTES){await reader.cancel();throw new Error("oversize");}chunks.push(item.value);}}
  finally{signal.removeEventListener("abort",cancel);reader.releaseLock();}
  const bytes=new Uint8Array(length);let offset=0;for(const chunk of chunks){bytes.set(chunk,offset);offset+=chunk.length;}
  return JSON.parse(new TextDecoder("utf-8",{fatal:true}).decode(bytes));
}
