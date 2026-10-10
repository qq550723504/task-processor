"use client";
import {useEffect,useRef,useState} from "react";
import {z} from "zod";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {configurationError,useConfigurationRead} from "./agent-configuration-hooks";
import {type ConfigurationScope} from "@/lib/api/agent-configuration";
import {imageSetRequest,ImageSetError} from "@/lib/api/product-image-set";
import {imageSetRecentSchema,imageSetRunSchema,type ImageSetRun} from "@/lib/contracts/product-image-set";
import {ProductImageSetPanel} from "../acquisition/product-image-set-panel";
const states:Record<string,string>={awaiting_plan_approval:"待确认计划",executing:"生成中",evaluating:"处理结果中",repairing:"核实原调用中",awaiting_final_approval:"待人工审核",completed:"已保存素材",blocked:"已停止",failed:"失败",cancelled:"已取消"};
export function RecentImageSets({scope,nonce}:{scope:ConfigurationScope;nonce:number}){
 const [cursor,setCursor]=useState(""),[selected,setSelected]=useState<ImageSetRun>(),[busy,setBusy]=useState(false),[error,setError]=useState("");
 const abort=useRef<AbortController|null>(null);
 useEffect(()=>()=>abort.current?.abort(),[]);
 const page=useConfigurationRead(scope,`product.image.agent/recent-runs?pageSize=20${cursor?`&cursor=${encodeURIComponent(cursor)}`:""}`,imageSetRecentSchema,nonce);
 async function open(item:z.infer<typeof imageSetRecentSchema>["items"][number]){
  const controller=new AbortController();abort.current?.abort();abort.current=controller;setBusy(true);setError("");try{const run=await imageSetRequest({...scope,kind:item.contextKind,contextId:item.contextId},"read",imageSetRunSchema,{runId:item.runId,signal:controller.signal});if(!controller.signal.aborted)setSelected(run)}catch(e){if(!controller.signal.aborted)setError(e instanceof ImageSetError?e.code:"IMAGE_UNAVAILABLE")}finally{if(!controller.signal.aborted)setBusy(false)}
 }
 return <Card className="space-y-4 p-6"><h2 className="font-semibold">最近图片任务</h2>{page.error?<p role="alert">{configurationError(page.error)}</p>:!page.data?<p>正在读取原任务…</p>:page.data.items.length?<ul className="space-y-3">{page.data.items.map(item=><li key={item.runId} className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3"><div><p className="text-sm">{states[item.status]??item.status} · {item.contextKind==="acquisition"?"采集商品":"供应链商品"} · {item.targetPlatform==="product"?"通用素材":item.targetPlatform.toUpperCase()}</p><p className="text-xs text-slate-500">{new Date(item.createdAt).toLocaleString()} · {item.runId}</p></div><Button variant="outline" size="sm" disabled={busy} onClick={()=>void open(item)}>查看原任务</Button></li>)}</ul>:<p className="text-sm text-slate-500">当前身份在本企业还没有图片任务。</p>}{error?<p role="alert">{error}</p>:null}{cursor||page.data?.nextCursor?<nav aria-label="图片任务分页" className="flex gap-2">{cursor?<Button variant="outline" onClick={()=>setCursor("")}>返回第一页</Button>:null}{page.data?.nextCursor?<Button variant="outline" onClick={()=>setCursor(page.data!.nextCursor)}>下一页</Button>:null}</nav>:null}{selected?<ProductImageSetPanel key={selected.runId} kind={selected.plan.Source.ContextKind} contextId={selected.plan.Source.OperationID} initialRunId={selected.runId} effectiveVersion={selected.plan.Source.EffectiveVersion} applyReceiptId={selected.plan.Source.ApplyReceiptID} target={selected.plan.Target.Platform==="shein"?selected.plan.Target:undefined}/>:null}</Card>;
}
