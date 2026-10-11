"use client";
import {useState} from "react";
import Link from "next/link";
import {ConsolePage} from "@/components/workbench/console/console-page";
import {Card} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import {Button} from "@/components/ui/button";
import type {MarketScope} from "@/lib/api/supply-market";
import {MarketBoundary,MarketLinks,useMarketCommands} from "./shared";
type CatalogView="selection"|"apply";
const catalogBreadcrumbs=[{label:"供应市场",href:"/workbench/supply"},{label:"货盘集成",href:"/workbench/supply/catalogs"}];
export function SupplyCatalogsPage({acquisitionAvailable=false,sdsAvailable=false,view}:{acquisitionAvailable?:boolean;sdsAvailable?:boolean;view?:CatalogView}){return <MarketBoundary>{scope=>view==="apply"?<CatalogConnection scope={scope}/>:view==="selection"?<CatalogSelection acquisitionAvailable={acquisitionAvailable} sdsAvailable={sdsAvailable}/>:<CatalogOverview/>}</MarketBoundary>}
function CatalogOverview(){return <ConsolePage title="货盘集成" description="沿现有货盘能力选品；需要其他货盘时，提交资料由平台人工评估。" breadcrumbs={catalogBreadcrumbs}><MarketLinks/><div className="grid gap-4 sm:grid-cols-2"><Card className="space-y-3 p-6"><h2>货盘选品</h2><p>查看当前实例的货盘能力，进入选品与采集，再沿现有流程保存到自己的数据与供应链。</p><Link href="/workbench/supply/catalogs/selection" prefetch={false}>进入货盘选品</Link></Card><Card className="space-y-3 p-6"><h2>申请对接</h2><p>提交货盘资料，由专员人工评估并线下确认技术与合作方案。</p><Link href="/workbench/supply/catalogs/apply" prefetch={false}>申请对接货盘</Link></Card></div></ConsolePage>}
function CatalogSelection({acquisitionAvailable,sdsAvailable}:{acquisitionAvailable:boolean;sdsAvailable:boolean}){return <ConsolePage title="货盘选品" description="查看当前实例的货盘能力，进入对应的选品与采集流程。" breadcrumbs={[...catalogBreadcrumbs,{label:"货盘选品"}]}><MarketLinks/><div className="grid gap-4 sm:grid-cols-2"><Card className="space-y-3 p-6"><h2>1688 选品与采集</h2><p>从链接或插件采集商品，查看保存结果，再选入我的供应链。网页采集沿现有资源计量规则。</p>{acquisitionAvailable?<Link href="/workbench/supply/acquisition" prefetch={false}>进入 1688 选品与采集</Link>:<p>当前实例未配置采集能力</p>}</Card><Card className="space-y-3 p-6"><h2>SDS POD 定制货盘</h2><p>选择模板与款式，使用本人已批准的图案设计，核实效果图和已保存成品后加入供应链。</p>{sdsAvailable?<Link href="/workbench/supply/catalogs/sds" prefetch={false}>查看模板与定制成品</Link>:<p>当前实例未配置 SDS 定制能力</p>}</Card></div></ConsolePage>}
function CatalogConnection({scope}:{scope:MarketScope}){
 const commands=useMarketCommands(scope),[input,setInput]=useState({name:"",website:"",contact:"",telephone:"",categories:""});
 return <ConsolePage title="申请对接" description="填写货盘基本信息，由专员人工评估并线下确认技术与合作方案。" breadcrumbs={[...catalogBreadcrumbs,{label:"申请对接"}]}><MarketLinks/>
  <Card className="space-y-4 p-6"><h2>申请对接其他货盘</h2><p>提交后由平台专员人工评估，并线下确认技术与合作方案。确认方案不会自动接入连接器。</p>{commands.notice}{commands.saved?.recordId?<p role="status">申请已保存。<Link href={"/workbench/supply/applications/"+commands.saved.recordId} prefetch={false}>查看对接进展</Link></p>:null}<form className="space-y-3" onSubmit={e=>{e.preventDefault();void commands.execute({action:"submit_connection",connection:input})}}><div className="grid gap-3 sm:grid-cols-2">{[{key:"name",label:"货盘名称",type:"text",max:200},{key:"website",label:"货盘网站（HTTPS）",type:"url",max:2048},{key:"contact",label:"联系人",type:"text",max:100},{key:"telephone",label:"联系电话",type:"tel",max:100}].map(f=><label key={f.key}>{f.label}<Input required type={f.type} maxLength={f.max} value={input[f.key as keyof typeof input]} onChange={e=>setInput({...input,[f.key]:e.target.value})}/></label>)}</div><label className="block">供应品类<Textarea required maxLength={4000} value={input.categories} onChange={e=>setInput({...input,categories:e.target.value})}/></label><Button disabled={commands.locked||!!commands.saved} type="submit">提交对接申请</Button></form></Card>
 </ConsolePage>;
}
