"use client";
import {useEffect,useRef,useState,type ReactNode} from "react";
import {useMutation,useQueryClient} from "@tanstack/react-query";
import type {z} from "zod";
import {useResourcePending} from "@/components/workbench/resources/resource-pending";
import {ecoPendingSchema,intentRoute} from "./pending";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {ecoRequest,ecoFileSchema,ecoCheckoutSchema,ecoResultSchema,ecoMerchantSchema,ecoFinancialSchema,EcoservicesError,type EcoScope} from "@/lib/api/ecoservices";
import "./ecoservices.css";

export function NonPaymentNotice(){return <Card role="status" style={{padding:20,marginBottom:20}}>当前开放资质申请、平台审核和协议确认。商户进件、服务草稿与发布及支付交易暂未开放；平台审核通过不会代替商户签约。</Card>}

export const categories=[{id:"COMPANY_REGISTRATION",name:"公司注册",group:"enterprise"},{id:"TRADEMARK_REGISTRATION",name:"商标注册",group:"enterprise"},{id:"STORE_OPENING",name:"店铺开通",group:"shop"},{id:"STORE_OPERATION",name:"店铺代运营",group:"shop"}] as const;
export const serviceStates:Record<string,string>={REQUESTED:"待服务商确认",QUOTED:"待确认报价",ORDER_PENDING:"待付款",PAID_READY:"待开始",SERVICING:"服务中",AWAITING_ACCEPTANCE:"待验收",ACCEPTED:"已验收",CANCEL_REQUESTED:"取消处理中",CANCELLED:"已取消"};
export const financialStates:Record<string,string>={AWAITING_PAYMENT:"等待付款",CREATED:"等待付款",PAID:"已收款",SETTLEMENT_PENDING:"分账处理中",SETTLED:"渠道结算已确认",REFUND_PENDING:"退款处理中",REFUNDED:"原路退款已确认",CANCELLATION_PENDING:"取消处理中",CLOSED_UNPAID:"未付款订单已关闭",WAITING_FUNDS:"等待原商户资金",RECONCILIATION_REQUIRED:"资金待核实",SOURCE_DENIED:"争议暂停资金操作",CHANNEL_OPERATION_FAILED:"原渠道操作失败",AUTOMATICALLY_RELEASED:"渠道自动解冻"};
export function yuan(minor:string){const v=BigInt(minor);return "¥"+(v/BigInt(100)).toString()+"."+(v%BigInt(100)).toString().padStart(2,"0")}
export function toMinor(raw:string){if(!/^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/.test(raw))throw new Error("请输入最多两位小数的金额");const [w,f=""]=raw.split(".");const v=BigInt(w)*BigInt(100)+BigInt(f.padEnd(2,"0"));if(v<=BigInt(0)||v>BigInt("9223372036854775807"))throw new Error("金额不正确");return v.toString()}
export function date(raw:string){return new Date(raw).toLocaleString("zh-CN",{year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit"})}
export function errorText(error:unknown){const code=error instanceof EcoservicesError?error.code:"";return ({ECOSERVICES_FORBIDDEN:"当前身份没有这项操作权限。",PERMISSION_DENIED:"当前身份没有这项操作权限。",ECOSERVICES_NOT_FOUND:"这项服务不存在，或不属于当前企业。",ECOSERVICES_CONFLICT:"状态或版本已更新，请刷新后确认。",ECOSERVICES_INVALID:"请检查填写内容、金额、版本和附件。",ORGANIZATION_CONTEXT_CHANGED:"企业已变化，请重新确认当前企业。",IDENTITY_CONTEXT_CHANGED:"登录身份已变化，请重新登录。",OUTCOME_UNKNOWN:"结果尚未确认，请重试原操作。不要重新提交不同内容。",ECOSERVICES_UNAVAILABLE:"生态服务暂时不可用，请稍后重试。"} as Record<string,string>)[code]??(error instanceof Error&&!code?error.message:"生态服务暂时不可用，请稍后重试。")}
export function EcoBoundary({children}:{children:(scope:EcoScope)=>ReactNode}){
 const c=useWorkbenchContext();const blocked=c.error||c.blockingError||c.isLoading||c.isSwitching;
 if(!c.user||!c.effectiveOrganization)return <ConsoleState kind={blocked?"loading":"unavailable"} title={blocked?"正在确认当前企业":"请先选择企业"}><Button onClick={()=>void c.retry()}>重新确认</Button></ConsoleState>;
 const scope={userId:c.user.id,organizationId:c.effectiveOrganization.id};
 return <>{blocked?<ConsoleState kind="loading" title="正在确认当前企业"/>:null}<div hidden={!!blocked} key={scope.userId+":"+scope.organizationId}>{children(scope)}</div></>;
}
export type EcoIntent={path:string;key:string;body:string|FormData;version?:string;output?:"file"|"checkout"|"merchant"|"financial";admin?:boolean;method?:"POST"|"PUT"};
export function useEcoCommands(scope:EcoScope){
 const client=useQueryClient(),context=useWorkbenchContext(),cacheKey=["ecoservices",scope.userId,scope.organizationId,"intent"];
 const liveContext=useRef(context);
 useEffect(()=>{liveContext.current=context},[context]);
 const pending=useResourcePending({expectedUserId:scope.userId,expectedOrganizationId:scope.organizationId},["ecoservices","json"],ecoPendingSchema,{storage:"local",maxLength:131072});
 const [ephemeral,setEphemeral]=useState<EcoIntent|null>(()=>client.getQueryData<EcoIntent>(cacheKey)??null),[message,setMessage]=useState("");
 const intent:EcoIntent|null=pending.command??ephemeral,running=useRef(false);
 const [resolved,setResolved]=useState<{intent:EcoIntent;data:unknown}|null>(null);
 function current(i:EcoIntent){
  const live=liveContext.current;
  if(live.user?.id!==scope.userId)throw new EcoservicesError("IDENTITY_CONTEXT_CHANGED");
  if(live.isLoading||live.isSwitching||live.error||live.blockingError||!i.admin&&live.effectiveOrganization?.id!==scope.organizationId)throw new EcoservicesError("ORGANIZATION_CONTEXT_CHANGED");
 }
 function clear(i:EcoIntent){
  if(typeof i.body==="string"&&ecoPendingSchema.safeParse(i).success)pending.clear(ecoPendingSchema.parse(i));
  client.removeQueries({queryKey:cacheKey,exact:true});setEphemeral(null);
 }
 const mutation=useMutation({mutationKey:["ecoservices",scope.userId,scope.organizationId,"write"],mutationFn:(i:EcoIntent)=>{
  current(i);
  const headers=new Headers();if(!(i.body instanceof FormData))headers.set("Content-Type","application/json");if(i.output!=="checkout")headers.set("Idempotency-Key",i.key);if(i.version)headers.set("If-Match",'"'+i.version+'"');
  const schema:z.ZodType<unknown>=i.output==="file"?ecoFileSchema:i.output==="checkout"?ecoCheckoutSchema:i.output==="merchant"?ecoMerchantSchema:i.output==="financial"?ecoFinancialSchema:ecoResultSchema;
  return ecoRequest(scope,i.path,schema,{method:i.method??"POST",headers,body:i.body},i.admin);
 },onSuccess:(data,i)=>{clear(i);setResolved({intent:i,data});setMessage("已保存，正在刷新真实状态。");void client.invalidateQueries({queryKey:["ecoservices",scope.userId,scope.organizationId]})},onError:(error,i)=>{
  setMessage(errorText(error));
  // These domain rejections prove that this exact operation was not applied.
  // Identity/org drift, revocation and all uncertain responses retain it.
  if(error instanceof EcoservicesError&&((error.code==="ECOSERVICES_INVALID"&&error.status===400)||(error.code==="ECOSERVICES_NOT_FOUND"&&error.status===404)||(error.code==="ECOSERVICES_CONFLICT"&&error.status===409)))clear(i);
 }});
 const registerSwitchGuard=context.registerOrganizationSwitchGuard;
 useEffect(()=>registerSwitchGuard(()=>!intent&&!mutation.isPending&&!pending.error),[registerSwitchGuard,intent,mutation.isPending,pending.error]);
 async function execute(i:EcoIntent){
  try{
   current(i);if(running.current||!pending.ready)throw new EcoservicesError("ECOSERVICES_CONFLICT");
   const route=intentRoute(i);if(!route||route.path!==i.path||route.admin!==!!i.admin)throw new EcoservicesError("ECOSERVICES_INVALID",400);
   const durable=!route.upload&&route.output!==ecoMerchantSchema;
   if(durable&&!navigator.locks)throw new Error("当前浏览器暂不支持安全提交，请使用当前版本的 Edge 浏览器。");
   const run=async()=>{
    current(i);if(running.current)throw new EcoservicesError("ECOSERVICES_CONFLICT");
    const saved=pending.read(),cached=client.getQueryData<EcoIntent>(cacheKey);
    if(saved&&(typeof i.body!=="string"||JSON.stringify(saved)!==JSON.stringify(ecoPendingSchema.parse(i)))||cached&&cached!==i)throw new EcoservicesError("ECOSERVICES_CONFLICT");
    if(durable){pending.persist(ecoPendingSchema.parse(i))}else{client.setQueryData(cacheKey,i);setEphemeral(i)}
    running.current=true;setMessage("");try{return await mutation.mutateAsync(i)}finally{running.current=false}
   };
   if(durable)return await navigator.locks.request(pending.storageKey,{ifAvailable:true},lock=>{if(!lock)throw new EcoservicesError("ECOSERVICES_CONFLICT");return run()});
   return await run();
  }catch(error){setMessage(errorText(error));throw error}
 }
 const json=(path:string,value:unknown,version?:string,admin=false,method:"POST"|"PUT"="POST")=>execute({path,key:crypto.randomUUID(),body:JSON.stringify(value),version,admin,method});
 const storageMessage=pending.error?"无法读取已保存的待确认操作，已暂停新的提交。请保留浏览器数据并联系平台处理。":"";
 const notice=message||intent||storageMessage?<Card className="eco-notice" role="status"><p>{storageMessage||message||"原操作尚未确认，请重试原操作。"}</p>{intent&&!mutation.isPending&&!pending.error?<Button variant="outline" onClick={()=>void execute(intent).catch(()=>undefined)}>重试原操作</Button>:null}</Card>:null;
 return {execute,json,intent,notice,resolved,locked:!pending.ready||!!intent||mutation.isPending,message,setMessage};
}
export type EcoCommands=ReturnType<typeof useEcoCommands>;
export function EcoPagination({page,total,onPage}:{page:number;total:string;onPage:(page:number)=>void}){return <div className="eco-pagination"><Button variant="outline" disabled={page===1} onClick={()=>onPage(page-1)}>上一页</Button><span>第 {page} 页 · 共 {total} 项</span><Button variant="outline" disabled={BigInt(page*20)>=BigInt(total)} onClick={()=>onPage(page+1)}>下一页</Button></div>}
export function ReadFailure({error,retry}:{error:unknown;retry:()=>void}){return <ConsoleState kind="error" title={errorText(error)}><Button variant="outline" onClick={retry}>重新读取</Button></ConsoleState>}
