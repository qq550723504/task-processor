"use client";
import { useEffect, useId, useRef, useState } from "react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { basesSchema, sourcesSchema, knowledgeRequest, KnowledgeError, type KnowledgeSource, type KnowledgeScope } from "@/lib/api/knowledge";
import styles from "../task-center/title-review.module.css";

export function TitleAgentConfirmation({scope,organization,platform,knowledgeAvailable,onConfirm,onCancel}:{scope:KnowledgeScope;organization:string;platform:string;knowledgeAvailable:boolean;onConfirm:(baseId?:string)=>void;onCancel:()=>void}) {
 const dialog=useRef<HTMLDialogElement>(null),cancelButton=useRef<HTMLButtonElement>(null),request=useRef<AbortController|null>(null);
 const id=useId();
 const [useKnowledge,setUseKnowledge]=useState(false),[bases,setBases]=useState<z.infer<typeof basesSchema>["items"]>([]),[baseId,setBaseId]=useState(""),[sources,setSources]=useState<KnowledgeSource[]>([]),[loading,setLoading]=useState(false),[failure,setFailure]=useState("");
 useEffect(()=>{const previous=document.activeElement,element=dialog.current;element?.showModal();cancelButton.current?.focus();return()=>{request.current?.abort();element?.close();if(previous instanceof HTMLElement&&previous.isConnected)previous.focus()};},[]);
 async function load(selection?:string) {
  request.current?.abort();const controller=new AbortController();request.current=controller;setLoading(true);setFailure("");
  try {
   if(selection) {const result=await knowledgeRequest(scope,`knowledge-bases/${selection}/sources`,sourcesSchema,{signal:controller.signal});if(!controller.signal.aborted)setSources(result.items);}
   else {const result=await knowledgeRequest(scope,"knowledge-bases?page=1&pageSize=100",basesSchema,{signal:controller.signal});if(!controller.signal.aborted)setBases(result.items.filter(base=>base.state==="ACTIVE"));}
  }catch(error){if(!controller.signal.aborted)setFailure(error instanceof KnowledgeError&&error.status===403?"当前身份无权读取企业知识。":"企业知识当前不可用，请稍后重试或明确选择不使用知识。");}
  finally{if(!controller.signal.aborted)setLoading(false);}
 }
 const activeSources=sources.filter(source=>source.state==="ACTIVE");
 const ready=!!baseId&&!loading&&!failure&&activeSources.length>0&&activeSources.length<=4&&activeSources.every(source=>source.currentReadableRevision);
 return <dialog ref={dialog} className={styles.confirmation} aria-labelledby={`${id}-title`} aria-describedby={`${id}-description`} onCancel={event=>{event.preventDefault();onCancel()}}>
  <h2 id={`${id}-title`}>确认标题优化</h2><p>当前企业 · {organization}</p>
  <p id={`${id}-description`}>仅生成标题建议，不自动修改商品。建议仍需 Human Review；商品事实以当前已保存版本为准。</p>
  <p>素材查询平台：{platform}。发送前重新检查企业权限、模型能力和成员额度；完整输入参与既有预算与计费，当前未报价。</p>
  {knowledgeAvailable?<label className="mt-4 flex items-center gap-2"><input type="checkbox" checked={useKnowledge} onChange={event=>{setUseKnowledge(event.target.checked);setBaseId("");setSources([]);setFailure("");if(event.target.checked)void load();else{request.current?.abort();setLoading(false)}}}/>使用企业知识（可选）</label>:<p>当前环境未开放企业知识；本次仅使用商品证据。</p>}
  {useKnowledge&&<div className="mt-4 rounded-lg border border-border bg-secondary p-4">
   <label htmlFor={`${id}-base`}>企业知识库</label><select id={`${id}-base`} className="mt-2 block w-full rounded-lg border border-border bg-background p-2" value={baseId} disabled={loading} onChange={event=>{setBaseId(event.target.value);setSources([]);if(event.target.value)void load(event.target.value)}}><option value="">请选择知识库</option>{bases.map(base=><option key={base.id} value={base.id}>{base.name}</option>)}</select>
   {loading?<p role="status">正在读取知识与版本…</p>:failure?<p role="alert">{failure}</p>:baseId?<><ul className="mt-3 space-y-2">{activeSources.map(source=><li key={source.id}>{source.name} · {source.currentReadableRevision?`v${source.currentReadableRevision.number} · ${source.currentReadableRevision.state==="PARTIAL"?"部分解析可用":"可读"}`:"尚无可读版本"}</li>)}</ul>{!ready&&<p>所选知识库当前未就绪，不能发送。请选择其他知识库，或取消使用企业知识。</p>}</>:!bases.length?<p>没有当前可用的知识库。</p>:null}
   <p>确认后服务端冻结实际采用的版本；若完整内容超限，将明确拒绝，不会截断或切换知识。</p>
  </div>}
  <div className={styles.actions}><Button ref={cancelButton} variant="outline" onClick={onCancel}>取消</Button><Button disabled={useKnowledge&&!ready} onClick={()=>onConfirm(useKnowledge?baseId:undefined)}>确认生成</Button></div>
 </dialog>;
}
