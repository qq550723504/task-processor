"use client";
import {useEffect,useRef,useState,type ReactNode} from "react";
import {useMutation,useQueryClient} from "@tanstack/react-query";
import type {z} from "zod";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {ecoRequest,ecoFileSchema,ecoCheckoutSchema,ecoResultSchema,ecoMerchantSchema,ecoFinancialSchema,EcoservicesError,type EcoScope} from "@/lib/api/ecoservices";
import "./ecoservices.css";

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
 const [intent,setIntent]=useState<EcoIntent|null>(()=>client.getQueryData<EcoIntent>(cacheKey)??null),[message,setMessage]=useState("");
 const original=useRef(intent),running=useRef(false);
 const [resolved,setResolved]=useState<{intent:EcoIntent;data:unknown}|null>(null);
 const clear=()=>{client.removeQueries({queryKey:cacheKey,exact:true});original.current=null;setIntent(null)};
 const mutation=useMutation({mutationKey:["ecoservices",scope.userId,scope.organizationId,"write"],mutationFn:(i:EcoIntent)=>{
  const headers=new Headers();if(!(i.body instanceof FormData))headers.set("Content-Type","application/json");if(i.output!=="checkout")headers.set("Idempotency-Key",i.key);if(i.version)headers.set("If-Match",'"'+i.version+'"');
  const schema:z.ZodType<unknown>=i.output==="file"?ecoFileSchema:i.output==="checkout"?ecoCheckoutSchema:i.output==="merchant"?ecoMerchantSchema:i.output==="financial"?ecoFinancialSchema:ecoResultSchema;
  return ecoRequest(scope,i.path,schema,{method:i.method??"POST",headers,body:i.body},i.admin);
 },onSuccess:(data,i)=>{setResolved({intent:i,data});clear();setMessage("已保存，正在刷新真实状态。");void client.invalidateQueries({queryKey:["ecoservices",scope.userId,scope.organizationId]})},onError:(error)=>{setMessage(errorText(error));if(!(error instanceof EcoservicesError)||error.code!=="OUTCOME_UNKNOWN")clear()}});
 const registerSwitchGuard=context.registerOrganizationSwitchGuard;
 useEffect(()=>registerSwitchGuard(()=>!intent&&!mutation.isPending),[registerSwitchGuard,intent,mutation.isPending]);
 async function execute(i:EcoIntent){if(context.isLoading||context.isSwitching||context.error||context.blockingError)throw new EcoservicesError("ORGANIZATION_CONTEXT_CHANGED");if(running.current||original.current&&original.current!==i)throw new EcoservicesError("ECOSERVICES_CONFLICT");original.current=i;running.current=true;client.setQueryData(cacheKey,i);setIntent(i);setMessage("");try{return await mutation.mutateAsync(i)}finally{running.current=false}}
 const json=(path:string,value:unknown,version?:string,admin=false,method:"POST"|"PUT"="POST")=>execute({path,key:crypto.randomUUID(),body:JSON.stringify(value),version,admin,method});
 const notice=message||intent?<Card className="eco-notice" role="status"><p>{message||"正在提交原操作…"}</p>{intent&&!mutation.isPending?<Button variant="outline" onClick={()=>void execute(intent).catch(()=>undefined)}>重试原操作</Button>:null}</Card>:null;
 return {execute,json,intent,notice,resolved,locked:!!intent||mutation.isPending,message,setMessage};
}
export type EcoCommands=ReturnType<typeof useEcoCommands>;
export function EcoPagination({page,total,onPage}:{page:number;total:string;onPage:(page:number)=>void}){return <div className="eco-pagination"><Button variant="outline" disabled={page===1} onClick={()=>onPage(page-1)}>上一页</Button><span>第 {page} 页 · 共 {total} 项</span><Button variant="outline" disabled={BigInt(page*20)>=BigInt(total)} onClick={()=>onPage(page+1)}>下一页</Button></div>}
export function ReadFailure({error,retry}:{error:unknown;retry:()=>void}){return <ConsoleState kind="error" title={errorText(error)}><Button variant="outline" onClick={retry}>重新读取</Button></ConsoleState>}
