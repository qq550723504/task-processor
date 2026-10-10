"use client";
import Link from "next/link";
import {useQuery} from "@tanstack/react-query";
import {ConsolePage,ConsoleState} from "../console/console-page";
import {MarketBoundary,MarketReadError} from "../supply-market/shared";
import {readPODProgress} from "@/lib/api/pod";
import type {MarketScope} from "@/lib/api/supply-market";
import {podReceiptSchema} from "@/lib/contracts/pod";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {usePODCommands} from "./commands";
const states={QUEUED:"设计操作已保存，等待处理",MATERIAL_PENDING:"正在登记素材",DESIGN_PENDING:"正在准备保存设计",VERIFYING:"正在核实成品及效果图",UNKNOWN:"结果待核实",SAVED:"SDS 成品已保存"};
export function PODOperationPage({id}:{id:string}){return <MarketBoundary>{scope=><Operation scope={scope} id={id}/>}</MarketBoundary>}
function Operation({scope,id}:{scope:MarketScope;id:string}){
 const key=["pod",scope.userId,scope.organizationId,"operation",id],commands=usePODCommands(scope);
 const operation=useQuery({queryKey:key,queryFn:({signal})=>readPODProgress(scope,id,false,signal),refetchInterval:q=>q.state.data&&!['SAVED','UNKNOWN'].includes(q.state.data.state)?5000:false,refetchIntervalInBackground:false});
 const observed=useQuery({queryKey:[...key,"verify"],queryFn:({signal})=>readPODProgress(scope,id,true,signal),enabled:false});
 const current=observed.data&&observed.dataUpdatedAt>operation.dataUpdatedAt?observed.data:operation.data;
 const receipt=commands.saved?.intent.kind==="finished"&&commands.saved.intent.id===id?podReceiptSchema.safeParse(commands.saved.result):null;
 return <ConsolePage title={current?.name??"SDS 定制操作"} breadcrumbs={[{label:"SDS 模板",href:"/workbench/supply/catalogs/sds"},{label:"定制结果"}]}>{commands.notice}
 {operation.isPending?<ConsoleState kind="loading" title="正在读取原设计操作"/>:operation.error?<MarketReadError error={operation.error} retry={()=>void operation.refetch()}/>:current?<><Card className="space-y-3 p-5"><p role="status">{states[current.state]}</p><p>原操作编号：{current.id}</p>{current.state==="UNKNOWN"?<p>尚未确认保存结果。系统保留原操作，核实不会再次上传图案或保存成品。</p>:null}{current.state!=="SAVED"?<Button variant="outline" disabled={observed.isFetching} onClick={()=>void observed.refetch()}>核实原操作成品</Button>:null}{observed.error?<MarketReadError error={observed.error} retry={()=>void observed.refetch()}/>:null}</Card>
 {current.finished?<><Card className="space-y-3 p-5"><p>SDS 成品编号：{current.finished.id}</p><p>商品号：{current.finished.number}</p><div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{current.finished.images.map((src,i)=><img key={src} src={src} className="aspect-square w-full rounded object-contain" alt={"SDS 成品效果图 "+(i+1)}/>)}</div><Button disabled={commands.locked||!!receipt?.success} onClick={()=>void commands.execute({kind:"finished",id,body:{}})}>保存成品到我的数据</Button></Card>{receipt?.success?<Card className="space-y-2 p-4" role="status"><p>成品已保存到我的数据。</p><Link href="/workbench/data/mine" prefetch={false}>打开我的数据</Link><p>选择“SDS 定制成品”批次中的商品，转入我的供应链后继续适配与上传。</p></Card>:null}</>:null}
 </>:null}</ConsolePage>;
}
