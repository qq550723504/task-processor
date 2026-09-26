"use client";
import Link from "next/link";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Building2, ShieldCheck, ExternalLink, UserRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AccountReadError } from "@/lib/api/account";
import { verificationRequest, type VerificationInput } from "@/lib/api/subject-verification";
import styles from "./subject-verification.module.css";
import { PersonalVerification } from "./personal-verification";

export function SubjectVerification({userId, organizationId}: {userId:string;organizationId:string}) {
  const [tab,setTab]=useState("personal");
  return <section className={styles.workspace} aria-label="身份认证">
    <div className={styles.tabs}><span>认证类型</span><button type="button" aria-pressed={tab==="personal"} onClick={()=>setTab("personal")}><UserRound size={17}/>个人认证</button><button type="button" aria-pressed={tab==="enterprise"} onClick={()=>setTab("enterprise")}><Building2 size={17}/>企业认证</button></div>
    {tab==="enterprise"?<EnterpriseVerification key={`${userId}:${organizationId}`} userId={userId} organizationId={organizationId}/>:<PersonalVerification key={userId} userId={userId}/>}
  </section>;
}
function EnterpriseVerification({userId,organizationId}:{userId:string;organizationId:string}) {
  const client=useQueryClient();const queryKey=["subject-verification",userId,organizationId];
  const query=useQuery({queryKey,queryFn:({signal})=>verificationRequest(userId,organizationId,undefined,signal),retry:false,gcTime:0,staleTime:0});
  const [companyName,setCompany]=useState("");const [creditCode,setCode]=useState("");const [legalName,setLegal]=useState("");const [consent,setConsent]=useState(false);const [uncertain,setUncertain]=useState(false);
  const [idempotencyKey]=useState(()=>crypto.randomUUID());
  const mutation=useMutation({retry:false,mutationFn:(input:VerificationInput)=>verificationRequest(userId,organizationId,input),onSuccess:value=>{client.setQueryData(queryKey,value);setUncertain(false);},onError:error=>{if(error instanceof AccountReadError&&error.code==="RESULT_UNVERIFIED"){setUncertain(true);void query.refetch();}}});
  const data=query.data;const state=uncertain&&data?.state==="NOT_STARTED"?"OUTCOME_UNKNOWN":data?.state;
  const code=query.error instanceof AccountReadError?query.error.code:"";
  const denied=["PERMISSION_DENIED","ORGANIZATION_ACCESS_DENIED","ORGANIZATION_ACCESS_REVOKED","ORGANIZATION_SUSPENDED"].includes(code);
  return <>
    <div className={`${styles.card} ${styles.status}`}><div className={styles.statusTitle}><span className={styles.icon}><Building2 size={24}/></span><div><h2>企业认证</h2><p>{query.isError?"认证状态读取失败":query.isPending?"正在读取认证状态":state==="VERIFIED"?"企业认证已通过":state==="PENDING"?"等待完成认证":state==="OUTCOME_UNKNOWN"?"申请结果待核实，请勿重复提交":state==="EXPIRED"?"认证链接已过期":"尚未认证"}</p></div></div><ol className={styles.steps} aria-label="认证流程"><li>1 提交资料</li><li>2 腾讯电子签认证</li><li>3 认证完成</li></ol></div>
    <div className={styles.card}>
      {query.isError?<div role="alert"><h3>{denied?"需要当前企业管理员权限":code==="AUTHENTICATION_REQUIRED"?"请重新登录":"企业认证暂不可用"}</h3><p>{denied?"请联系当前企业管理员办理认证。":"本次未取得认证状态，请稍后刷新。"}</p></div>:query.isPending?<p role="status">正在读取认证资料…</p>:data?<>
        {state==="NOT_STARTED"?<form onSubmit={event=>{event.preventDefault();if(consent&&!mutation.isPending&&data.canStart)mutation.mutate({companyName,creditCode,legalName,consent:true,idempotencyKey});}}>
          <h3>企业基本信息</h3><p>请填写与营业执照一致的企业信息。</p>
          <div className={styles.fields}><label>企业名称<input required maxLength={85} value={companyName} onChange={e=>setCompany(e.target.value)} placeholder="请输入企业名称" disabled={mutation.isPending}/></label><label>统一社会信用代码<input required minLength={18} maxLength={18} pattern="[0-9A-Z]{18}" value={creditCode} onChange={e=>setCode(e.target.value.toUpperCase())} placeholder="请输入18位统一社会信用代码" disabled={mutation.isPending}/></label><label>法定代表人（选填）<input maxLength={40} value={legalName} onChange={e=>setLegal(e.target.value)} placeholder="请输入法定代表人姓名" disabled={mutation.isPending}/></label><label>经办手机号<input readOnly value={data.maskedPhone||"尚未验证中国大陆手机号"}/></label></div>
          {!data.canStart?<p className={styles.warning}>请先在<Link href="/workbench/account/profile/settings">账户设置</Link>绑定并验证本人的中国大陆手机号。</p>:null}
          <div className={styles.hosted}><ShieldCheck size={28}/><div><h3>在腾讯电子签完成资料提交与身份核验</h3><p>营业执照、证件及人脸信息由腾讯电子签采集。经办人需完成法人授权或授权书与对公打款验证。</p></div></div>
          <label className={styles.consent}><input type="checkbox" checked={consent} onChange={e=>setConsent(e.target.checked)} disabled={mutation.isPending}/>我确认有权代表该企业发起认证，同意将企业名称、信用代码、已验证手机号及所填法人姓名提交给腾讯电子签用于认证。</label>
          {mutation.isError?<p role="alert" className={styles.warning}>{mutation.error instanceof AccountReadError&&mutation.error.code==="VERIFIED_PHONE_REQUIRED"?"请先完成本人手机号验证。":mutation.error instanceof AccountReadError&&mutation.error.code==="VERIFICATION_CONFLICT"?"当前企业已有认证申请，请刷新认证状态。":"提交未完成，请刷新确认当前认证状态。"}</p>:null}
          <div className={styles.actions}><Button type="submit" disabled={!consent||!data.canStart||mutation.isPending}>{mutation.isPending?"正在提交…":"前往腾讯电子签认证"}</Button></div>
        </form>:<div className={styles.result}>
          {data.companyName?<><h3>{data.companyName}</h3><p>统一社会信用代码：{data.creditCode}</p><p>经办手机号：{data.maskedPhone}</p></>:null}
          {state==="PENDING"?<><p>请由发起申请的经办人在腾讯电子签完成认证，返回本页后刷新状态。</p>{data.verificationUrl?<Button asChild><a href={data.verificationUrl} target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">继续认证<ExternalLink size={16}/></a></Button>:<p>认证链接仅向本次申请的经办人提供。</p>}</>:null}
          {state==="OUTCOME_UNKNOWN"?<p>尚未确认是否已生成认证链接。本页不会重新发起申请；如已完成腾讯认证，请刷新查看结果。持续未更新时请联系管理员核实。</p>:null}
          {state==="EXPIRED"?<p>本次链接已失效，暂不支持重新申请。原认证完成后，仍可刷新查看结果。</p>:null}
          {state==="VERIFIED"?<p>认证时间：{data.verifiedAt?new Date(data.verifiedAt).toLocaleString("zh-CN"):"—"}</p>:null}
        </div>}
      </>:null}
      <footer className={styles.footer}><p>认证不会增加平台权限。本次仅支持首次企业认证，不支持重新申请或更换经办人。</p><Button type="button" variant="outline" disabled={query.isFetching||mutation.isPending} onClick={()=>void query.refetch()}>刷新认证状态</Button></footer>
    </div>
  </>;
}
