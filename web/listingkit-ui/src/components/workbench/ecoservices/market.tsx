"use client";
import Link from "next/link";
import Image from "next/image";
import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {ConsolePage,ConsoleState} from "@/components/workbench/console/console-page";
import {ResourceDialog} from "@/components/workbench/resources/resource-dialog";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {ecoRequest,ecoPageSchema,type EcoListing,type EcoScope} from "@/lib/api/ecoservices";
import {EcoBoundary,EcoPagination,ReadFailure,categories,yuan,useEcoCommands} from "./shared";
import {FileUpload} from "./files";
export function EcoservicesMarket(){return <EcoBoundary>{scope=><Market key={scope.userId+scope.organizationId} scope={scope}/>}</EcoBoundary>}
function Market({scope}:{scope:EcoScope}){
 const context=useWorkbenchContext(),[group,setGroup]=useState(""),[page,setPage]=useState(1),[selected,setSelected]=useState<EcoListing|null>(null),[description,setDescription]=useState(""),[files,setFiles]=useState<string[]>([]);
 const commands=useEcoCommands(scope);const canRead=context.permissions.includes("workbench.ecoservices.read"),canPurchase=context.permissions.includes("workbench.ecoservices.purchase");
 const query=useQuery({queryKey:["ecoservices",scope.userId,scope.organizationId,"catalog",group,page,context.permissions.join("|")],queryFn:({signal})=>ecoRequest(scope,"catalog?page="+page+"&pageSize=20"+(group?"&group="+group:""),ecoPageSchema,{signal}),enabled:canRead&&!context.error&&!context.blockingError&&!context.isLoading&&!context.isSwitching});
 const counts=query.data?.counts;const enterprise=(counts?.COMPANY_REGISTRATION??0)+(counts?.TRADEMARK_REGISTRATION??0),shop=(counts?.STORE_OPENING??0)+(counts?.STORE_OPERATION??0);
 return <ConsolePage className="eco-page" title="服务市场" description="围绕跨境业务所需的企业与店铺能力，由专业服务商提供支持。" breadcrumbs={[{label:"生态服务"},{label:"服务市场"}]} actions={<Link href="/workbench/services/mine">我的服务 →</Link>}>
  {commands.notice}
  {!canRead?<ConsoleState kind="unavailable" title="当前身份没有服务市场读取权限"/>:<>
  <Card className="eco-catalog-filter"><div><h2>服务分类</h2><p>根据业务需要选择合适的服务</p></div><div className="eco-category-tabs">{[["","全部服务",enterprise+shop],["enterprise","企业服务",enterprise],["shop","店铺服务",shop]].map(([id,label,count])=><Button key={id} variant="outline" aria-pressed={group===id} onClick={()=>{setGroup(String(id));setPage(1)}}>{label} <span>{query.data?count:"—"}</span></Button>)}</div></Card>
  {query.isPending?<ConsoleState kind="loading" title="正在读取服务目录"/>:query.error?<ReadFailure error={query.error} retry={()=>void query.refetch()}/>:query.data?.listings?.length?<><div className="eco-market-grid">{query.data.listings.map((listing,index)=>{const category=categories.find(c=>c.id===listing.category)!;return <Card className="eco-service-card" key={listing.id} data-tone={listing.category}><div className="eco-card-top"><span className="eco-tag">{category.group==="enterprise"?"企业服务":"店铺服务"}</span><span>{String((page-1)*20+index+1).padStart(2,"0")}</span></div><h2>{listing.title}</h2><p className="eco-card-description">{listing.description}</p><div className="eco-includes"><strong>包含服务</strong><ul>{listing.items.slice(0,3).map((item,i)=><li key={i}><Image width={5} height={5} src={"/figma/ecoservices/"+(listing.category==="STORE_OPENING"?"cfdd2.svg":listing.category==="STORE_OPERATION"?"03b8d.svg":"eea41.svg")} alt=""/>{item}</li>)}</ul></div><p className="eco-provider">{listing.providerName} · {yuan(listing.priceMinor)}起</p><Button variant="outline" onClick={()=>{setSelected(listing);setDescription("");setFiles([])}}>查看详情 →</Button></Card>})}</div><EcoPagination page={page} total={query.data.total} onPage={setPage}/></>:<ConsoleState kind="empty" title="暂无已发布服务">服务商通过审核并发布后，会在这里显示真实服务内容。</ConsoleState>}
  <Card className="eco-market-explainer"><div><small>服务流程</small><h2>查看详情 → 提交需求 → 服务商确认 → 我的服务跟进</h2><p>提交后可在“我的服务”查看报价、交付节点和售后进度。</p></div><div><small>服务说明</small><h2>具体服务信息在详情页明确展示</h2><p>服务地区、费用和交付周期由服务商根据实际能力提供。</p></div></Card></>}
  {selected?<ResourceDialog title={selected.title} onClose={()=>setSelected(null)} locked={commands.locked}><div className="eco-detail"><p>{selected.description}</p><dl><dt>服务商</dt><dd>{selected.providerName}</dd><dt>参考价</dt><dd>{yuan(selected.priceMinor)}，成交以确认报价为准</dd><dt>预计周期</dt><dd>{selected.deliveryDays} 天</dd><dt>服务地区</dt><dd>{selected.regions.join("、")}</dd><dt>适用平台</dt><dd>{selected.platforms?.join("、")||"按报价约定"}</dd></dl><ul>{selected.items.map((v,i)=><li key={i}>{v}</li>)}</ul><form onSubmit={event=>{event.preventDefault();void commands.json("catalog/"+selected.id+"/requests",{description,fileIds:files}).then(()=>setSelected(null)).catch(()=>undefined)}}><fieldset disabled={!canPurchase||commands.locked}><label>具体需求<textarea required maxLength={10000} value={description} onChange={e=>setDescription(e.target.value)}/></label><FileUpload commands={commands} path="requests/files" ids={files} onFiles={setFiles}/><Button type="submit">提交需求</Button></fieldset></form>{!canPurchase?<p>当前身份没有购买或提交需求权限。</p>:null}{commands.notice}</div></ResourceDialog>:null}
 </ConsolePage>;
}
