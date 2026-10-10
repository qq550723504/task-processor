"use client";
import {useEffect,useRef,useState} from "react";
import {useQueryClient} from "@tanstack/react-query";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {useResourcePending} from "@/components/workbench/resources/resource-pending";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import {MarketAPIError,type MarketScope} from "@/lib/api/supply-market";
import {writePOD,resolvePOD} from "@/lib/api/pod";
import {podIntentSchema,type PODIntent,type PODResult} from "@/lib/contracts/pod";
import {marketError} from "../supply-market/shared";

type NewIntent=PODIntent extends infer I?I extends {key:string}?Omit<I,"key">:never:never;
export function usePODCommands(scope:MarketScope){
 const context=useWorkbenchContext(),client=useQueryClient();
 const live=useRef(context),active=useRef(false),alive=useRef(true),controller=useRef<AbortController|null>(null);
 useEffect(()=>{live.current=context},[context]);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;controller.current?.abort()}},[]);
 const pending=useResourcePending({expectedUserId:scope.userId,expectedOrganizationId:scope.organizationId},["pod"],podIntentSchema,{storage:"local",maxLength:65536});
 const [busy,setBusy]=useState(false),[error,setError]=useState<unknown>(null);
 const [saved,setSaved]=useState<{intent:PODIntent;result:PODResult}|null>(null);
 const register=context.registerOrganizationSwitchGuard;
 useEffect(()=>register(()=>!active.current&&!pending.command&&!pending.error),[register,pending.command,pending.error]);
 function current(){const c=live.current;if(c.user?.id!==scope.userId)throw new MarketAPIError("IDENTITY_CONTEXT_CHANGED",409);if(c.isLoading||c.isSwitching||c.error||c.blockingError||c.effectiveOrganization?.id!==scope.organizationId)throw new MarketAPIError("ORGANIZATION_CONTEXT_CHANGED",409)}
 async function dispatch(intent:PODIntent,verify:boolean){
  if(active.current||!pending.ready)return;
  const abort=new AbortController();controller.current=abort;active.current=true;setBusy(true);setError(null);
  try{current();if(!verify)pending.persist(intent);const result=await(verify?resolvePOD(scope,intent,abort.signal):writePOD(scope,intent,abort.signal));if(!alive.current)return;current();pending.clear(intent);setSaved({intent,result});void client.invalidateQueries({queryKey:["pod",scope.userId,scope.organizationId]});return result}
  catch(e){if(alive.current){setError(e);if(!verify&&e instanceof MarketAPIError&&(e.code==="INVALID_REQUEST"&&e.status===400||e.code==="REVISION_CONFLICT"&&e.status===409)){try{pending.clear(intent)}catch{setError(new MarketAPIError("OUTCOME_UNKNOWN",409))}}}}
  finally{active.current=false;if(alive.current)setBusy(false);if(controller.current===abort)controller.current=null}
 }
 async function lockedRun(run:()=>Promise<PODResult|undefined>){
  if(!navigator.locks){setError(new MarketAPIError("DEPENDENCY_UNAVAILABLE",503));return}
  try{return await navigator.locks.request(pending.storageKey,{ifAvailable:true},lock=>lock?run():Promise.resolve(undefined))}catch{setError(new MarketAPIError("OUTCOME_UNKNOWN",409))}
 }
 async function execute(input:NewIntent){
  if(pending.command||active.current||!pending.ready)return;
  const key=input.kind==="approval"?input.body.actionId:crypto.randomUUID();const intent=podIntentSchema.safeParse({...input,key});
  if(!intent.success){setError(new MarketAPIError("INVALID_REQUEST",400));return}
  return lockedRun(async()=>pending.read()?undefined:dispatch(intent.data,false));
 }
 const verify=()=>lockedRun(async()=>{const original=pending.read();return original?dispatch(original,true):undefined});
 const notice=pending.command||pending.error||error?<Card className="space-y-3 p-4" role="status"><p>{pending.error?"无法读取原操作，已暂停新提交。请保留浏览器数据。":error?marketError(error):"原操作的结果尚未确认，请先查询原回执。"}</p>{pending.command&&!pending.error?<Button variant="outline" disabled={busy} onClick={()=>void verify()}>核实原操作</Button>:null}</Card>:null;
 return {execute,verify,busy,saved,notice,locked:!pending.ready||!!pending.command||busy||pending.error};
}
