"use client";
import {useEffect,useRef,useState} from "react";
import {z} from "zod";
import {useResourcePending} from "@/components/workbench/resources/resource-pending";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {Button} from "@/components/ui/button";
import {MarketAPIError,uploadQualification,type MarketScope} from "@/lib/api/supply-market";
import {marketID} from "@/lib/contracts/supply-market";
import {marketError} from "./shared";
const fileIntent=z.object({key:marketID,hash:z.string().regex(/^[a-f0-9]{64}$/),size:z.number().int().positive().max(20*1024*1024),type:z.enum(["image/jpeg","image/png","application/pdf"])}).strict();
export function Qualifications({scope,ids,onIDs,onLocked}:{scope:MarketScope;ids:string[];onIDs:(ids:string[])=>void;onLocked:(locked:boolean)=>void}){
 const context=useWorkbenchContext(),live=useRef(context),running=useRef(false);useEffect(()=>{live.current=context},[context]);
 const pending=useResourcePending({expectedUserId:scope.userId,expectedOrganizationId:scope.organizationId},["supply-market","qualification"],fileIntent,{storage:"local"});
 const [file,setFile]=useState<File>(),[busy,setBusy]=useState(false),[error,setError]=useState("");
 const register=context.registerOrganizationSwitchGuard;useEffect(()=>register(()=>!running.current&&!pending.command&&!pending.error),[register,pending.command,pending.error]);
 const locked=busy||!pending.ready||pending.error||!!pending.command;useEffect(()=>onLocked(locked),[locked,onLocked]);
 async function upload(){if(!file||running.current||!pending.ready||pending.error||ids.length>=6)return;setBusy(true);running.current=true;setError("");
  try{if(!["image/jpeg","image/png","application/pdf"].includes(file.type)||file.size>20*1024*1024||file.size===0)throw new Error("仅支持 20 MiB 内的 JPG、PNG、PDF。");const raw=await file.arrayBuffer();const hash=Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",raw)),v=>v.toString(16).padStart(2,"0")).join("");const original=pending.read();if(original&&(original.hash!==hash||original.size!==file.size||original.type!==file.type))throw new Error("请重新选择原文件，不能替换待确认的附件。");const intent=original??fileIntent.parse({key:crypto.randomUUID(),hash,size:file.size,type:file.type});if(!navigator.locks)throw new Error("请使用当前版本的 Edge 浏览器。");await navigator.locks.request(pending.storageKey,{ifAvailable:true},async lock=>{if(!lock)throw new Error("另一页面正在提交附件。");const c=live.current;if(c.isLoading||c.isSwitching||c.error||c.blockingError||c.user?.id!==scope.userId||c.effectiveOrganization?.id!==scope.organizationId)throw new MarketAPIError("ORGANIZATION_CONTEXT_CHANGED",409);pending.persist(intent);const receipt=await uploadQualification(scope,intent.key,file);if(receipt.sha256!==hash||receipt.size!==file.size||receipt.contentType!==file.type)throw new MarketAPIError("OUTCOME_UNKNOWN",409);pending.clear(intent);onIDs([...new Set([...ids,receipt.id])]);setFile(undefined)})
  }catch(e){setError(e instanceof MarketAPIError?marketError(e):e instanceof Error?e.message:"附件未确认")}finally{running.current=false;setBusy(false)}
 }
 return <fieldset className="space-y-2"><legend>私密资质附件</legend><p className="text-sm text-muted-foreground">JPG、PNG、PDF；每件最多 20 MiB，最多 6 件，总计最多 60 MiB。仅申请人和获授权专员可读取。</p>{ids.map((id,i)=><div key={id} className="flex items-center gap-2"><span>已保存附件 {i+1}</span><Button type="button" variant="ghost" disabled={locked} onClick={()=>onIDs(ids.filter(x=>x!==id))}>从本次申请移除</Button></div>)}
  <input type="file" aria-label="选择资质附件" accept="image/png,image/jpeg,application/pdf" disabled={busy||ids.length>=6} onChange={e=>setFile(e.target.files?.[0])}/><Button type="button" variant="outline" disabled={!file||busy||!pending.ready||pending.error||ids.length>=6} onClick={()=>void upload()}>{pending.command?"核实原文件并继续保存":"保存附件"}</Button>{pending.command?<p role="status">原附件结果待确认，请重新选择同一文件后继续。不会删除已有文件。</p>:null}{error?<p role="alert">{error}</p>:null}
 </fieldset>;
}
