"use client";
import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import Link from "next/link";
import {ConsolePage,ConsoleState} from "../console/console-page";
import {MarketBoundary,MarketLinks,MarketReadError} from "../supply-market/shared";
import {Card} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Button} from "@/components/ui/button";
import {listPODTemplates,readPODTemplate} from "@/lib/api/pod";
import type {MarketScope} from "@/lib/api/supply-market";
import {podReceiptSchema} from "@/lib/contracts/pod";
import {usePODCommands} from "./commands";

export function PODCatalogPage(){return <MarketBoundary>{scope=><Catalog scope={scope}/>}</MarketBoundary>}
function Catalog({scope}:{scope:MarketScope}){
 const [keyword,setKeyword]=useState(""),[search,setSearch]=useState(""),[page,setPage]=useState(1),[id,setID]=useState("");
 const commands=usePODCommands(scope),key=["pod",scope.userId,scope.organizationId];
 const list=useQuery({queryKey:[...key,"templates",page,search],queryFn:({signal})=>listPODTemplates(scope,page,search,signal)});
 const detail=useQuery({queryKey:[...key,"template",id],queryFn:({signal})=>readPODTemplate(scope,id,signal),enabled:!!id});
 const receipt=commands.saved?.intent.kind==="template"?podReceiptSchema.safeParse(commands.saved.result):null;
 return <ConsolePage title="SDS 定制货盘" description="先保存模板，再选择款式与本人批准的图案。模板尚未定制，成品保存后可加入我的数据。"><MarketLinks/>{commands.notice}
 <Link href="/workbench/supply/catalogs/sds/design" prefetch={false}>使用我的数据中的模板继续定制</Link>
 {receipt?.success?<Card className="space-y-2 p-4" role="status"><p>模板已保存到我的数据，尚未完成定制。</p><Link href={"/workbench/supply/catalogs/sds/design?template="+receipt.data.itemId} prefetch={false}>选择款式并开始定制</Link></Card>:null}
 <form className="flex gap-2" onSubmit={e=>{e.preventDefault();setSearch(keyword);setPage(1);setID("")}}><Input aria-label="搜索 SDS 模板" maxLength={200} value={keyword} onChange={e=>setKeyword(e.target.value)}/><Button type="submit">搜索</Button></form>
 {list.isPending?<ConsoleState kind="loading" title="正在读取 SDS 模板"/>:list.error?<MarketReadError error={list.error} retry={()=>void list.refetch()}/>:list.data?.items.length===0?<ConsoleState kind="empty" title="没有符合条件的模板"/>:<div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{list.data?.items.map(t=><Card className="space-y-3 p-4" key={t.id}><img className="aspect-square w-full rounded-lg object-contain" src={t.images[0]} alt={t.name}/><h2>{t.name}</h2><p className="text-sm text-muted-foreground">SKU：{t.sku||"未提供"}</p><Button variant="outline" onClick={()=>setID(t.id)}>查看款式与模板</Button></Card>)}</div>}
 <div className="flex items-center gap-3"><Button variant="outline" disabled={page===1||list.isFetching} onClick={()=>setPage(page-1)}>上一页</Button><span>第 {page} 页{list.data?`，共 ${list.data.total} 项`:""}</span><Button variant="outline" disabled={!list.data||page*list.data.size>=list.data.total||list.isFetching} onClick={()=>setPage(page+1)}>下一页</Button></div>
 {id?<Card className="space-y-3 p-5"><h2>模板详情</h2>{detail.isPending?<p role="status">正在读取…</p>:detail.error?<MarketReadError error={detail.error} retry={()=>void detail.refetch()}/>:detail.data?<><h3>{detail.data.template.name}</h3><div className="flex flex-wrap gap-2">{detail.data.template.images.map(src=><img key={src} className="h-32 w-32 rounded object-contain" src={src} alt="模板参考图"/>)}</div><p>当前模板包含 {detail.data.template.variants.length} 个款式。保存后选择款式，系统再核对可编辑区域。</p><ul>{detail.data.template.variants.map(v=><li key={v.id}>{v.name||v.sku||v.id} · {v.color} {v.size}{v.type==="FREE"?"":" · 当前定制方式暂不支持"}</li>)}</ul><Button disabled={commands.locked} onClick={()=>void commands.execute({kind:"template",body:{id:detail.data!.template.id,hash:detail.data!.hash}})}>保存模板到我的数据</Button></>:null}</Card>:null}
 </ConsolePage>;
}
