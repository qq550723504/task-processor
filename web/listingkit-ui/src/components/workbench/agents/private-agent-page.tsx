"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { CustomizationError, type CustomScope } from "@/lib/api/agent-customization";
import { privateAgentRequest, privateDelivery, privatePage, qualityPage, qualityRun, type QualityRun } from "@/lib/api/private-agent";
import { PrivateDraftForm } from "./private-draft-form";
import styles from "./agents.module.css";
import qualityStyles from "./private-agent.module.css";
const limitation="基于所选平台草稿保存时的确定性规则，不调用模型。报告不代表最新平台规则、当前图片可访问性或上传批准；上传流程仍会重新核对。";
const href=(id:string)=>"/workbench/agents/mine/private/"+id;
function errorText(e:unknown){const code=e instanceof CustomizationError?e.code:"";return ({CUSTOMIZATION_NOT_FOUND:"当前企业或成员无权读取这项交付或报告。",CUSTOMIZATION_FORBIDDEN:"当前身份无权读取所选商品资料。",PERMISSION_DENIED:"当前身份没有操作权限。",CUSTOMIZATION_REVISION_MISMATCH:"草稿版本或阶段已变化，请重新选择。",CUSTOMIZATION_CONFLICT:"原检查与请求编号不一致，或此交付仅供历史读取。",OUTCOME_UNKNOWN:"结果尚未确认，请核实同一次检查。"} as Record<string,string>)[code]??"暂时无法读取，请确认供应链功能和当前来源权限后重试。"}
function usePrivateRead<T>(scope:CustomScope,path:string,schema:z.ZodType<T>,nonce=0,enabled=true){
  const [state,setState]=useState<{key:string;data?:T;error?:unknown}>({key:""});
  const {userId,organizationId}=scope,key=path+":"+nonce;
  useEffect(()=>{if(!enabled)return;const c=new AbortController(),timer=setTimeout(()=>c.abort(),45000);let alive=true;
    void privateAgentRequest({userId,organizationId},path,schema,{signal:c.signal}).then(data=>{if(alive)setState({key,data})}).catch(error=>{if(alive)setState({key,error})});
    return()=>{alive=false;clearTimeout(timer);c.abort()};
  },[userId,organizationId,path,schema,key,enabled]);
  return enabled&&state.key===key?state:{key};
}
export function PrivateAgentCards({scope}:{scope:CustomScope}){
  const [cursor,setCursor]=useState(""),[nonce,setNonce]=useState(0),ctx=useWorkbenchContext();
  const result=usePrivateRead(scope,cursor?"?cursor="+cursor:"",privatePage,nonce);
  if(!ctx.permissions.includes("workbench.agent.read"))return null;
  return <section className={qualityStyles.section} aria-label="企业私有智能体"><h2>企业私有智能体</h2>
    {result.error?<ConsoleState kind="unavailable" title="暂时无法读取私有智能体"><p>{errorText(result.error)}</p><Button onClick={()=>setNonce(v=>v+1)}>重新读取私有智能体</Button></ConsoleState>:!result.data?<ConsoleState kind="loading" title="正在读取私有智能体"/>:!result.data.items.length?<ConsoleState kind="empty" title="当前企业暂无已交付的私有智能体"><Link href="/workbench/agents/custom">了解智能体定制 →</Link></ConsoleState>:<div className={styles.mineCards}>{result.data.items.map(v=><Card key={v.id} className={styles.agentCard}><div className={styles.cardActions}><h2>{v.name}</h2><span className={styles.status}>企业专属{v.version==="1.0.0"?" · 历史只读":""}</span></div><p>{v.version==="2.0.0"?"选择上传前的平台草稿，保存绑定草稿版本的资料质检报告。":"原手工输入试用已结束，可以查看已保存的历史报告。"}</p><p>交付版本 {v.version}</p><div className={styles.cardActions}><span className={styles.support}>无外部写入 · 不调用模型</span><Button asChild><Link href={href(v.id)}>{v.version==="2.0.0"?"进入使用":"查看历史"}</Link></Button></div></Card>)}</div>}
    {result.data?.nextCursor?<Button variant="outline" onClick={()=>setCursor(result.data!.nextCursor)}>更多私有智能体</Button>:null}{cursor?<Button variant="outline" onClick={()=>setCursor("")}>返回首页</Button>:null}
  </section>;
}
export function PrivateAgentPage({id}:{id:string}){
  const c=useWorkbenchContext();
  if(c.isLoading||c.isSwitching)return <ConsoleState kind="loading" title="正在确认当前企业"/>;
  if(c.error||c.blockingError||!c.user||!c.effectiveOrganization)return <ConsoleState kind="unavailable" title="请先确认登录身份与当前企业"/>;
  if(!c.permissions.includes("workbench.agent.read"))return <ConsoleState kind="unavailable" title="当前身份没有智能体读取权限"/>;
  const scope={userId:c.user.id,organizationId:c.effectiveOrganization.id};
  return <ScopedPrivate key={[scope.userId,scope.organizationId,id,c.permissions.join("|")].join(":")} scope={scope} id={id} organization={c.effectiveOrganization.name} canUse={c.permissions.includes("workbench.agent.use")}/>;
}
function ScopedPrivate({scope,id,organization,canUse}:{scope:CustomScope;id:string;organization:string;canUse:boolean}){
  const [nonce,setNonce]=useState(0),[cursor,setCursor]=useState(""),[selectedID,setSelectedID]=useState(""),[localRun,setLocalRun]=useState<QualityRun>();
  const context=useWorkbenchContext(),sourcesReadable=["workbench.collection.read","workbench.supply.read"].every(p=>context.permissions.includes(p));
  const delivery=usePrivateRead(scope,"/"+id,privateDelivery,nonce);
  const readable=sourcesReadable||delivery.data?.version==="1.0.0";
  const reports=usePrivateRead(scope,"/"+id+"/reports"+(cursor?"?cursor="+cursor:""),qualityPage,nonce,readable);
  const detail=usePrivateRead(scope,`/${id}/reports/${selectedID}`,qualityRun,nonce,!!selectedID&&readable);
  const selected=!readable||detail.error?undefined:detail.data??(localRun?.id===selectedID?localRun:undefined);
  return <ConsolePage title="平台草稿资料质检" description={`企业私有智能体 · 当前企业：${organization}`} breadcrumbs={[{label:"智能市场"},{label:"我的智能体",href:"/workbench/agents/mine"},{label:"平台草稿资料质检"}]} actions={<Link href="/workbench/agents/mine">返回我的智能体 →</Link>}>
    {delivery.error?<ConsoleState kind="unavailable" title="此智能体在当前企业不可用"><p>{errorText(delivery.error)}</p><Button onClick={()=>setNonce(v=>v+1)}>重新读取</Button></ConsoleState>:!delivery.data?<ConsoleState kind="loading" title="正在读取私有交付"/>:<>
      <Card className={styles.detailHead}><div><h2>{delivery.data.name}</h2><p>{delivery.data.version==="2.0.0"?limitation:"此版本为原手工输入试用的只读历史，不能执行新的检查。"}</p><p>交付版本 {delivery.data.version} · {new Date(delivery.data.createdAt).toLocaleString("zh-CN")}</p></div></Card>
      {delivery.data.version==="2.0.0"?<div className={qualityStyles.grid}><PrivateDraftForm scope={scope} id={id} canUse={canUse} saved={run=>{setSelectedID(run.id);setLocalRun(run);setCursor("");setNonce(v=>v+1)}}/><Card className={styles.panel}><h2>检查范围</h2><p>待补全或已适配的 SHEIN 美国站平台草稿。</p><p>复用草稿保存时的必填项、规格、图片及平台资料校验；问题逐项保留。</p><p>报告仅当前操作人可读，读取时仍需原商品资料权限。</p><p>修改并保存草稿后，请选择新版本检查。未发现问题不等于允许上传。</p></Card></div>:null}
      {detail.error?<p role="alert">{errorText(detail.error)}</p>:selected?<Report run={selected}/>:selectedID?<p role="status">正在读取报告…</p>:null}
      <Card className={styles.panel}><h2>我的已保存报告</h2>{!readable?<p role="alert">需要当前商品资料和供应链读取权限，才能查看草稿报告。</p>:reports.error?<p role="alert">{errorText(reports.error)}</p>:!reports.data?<p>正在读取报告…</p>:!reports.data.items.length?<p>尚未保存质检报告。</p>:<ul className={qualityStyles.history}>{reports.data.items.map(v=><li key={v.id}><div><strong>{v.title||"未提供名称"}</strong><p>{new Date(v.createdAt).toLocaleString("zh-CN")} · {v.findingCount} 项提示{v.version==="2.0.0"?` · 草稿 v${v.draft.revision}`:" · 手工输入历史"}</p></div><Button variant="outline" onClick={()=>{setLocalRun(undefined);setSelectedID(v.id)}}>查看报告</Button></li>)}</ul>}{readable?<div className={styles.actions}><Button variant="outline" onClick={()=>setNonce(v=>v+1)}>刷新报告</Button>{reports.data?.nextCursor?<Button variant="outline" onClick={()=>setCursor(reports.data!.nextCursor)}>更多报告</Button>:null}{cursor?<Button variant="outline" onClick={()=>setCursor("")}>报告首页</Button>:null}</div>:null}</Card>
    </>}
  </ConsolePage>;
}
function Report({run}:{run:QualityRun}){
  if(run.version==="1.0.0")return <Card className={`${styles.panel} ${qualityStyles.report}`} aria-label="历史质检报告"><h2>历史手工报告 · {run.input.name||"未提供名称"}</h2><p>{run.report.summary}</p><p>报告编号 {run.id} · {new Date(run.createdAt).toLocaleString("zh-CN")}</p><p>材质：{run.input.material||"未提供"}；尺寸：{run.input.dimensions||"未提供"}</p><p>{run.input.description}</p><ul>{run.input.specifications.map((v,i)=><li key={i}>{v.name}：{v.value}</li>)}</ul><ol>{run.report.findings.map((v,i)=><li key={i}><strong>{v.message}</strong><p>{v.suggestion}</p></li>)}</ol><p>这是原1.0.0试用的不可变记录，不是平台草稿检查结果。</p></Card>;
  const d=run.draft;
  return <Card className={`${styles.panel} ${qualityStyles.report}`} aria-label="质检报告"><h2>草稿质检报告 · {d.title||d.productKey}</h2><p>{d.issues.length?`${d.issues.length} 项资料问题，请在我的供应链补全并保存新草稿。`:"保存时规则未发现资料问题，上传前仍须重新核对。"}</p><dl><dt>平台 / 站点</dt><dd>SHEIN / 美国站</dd><dt>草稿版本</dt><dd>v{d.revision} · {d.recordId}</dd><dt>草稿保存时间</dt><dd>{new Date(d.savedAt).toLocaleString("zh-CN")}</dd><dt>检查时间</dt><dd>{new Date(run.createdAt).toLocaleString("zh-CN")}</dd><dt>商品版本</dt><dd>{d.productVersion}</dd><dt>报告编号</dt><dd>{run.id}</dd></dl><ol>{d.issues.map((v,i)=><li key={i}><strong>{v.message}</strong><p>{v.field} · {v.code}</p></li>)}</ol><p>{limitation}</p><Button asChild variant="outline"><Link href={`/workbench/supply/mine?preparation=${d.preparationId}&store=${d.storeId}`}>查看当前草稿并补全</Link></Button></Card>;
}
