"use client";
import Link from "next/link";
import { useRef,useState } from "react";
import { useMutation,useQuery,useQueryClient } from "@tanstack/react-query";
import { ExternalLink,ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AccountReadError } from "@/lib/api/account";
import { personalVerificationRequest,type PersonalInput } from "@/lib/api/personal-verification";
import styles from "./subject-verification.module.css";

type MetaWindow=Window&{getMetaInfo?:()=>unknown};
let metaLoading:Promise<void>|undefined;
async function deviceMeta():Promise<string>{
 if(!(window as MetaWindow).getMetaInfo){
  if(!metaLoading)metaLoading=new Promise<void>((resolve,reject)=>{
   const script=document.createElement("script");script.src="https://o.alicdn.com/yd-cloudauth/cloudauth-cdn/jsvm_all.js";script.async=true;script.referrerPolicy="no-referrer";
   const finish=(error?:Error)=>{clearTimeout(timer);script.onload=null;script.onerror=null;if(error){script.remove();metaLoading=undefined;reject(error);}else resolve();};
   const timer=setTimeout(()=>finish(new Error("认证组件加载超时，请稍后再试。")),8000);
   script.onload=()=>finish();script.onerror=()=>finish(new Error("认证组件加载失败，请检查网络后重试。"));document.head.appendChild(script);
  });await metaLoading;
 }
 const value=(window as MetaWindow).getMetaInfo?.();if(!value)throw new Error("当前浏览器无法启动认证，请更换浏览器。");
 return typeof value==="string"?value:JSON.stringify(value);
}
const stateText:Record<string,string>={NOT_STARTED:"尚未认证",PENDING:"等待完成刷脸认证",OUTCOME_UNKNOWN:"申请结果待核实，请勿重复提交",EXPIRED:"本次认证链接已过期",REJECTED:"本次认证未通过",VERIFIED:"个人认证已通过"};
const errorText:Record<string,string>={VERIFICATION_TOTAL_LIMIT:"累计认证次数已用完，不能再发起新申请。",VERIFICATION_DAILY_LIMIT:"今日认证次数已用完，请明日再试。",VERIFICATION_COOLDOWN:"距离上次发起不足规定间隔，请稍后刷新。",VERIFICATION_REFRESH_BUSY:"认证结果正在查询，请稍后刷新。",VERIFIED_PHONE_REQUIRED:"请先绑定并验证本人的中国大陆手机号。",VERIFICATION_CONFLICT:"申请状态已变化，请刷新后确认。",RESULT_UNVERIFIED:"提交结果尚未确认，请先刷新状态，本页不会自动重新提交。"};
export function PersonalVerification({userId}:{userId:string}){
 const client=useQueryClient(),queryKey=["personal-verification",userId];
 const query=useQuery({queryKey,queryFn:({signal})=>personalVerificationRequest(userId,"read",undefined,signal),retry:false,gcTime:0,staleTime:0});
 const [name,setName]=useState(""),[idNumber,setNumber]=useState(""),[consent,setConsent]=useState(false),[uncertain,setUncertain]=useState(false);
 const key=useRef("");
 const start=useMutation({retry:false,mutationFn:async()=>{
  if(!key.current)key.current=crypto.randomUUID();
  try{const metaInfo=await deviceMeta();const input:PersonalInput={name,idNumber,metaInfo,consent:true,idempotencyKey:key.current};return await personalVerificationRequest(userId,"start",input);}
  finally{setName("");setNumber("");setConsent(false);}
 },onSuccess:value=>{client.setQueryData(queryKey,value);key.current="";setUncertain(false);},onError:error=>{if(error instanceof AccountReadError&&error.code==="RESULT_UNVERIFIED")setUncertain(true);void query.refetch();}});
 const refresh=useMutation({retry:false,mutationFn:async()=>{
  const data=query.data;return data?.canRefresh&&data.applicationId?personalVerificationRequest(userId,"refresh",{applicationId:data.applicationId}):personalVerificationRequest(userId);
 },onSuccess:value=>{client.setQueryData(queryKey,value);key.current="";start.reset();setUncertain(false);}});
 const data=query.data,busy=start.isPending||refresh.isPending||query.isFetching;
 const error=start.error??refresh.error;
 const state=uncertain?"OUTCOME_UNKNOWN":data?.state;
 return <>
  <div className={`${styles.card} ${styles.status}`}><div><h2>个人认证</h2><p>{query.isError?"个人认证暂不可用":query.isPending?"正在读取认证状态":stateText[state??"NOT_STARTED"]}</p></div><ol className={styles.steps} aria-label="个人认证流程"><li>1 核对身份与手机号</li><li>2 阿里云刷脸</li><li>3 查询认证结果</li></ol></div>
  <div className={styles.card}>
   {query.isError?<p role="alert">暂时无法取得认证状态，请稍后刷新。</p>:data?<>
    <div aria-label="认证次数"><p>今日剩余 {data.quota.dailyRemaining} / {data.quota.dailyLimit} 次（北京时间）</p><p>累计剩余 {data.quota.totalRemaining} / {data.quota.totalLimit} 次</p><p>失败和超时也占次数；继续已有认证或查询结果不另扣次数。</p></div>
    {state!=="VERIFIED"&&data.quota.totalRemaining===0?<p className={styles.warning}>累计认证次数已用完，不能再发起新申请；仍可继续或查询已有申请。</p>:data.quota.dailyRemaining===0?<p className={styles.warning}>今日次数已用完，将于北京时间 {new Date(data.quota.resetAt).toLocaleString("zh-CN",{timeZone:"Asia/Shanghai"})} 恢复每日额度。</p>:null}
    {data.quota.nextAllowedAt>data.quota.serverTime?<p>下次可发起时间：北京时间 {new Date(data.quota.nextAllowedAt).toLocaleString("zh-CN",{timeZone:"Asia/Shanghai"})}，到时请刷新。</p>:null}
    {!uncertain&&["NOT_STARTED","REJECTED","EXPIRED"].includes(state??"")&&data.quota.totalRemaining>0?<form onSubmit={e=>{e.preventDefault();if(consent&&data.canStart&&!busy)start.mutate();}}>
     <div className={styles.fields}><label>真实姓名<input required autoComplete="off" maxLength={40} value={name} disabled={busy} onChange={e=>setName(e.target.value)}/></label><label>身份证号码<input required autoComplete="off" minLength={18} maxLength={18} pattern="[0-9]{17}[0-9X]" value={idNumber} disabled={busy} onChange={e=>setNumber(e.target.value.toUpperCase())}/></label><label>已验证手机号<input readOnly value={data.maskedPhone||"尚未验证中国大陆手机号"}/></label></div>
     {!data.phoneReady?<p className={styles.warning}>请先在<Link href="/workbench/account/profile/settings">账户设置</Link>绑定并验证本人的中国大陆手机号。</p>:null}
     <div className={styles.hosted}><ShieldCheck size={28}/><div><h3>本人手机号核验后，在阿里云完成刷脸</h3><p>手机号必须登记在该姓名和身份证名下。硕米不保存完整姓名、身份证号或人脸照片；阿里云负责身份核验和人脸采集。</p></div></div>
     <label className={styles.consent}><input type="checkbox" checked={consent} disabled={busy} onChange={e=>setConsent(e.target.checked)}/>我确认使用本人身份，同意将姓名、身份证号、已验证手机号和认证所需设备信息提交阿里云用于本次认证。</label>
     <div className={styles.actions}><Button type="submit" disabled={!consent||!data.canStart||busy}>{start.isPending?"正在发起认证…":"开始个人认证"}</Button></div>
    </form>:null}
    {state==="PENDING"?<div className={styles.result}><p>请完成阿里云刷脸，返回本页点击“刷新认证结果”。</p>{data.verificationUrl?<Button asChild><a href={data.verificationUrl} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">继续个人认证<ExternalLink size={16}/></a></Button>:<p>当前手机号与申请不一致，无法继续本次认证。</p>}</div>:null}
    {state==="OUTCOME_UNKNOWN"?<p>本次结果尚未确认，次数已占用；不会自动重发。请先刷新状态，申请到期后可在剩余额度内重新发起。</p>:null}
    {state==="EXPIRED"&&data.canRefresh?<p>若已完成刷脸，请先刷新查询原申请结果，再决定是否重新发起。</p>:null}
    {state==="VERIFIED"?<p>认证时间：{new Date(data.verifiedAt!).toLocaleString("zh-CN")}。本次认证不会自动增加平台权限。</p>:null}
   </>:<p role="status">正在读取认证资料…</p>}
   {error?<p role="alert" className={styles.warning}>{error instanceof AccountReadError?(errorText[error.code]??"操作未完成，请刷新状态后确认。"):"认证组件未能启动，请检查网络或更换浏览器后重试。"}</p>:null}
   <footer className={styles.footer}><p>仅支持首次个人认证。实名通过后暂不支持更换身份。</p><Button type="button" variant="outline" disabled={busy} onClick={()=>refresh.mutate()}>刷新认证结果</Button></footer>
  </div>
 </>;
}
