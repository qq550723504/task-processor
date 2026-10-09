"use client";
import {useEffect,useRef,useState,type ReactNode} from "react";
import {useQueryClient} from "@tanstack/react-query";
import Link from "next/link";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {useResourcePending} from "@/components/workbench/resources/resource-pending";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {MarketAPIError,marketIntentSchema,writeMarket,resolveMarket,type MarketScope,type MarketIntent} from "@/lib/api/supply-market";
import type {MarketCommand,MarketReceipt} from "@/lib/contracts/supply-market";
export const marketStages={DRAFT:"待发布",SUBMITTED:"已提交",EVALUATING:"人工评估中",SUPPLEMENT_REQUIRED:"待补充资料",APPROVED:"审核通过",REJECTED:"未通过",PLAN_CONFIRMED:"对接方案已确认",CLOSED:"已结束"};
export function marketError(e:unknown){const code=e instanceof MarketAPIError?e.code:"";return ({PERMISSION_DENIED:"当前身份没有这项权限。",NOT_FOUND:"记录不存在、已撤销或不属于当前成员。",REVISION_CONFLICT:"商品或申请已变化，请刷新后重新确认。",INVALID_REQUEST:"请检查商品、供货声明和附件。",OUTCOME_UNKNOWN:"结果尚未确认，已保留原操作。请先核实。",IDENTITY_CONTEXT_CHANGED:"登录身份已变化，请重新登录。",ORGANIZATION_CONTEXT_CHANGED:"企业已变化，请重新确认。"} as Record<string,string>)[code]??"供应市场暂时不可用，请稍后重新读取。"}
export function MarketBoundary({children,admin=false}:{children:(scope:MarketScope)=>ReactNode;admin?:boolean}){
 const c=useWorkbenchContext();if(c.isLoading||c.isSwitching||c.error||c.blockingError)return <ConsoleState kind="loading" title="正在确认当前身份"/>;
 if(!c.user||!admin&&!c.effectiveOrganization)return <ConsoleState kind="unavailable" title="请先登录并选择企业"><Button onClick={()=>void c.retry()}>重新确认</Button></ConsoleState>;
 const scope={userId:c.user.id,organizationId:admin?"":c.effectiveOrganization!.id};return <div key={JSON.stringify(scope)}>{children(scope)}</div>;
}
export function useMarketCommands(scope:MarketScope,admin=false){
 const context=useWorkbenchContext(),client=useQueryClient();const live=useRef(context),active=useRef(false),controller=useRef<AbortController|null>(null),alive=useRef(true);
 useEffect(()=>{live.current=context},[context]);useEffect(()=>{alive.current=true;return()=>{alive.current=false;controller.current?.abort()}},[]);
 const pending=useResourcePending({expectedUserId:scope.userId,expectedOrganizationId:scope.organizationId},["supply-market",admin?"platform":"member"],marketIntentSchema,{storage:"local",maxLength:131072});
 const [busy,setBusy]=useState(false),[error,setError]=useState<unknown>(null),[saved,setSaved]=useState<MarketReceipt|null>(null);
 const register=context.registerOrganizationSwitchGuard;useEffect(()=>register(()=>!active.current&&!pending.command&&!pending.error),[register,pending.command,pending.error]);
 function current(){const c=live.current;if(c.user?.id!==scope.userId)throw new MarketAPIError("IDENTITY_CONTEXT_CHANGED",409);if(c.isLoading||c.isSwitching||c.error||c.blockingError||!admin&&c.effectiveOrganization?.id!==scope.organizationId)throw new MarketAPIError("ORGANIZATION_CONTEXT_CHANGED",409)}
 async function dispatch(i:MarketIntent,verify:boolean){
  if(active.current||!pending.ready)return;const abort=new AbortController();controller.current=abort;active.current=true;setBusy(true);setError(null);
  try{current();if(i.admin!==admin)throw new MarketAPIError("INVALID_REQUEST",400);if(!verify)pending.persist(i);const result=await(verify?resolveMarket(scope,i,abort.signal):writeMarket(scope,i,abort.signal));if(!alive.current)return;current();pending.clear(i);setSaved(result);void client.invalidateQueries({queryKey:["supply-market",scope.userId,scope.organizationId]});return result;
  }catch(e){if(alive.current){setError(e);if(!verify&&e instanceof MarketAPIError&&((e.code==="INVALID_REQUEST"&&e.status===400)||(e.code==="REVISION_CONFLICT"&&e.status===409)||(e.code==="NOT_FOUND"&&e.status===404))){try{pending.clear(i)}catch{setError(new MarketAPIError("OUTCOME_UNKNOWN",409))}}}}
  finally{active.current=false;if(alive.current)setBusy(false);if(controller.current===abort)controller.current=null}
 }
 async function execute(command:MarketCommand){if(pending.command||active.current||!pending.ready)return;const parsed=marketIntentSchema.safeParse({key:crypto.randomUUID(),command,admin});if(!parsed.success){setError(new MarketAPIError("INVALID_REQUEST",400));return}if(!navigator.locks){setError(new MarketAPIError("DEPENDENCY_UNAVAILABLE",503));return}try{return await navigator.locks.request(pending.storageKey,{ifAvailable:true},async lock=>{if(!lock||pending.read())return;return dispatch(parsed.data,false)})}catch{setError(new MarketAPIError("OUTCOME_UNKNOWN",409))}}
 const verify=()=>pending.command?dispatch(pending.command,true):Promise.resolve(undefined);
 const notice=pending.error||pending.command||error?<Card className="p-4" role="status"><p>{pending.error?"无法读取原操作，已暂停新提交。请保留浏览器数据。":error?marketError(error):"原操作待确认，请先核实。"}</p>{pending.command&&!busy&&!pending.error?<div className="flex gap-2"><Button variant="outline" onClick={()=>void verify()}>核实原操作</Button><Button variant="outline" onClick={()=>{if(!navigator.locks)return;void navigator.locks.request(pending.storageKey,{ifAvailable:true},async lock=>{if(lock&&pending.command)await dispatch(pending.command,false)}).catch(()=>setError(new MarketAPIError("OUTCOME_UNKNOWN",409)))}}>重试原操作</Button></div>:null}</Card>:null;
 return {execute,verify,busy,saved,notice,locked:!pending.ready||!!pending.command||busy||pending.error};
}
export function MarketReadError({error,retry}:{error:unknown;retry:()=>void}){return <ConsoleState kind="error" title={marketError(error)}><Button variant="outline" onClick={retry}>重新读取</Button></ConsoleState>}
export function MarketLinks(){return <nav className="flex flex-wrap gap-4" aria-label="供应市场"><Link href="/workbench/supply/official" prefetch={false}>硕米自营</Link><Link href="/workbench/supply/selected" prefetch={false}>硕米优选</Link><Link href="/workbench/supply/catalogs" prefetch={false}>货盘集成</Link><Link href="/workbench/supply/applications" prefetch={false}>优选申请</Link></nav>}
export function CursorButtons({after,next,total,onAfter}:{after?:string;next?:string;total:number;onAfter:(s?:string)=>void}){return <div className="flex items-center gap-4"><span>共 {total} 项</span>{after?<Button variant="outline" onClick={()=>onAfter(undefined)}>回到首页</Button>:null}<Button variant="outline" disabled={!next} onClick={()=>onAfter(next)}>下一页</Button></div>}
