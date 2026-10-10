"use client";
import { useState } from "react";
import Image from "next/image";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Select } from "@/components/ui/select";
import { ResourceDialog } from "../resources/resource-dialog";
import { carouselTasks, detailTasks, imageSetTemplateSchema, type ImageSetTemplate } from "@/lib/contracts/image-set-configuration";
import styles from "./image-configuration.module.css";

export function ImageConfigurationDialog({ initial, onSave, onClose, locked=false }: {initial:ImageSetTemplate;onSave:(image:ImageSetTemplate)=>void;onClose:()=>void;locked?:boolean}) {
  const [draft,setDraft]=useState(()=>structuredClone(initial));
  const [alternate,setAlternate]=useState<ImageSetTemplate>(()=>({...structuredClone(initial),mode:initial.mode==="standard"?"custom":"standard",carousel:[],detail:[]}));
  const [group,setGroup]=useState<"carousel"|"detail">("carousel");
  const [error,setError]=useState("");
  const library=group==="carousel"?carouselTasks:detailTasks;
  const count=draft.carousel.length+draft.detail.length;
  function mode(next:ImageSetTemplate["mode"]){
    if(next===draft.mode)return;
    setAlternate(draft);setDraft({...alternate,shareOriginals:draft.shareOriginals,background:draft.background,language:draft.language});setError("");
  }
  function toggle(purpose:string){
    const selected=draft[group];
    const tasks=selected.some(task=>task.purpose===purpose)?selected.filter(task=>task.purpose!==purpose):[...selected,{id:`${group}-${purpose}`,purpose}];
    tasks.sort((a,b)=>library.findIndex(item=>item.purpose===a.purpose)-library.findIndex(item=>item.purpose===b.purpose));
    setDraft({...draft,[group]:tasks});setError("");
  }
  function confirm(){
    if(locked)return;
    const parsed=imageSetTemplateSchema.safeParse(draft);
    if(!parsed.success){setError(parsed.error.issues[0]?.message??"请检查图片配置。");return}
    onSave(parsed.data);
  }
  const choice=(title:string,description:string,selected:boolean,onClick:()=>void)=><Button variant="outline" className={styles.choice} aria-label={`${title} ${description}`} aria-pressed={selected} disabled={locked} onClick={onClick}><strong>{title}</strong><span>{description}</span></Button>;
  return <ResourceDialog title="AI重构图片设置" onClose={onClose} locked={locked} className={styles.dialog}>
    <div className={styles.subheading}><p>先选择生成规则，再配置主图或详情图。</p><div className={styles.settings}>
      <label>语言<Select aria-label="图片文字语言" value={draft.language} disabled={locked} onChange={event=>setDraft({...draft,language:event.target.value as ImageSetTemplate["language"]})}>{[["en","英语"],["zh","中文"],["es","西班牙语"],["fr","法语"],["de","德语"],["ja","日语"]].map(([value,label])=><option key={value} value={value}>{label}</option>)}</Select></label>
      <label>背景<input aria-label="图片背景" value={draft.background} disabled={locked} onChange={event=>setDraft({...draft,background:event.target.value})}/></label>
    </div></div>
    <div className={styles.steps}>
      <div className={styles.stepLabel}><span>1</span><div><strong>生成方式</strong><small>选择内容模板</small></div></div>
      <div className={styles.options}>{choice("标准方案","可选 8+8 内容任务",draft.mode==="standard",()=>mode("standard"))}{choice("自定义指令","整套最多 32 个任务",draft.mode==="custom",()=>mode("custom"))}</div>
      <div className={styles.stepLabel}><span>2</span><div><strong>图片策略</strong><small>选择原始素材</small></div></div>
      <div className={styles.options}>{choice("共用原始素材","两组仍独立生成、分别计费",draft.shareOriginals,()=>setDraft({...draft,shareOriginals:true}))}{choice("分别选择素材","主图、详情图各选原图",!draft.shareOriginals,()=>setDraft({...draft,shareOriginals:false}))}</div>
      <div className={styles.stepLabel}><span>3</span><div><strong>配置对象</strong><small>选择主图或详情图</small></div></div>
      <div className={styles.options}>{choice("主图","配置主图生成任务",group==="carousel",()=>setGroup("carousel"))}{choice("详情图","配置详情图生成任务",group==="detail",()=>setGroup("detail"))}</div>
      <div className={styles.stepLabel}><span className={styles.statusDisc}><Image src="/images/image-agent/status-disc.svg" width={24} height={24} alt=""/><b>✓</b></span><div><strong>当前状态</strong><small>两组独立生成</small></div></div>
      <div className={styles.summary}><strong>{draft.shareOriginals?"共用原始素材，两组分别生成":"主图、详情图分别选择素材"}</strong><small>主图 {draft.carousel.length} 张 · 详情图 {draft.detail.length} 张 · 内容模板不替代平台图片规则</small></div>
    </div>
    <fieldset disabled={locked} className={styles.tasks}>
      <legend>{draft.mode==="standard"?"标准":"自定义"}{group==="carousel"?"主图":"详情图"}任务 · 当前选择 {draft[group].length} 张</legend>
      {draft.mode==="standard"?<div className={styles.taskGrid}>{library.map((definition,index)=>{
        const selected=draft[group].some(task=>task.purpose===definition.purpose);
        return <label key={definition.purpose} className={styles.task} data-selected={selected}><Checkbox checked={selected} onChange={()=>toggle(definition.purpose)}/><span className={styles.index}>{String(index+1).padStart(2,"0")}</span><span><strong>{definition.label}</strong><small>{definition.description}</small></span></label>;
      })}</div>:<div className={styles.customTasks}>{draft[group].map((task,index)=><div className={styles.customTask} key={task.id}><label>{group==="carousel"?"主图":"详情图"}任务 {index+1} 指令<textarea aria-label={`${group==="carousel"?"主图":"详情图"}任务 ${index+1} 指令`} value={task.brief??""} onChange={event=>setDraft({...draft,[group]:draft[group].map(item=>item.id===task.id?{...item,brief:event.target.value}:item)})}/></label><Button variant="ghost" onClick={()=>setDraft({...draft,[group]:draft[group].filter(item=>item.id!==task.id)})} aria-label={`删除任务 ${index+1}`}>删除</Button></div>)}<Button variant="outline" disabled={count>=32} onClick={()=>setDraft({...draft,[group]:[...draft[group],{id:`custom-${crypto.randomUUID()}`,purpose:"custom",brief:""}]})}>新增{group==="carousel"?"主图":"详情图"}任务</Button></div>}
    </fieldset>
    <div className={styles.billing}>计费说明：按实际生成任务分别扣点；生成前确认具体张数与总点数。规格、步骤和配件任务需要真实依据。</div>
    {error&&<p className={styles.error} role="alert">{error}</p>}
    <div className={styles.footer}><p>完成配置后保存，生成前仍需确认计划。</p><Button variant="outline" disabled={locked} onClick={onClose}>取消</Button><Button className={styles.confirm} disabled={locked} onClick={confirm}>确认选择</Button></div>
  </ResourceDialog>;
}
