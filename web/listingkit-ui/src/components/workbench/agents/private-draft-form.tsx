"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { useResourcePending } from "../resources/resource-pending";
import { CustomizationError, type CustomScope } from "@/lib/api/agent-customization";
import { privateAgentRequest, qualityPending, qualityRun, type QualityRun } from "@/lib/api/private-agent";
import { listSupplyPreparations, listSupplyStages, readSupplyTarget } from "@/lib/api/supply-chain";
import type { SupplyPreparation, SupplyTarget } from "@/lib/contracts/supply-chain";
import { listWorkbenchStores, type WorkbenchStore } from "@/lib/api/workbench-stores";
import styles from "./agents.module.css";
import qualityStyles from "./private-agent.module.css";

export function PrivateDraftForm({scope,id,canUse,saved,offlineTrial=false}:{scope:CustomScope;id:string;canUse:boolean;offlineTrial?:boolean;saved:(run:QualityRun)=>void}) {
  const context=useWorkbenchContext();
  const {userId,organizationId}=scope;
  const canSelect=["workbench.collection.read","workbench.supply.read","workbench.store.read"].every(p=>context.permissions.includes(p));
  const pending=useResourcePending({expectedUserId:scope.userId,expectedOrganizationId:scope.organizationId},["private-draft-quality",id],qualityPending,{storage:"session",maxLength:4096});
  const [preparations,setPreparations]=useState<SupplyPreparation[]>([]),[prepAfter,setPrepAfter]=useState<string>(),[prepNext,setPrepNext]=useState<string>();
  const [stores,setStores]=useState<WorkbenchStore[]>([]),[storePage,setStorePage]=useState(1),[storeTotal,setStoreTotal]=useState(0);
  const [prep,setPrep]=useState(""),[store,setStore]=useState(""),[stage,setStage]=useState<"missing"|"ready">("missing"),[after,setAfter]=useState<string>(),[reload,setReload]=useState(0);
  const [choices,setChoices]=useState<{key:string;items:SupplyTarget[];next?:string;error?:string}>({key:"",items:[]}),[selected,setSelected]=useState<{key:string;record:SupplyTarget}>();
  const [busy,setBusy]=useState(false),[failure,setFailure]=useState(""),[message,setMessage]=useState("");
  const live=useRef(true),flight=useRef(false),abort=useRef<AbortController|null>(null);
  const key=JSON.stringify([prep,store,stage,after,reload]),rows=choices.key===key?choices.items:[],record=selected?.key===key?selected.record:undefined;
  const loading=!!prep&&!!store&&choices.key!==key;
  const locked=busy||!!pending.command||!pending.ready;
  useEffect(()=>{live.current=true;return()=>{live.current=false;abort.current?.abort()}},[]);
  const guard=context.registerOrganizationSwitchGuard;
  useEffect(()=>guard?.(()=>!locked&&!flight.current),[guard,locked]);
  useEffect(()=>{
    if(!canSelect)return;let active=true;const c=new AbortController();
    void Promise.all([listSupplyPreparations({userId,organizationId},{after:prepAfter,limit:20},c.signal),listWorkbenchStores({page:storePage,pageSize:20,platform:"shein",status:"active"},organizationId,c.signal)])
      .then(([b,s])=>{if(active){setPreparations(b.items);setPrepNext(b.nextCursor);setStores(s.items);setStoreTotal(s.pagination.total);setFailure("")}})
      .catch(()=>{if(active)setFailure("无法读取草稿选择范围，请确认供应链功能、店铺和当前权限已就绪。")});
    return()=>{active=false;c.abort()};
  },[userId,organizationId,canSelect,prepAfter,storePage,reload]);
  useEffect(()=>{
    if(!canSelect||!prep||!store)return;let active=true;const c=new AbortController();
    void listSupplyStages({userId,organizationId},prep,store,stage,{after,limit:20},c.signal).then(async page=>({items:await Promise.all(page.items.map(v=>readSupplyTarget({userId,organizationId},v.sourceId,store,c.signal))),next:page.nextCursor}))
      .then(result=>{if(active)setChoices({key,...result})})
      .catch(()=>{if(active)setChoices({key,items:[],error:"无法读取所选草稿，请重新确认店铺、企业和来源访问权限。"})});
    return()=>{active=false;c.abort()};
  },[userId,organizationId,canSelect,prep,store,stage,after,reload,key]);
  async function execute(command:z.infer<typeof qualityPending>,verification=false){
    if(flight.current||!pending.ready||!canUse||!canSelect||command.deliveryId!==id)return;
    flight.current=true;setBusy(true);setMessage("");const c=new AbortController();abort.current=c;const timer=setTimeout(()=>c.abort(),45000);
    try{pending.persist(command);const run=await privateAgentRequest(scope,`/${id}/reports`,qualityRun,{method:"POST",headers:{"Content-Type":"application/json","Idempotency-Key":command.key},body:JSON.stringify(command.input),signal:c.signal});
      if(live.current){pending.clear(command);setMessage("草稿质检报告已保存。");saved(run)}}
    catch(e){if(!live.current)return;
      if(e instanceof CustomizationError&&[400,404,412].includes(e.status)){
        if(verification)setMessage("暂时无法核实原检查，请保留原草稿引用与请求编号，恢复来源访问后核实。");
        else{pending.clear(command);setSelected(undefined);setReload(v=>v+1);setMessage("草稿版本或可用状态已变化，请重新选择当前草稿。");}
      }
      else setMessage(e instanceof CustomizationError&&e.code==="OUTCOME_UNKNOWN"?"结果尚未确认，请核实同一次检查。":"无法确认原检查，请保留原草稿引用与请求编号后核实。");
    }finally{clearTimeout(timer);flight.current=false;if(live.current)setBusy(false)}
  }
  return <Card className={`${styles.panel} ${qualityStyles.form}`}><h2>选择上传前的平台草稿</h2><p>直接检查保存的草稿，无需重填商品资料。当前支持 SHEIN 美国站。</p>
    {!canUse?<p>当前身份没有执行权限。</p>:null}{!canSelect?<p role="alert">需要当前商品资料、供应链和店铺读取权限，才能选择自己的草稿。</p>:null}
    {pending.error?<p role="alert">原检查记录无法读取，请保留浏览器数据并联系平台。</p>:null}{failure?<p role="alert">{failure}</p>:null}{choices.key===key&&choices.error?<p role="alert">{choices.error}</p>:null}{message?<p role="status">{message}</p>:null}
    {pending.command?<div role="status"><p>有一次检查尚未确认，核实会复用原草稿编号、版本和请求编号。</p><Button disabled={busy||!canUse||!canSelect} variant="outline" onClick={()=>pending.command&&void execute(pending.command,true)}>核实同一次检查</Button></div>:null}
    <fieldset disabled={locked||!canSelect||!canUse}>
      <label>供应链批次<Select value={prep} onChange={e=>{setPrep(e.target.value);setAfter(undefined)}}><option value="">请选择批次</option>{preparations.map(v=><option key={v.id} value={v.id}>{v.name}</option>)}</Select></label>
      <div className={styles.actions}>{prepNext?<Button type="button" variant="outline" onClick={()=>setPrepAfter(prepNext)}>更多批次</Button>:null}{prepAfter?<Button type="button" variant="outline" onClick={()=>setPrepAfter(undefined)}>批次首页</Button>:null}</div>
      <label>目标店铺<Select value={store} onChange={e=>{setStore(e.target.value);setAfter(undefined)}}><option value="">请选择店铺</option>{stores.map(v=><option key={v.id} value={v.id}>{v.name}</option>)}</Select></label>
      <div className={styles.actions}>{storePage>1?<Button type="button" variant="outline" onClick={()=>setStorePage(v=>v-1)}>上一页店铺</Button>:null}{storePage*20<storeTotal?<Button type="button" variant="outline" onClick={()=>setStorePage(v=>v+1)}>下一页店铺</Button>:null}</div>
      <label>草稿阶段<Select value={stage} onChange={e=>{setStage(e.target.value as "missing"|"ready");setAfter(undefined)}}><option value="missing">待补全</option><option value="ready">已适配</option></Select></label>
      <label>平台草稿<Select value={record?.id??""} disabled={!prep||!store||loading} onChange={e=>{const record=rows.find(v=>v.id===e.target.value);setSelected(record?{key,record}:undefined)}}><option value="">{loading?"正在读取草稿…":"请选择草稿"}</option>{rows.map(v=><option key={v.id} value={v.id}>{v.result.product.multi_language_name_list[0]?.name||v.source.source.productKey} · 草稿 v{v.revision}</option>)}</Select></label>
      {prep&&store&&!loading&&choices.key===key&&!choices.error&&!rows.length?<p>当前阶段没有可选草稿。请先在我的供应链保存平台资料。</p>:null}
      <div className={styles.actions}>{choices.key===key&&choices.next?<Button type="button" variant="outline" onClick={()=>setAfter(choices.next)}>更多草稿</Button>:null}{after?<Button type="button" variant="outline" onClick={()=>setAfter(undefined)}>草稿首页</Button>:null}<Button type="button" variant="outline" onClick={()=>setReload(v=>v+1)}>刷新草稿</Button></div>
      {record?<p>将检查草稿 v{record.revision}，保存时间 {new Date(record.createdAt).toLocaleString("zh-CN")}。报告与这个版本绑定。</p>:null}
      <Button type="button" disabled={!record} onClick={()=>record&&void execute({deliveryId:id,key:crypto.randomUUID(),input:{recordId:record.id,expectedRevision:record.revision}})}>{busy?"正在检查并保存…":"检查并保存报告"}</Button>
    </fieldset>
    {!offlineTrial?<Link href={prep&&store?`/workbench/supply/mine?preparation=${prep}&store=${store}`:"/workbench/supply/mine"}>前往我的供应链补全资料 →</Link>:null}
  </Card>;
}
