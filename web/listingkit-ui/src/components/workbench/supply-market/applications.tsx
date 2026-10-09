/* eslint-disable @next/next/no-img-element */
"use client";
import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import Link from "next/link";
import {ConsolePage,ConsoleState,ConsoleToolbar} from "@/components/workbench/console/console-page";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Select} from "@/components/ui/select";
import {Textarea} from "@/components/ui/textarea";
import {listOwnProducts} from "@/lib/api/product-collection";
import {listMarketRecords,readMarketChoice,type MarketScope} from "@/lib/api/supply-market";
import {MarketBoundary,MarketLinks,MarketReadError,CursorButtons,marketStages,useMarketCommands} from "./shared";
import {Qualifications} from "./qualifications";
export function SupplyApplicationsPage({admin=false}:{admin?:boolean}){return <MarketBoundary admin={admin}>{scope=><Applications scope={scope} admin={admin}/>}</MarketBoundary>}
function Applications({scope,admin}:{scope:MarketScope;admin:boolean}){
 const [ended,setEnded]=useState(false),[kind,setKind]=useState<"selected"|"official"|"connection">("selected"),[after,setAfter]=useState<string>();
 const q=useQuery({queryKey:["supply-market",scope.userId,scope.organizationId,"records",admin,kind,ended,after],queryFn:({signal})=>listMarketRecords(scope,{kind,ended,after},admin,signal),retry:false});
 return <ConsolePage title={admin?"供应发布与人工评估":"优选申请"} description={admin?"按实际资质和线下合作评估，审批后明确发布。":"提交本人自有且已有实际优化结果的商品，查看人工评估进展。"} actions={!admin?<Link href="/workbench/supply/applications/new" prefetch={false}>申请加入硕米优选</Link>:null}><MarketLinks/>
  <ConsoleToolbar><div className="flex gap-3"><Select aria-label="申请类型" value={kind} onChange={e=>{setKind(e.target.value as typeof kind);setAfter(undefined)}}><option value="selected">优选申请</option><option value="connection">货盘对接</option><option value="official">自营发布草稿</option></Select><Button variant={ended?"outline":"default"} onClick={()=>{setEnded(false);setAfter(undefined)}}>进行中</Button><Button variant={ended?"default":"outline"} onClick={()=>{setEnded(true);setAfter(undefined)}}>已结束</Button></div></ConsoleToolbar>
  {q.isPending?<ConsoleState kind="loading" title="正在读取申请"/>:q.error?<MarketReadError error={q.error} retry={()=>void q.refetch()}/>:q.data.items.length===0?<ConsoleState kind="empty" title="暂无申请记录"/>:<div className="space-y-3">{q.data.items.map(r=><Card key={r.id} className="flex items-center justify-between gap-3 p-4"><div><h2>{r.product?.title||r.connection?.name}</h2><p>{marketStages[r.stage]} · {new Date(r.createdAt).toLocaleString("zh-CN")}</p>{r.stage==="APPROVED"?<p>已通过人工评估；市场发布为单独操作。</p>:null}</div><Link href={(admin?"/workbench/supply/operator/":"/workbench/supply/applications/")+r.id} prefetch={false}>查看进展</Link></Card>)}</div>}
  {q.data?<CursorButtons after={after} next={q.data.nextCursor} total={q.data.total} onAfter={setAfter}/>:null}{!admin?<Link href="/workbench/supply/official/new" prefetch={false}>平台专员：选择本人商品，建立自营发布草稿</Link>:null}
 </ConsolePage>;
}
export function SupplyApplicationForm({official=false}:{official?:boolean}){return <MarketBoundary>{scope=><ApplicationForm scope={scope} official={official}/>}</MarketBoundary>}
function ApplicationForm({scope,official}:{scope:MarketScope;official:boolean}){
 const [keyword,setKeyword]=useState(""),[after,setAfter]=useState<string>(),[item,setItem]=useState(""),[fileIds,setFileIDs]=useState<string[]>([]),[uploadLocked,setUploadLocked]=useState(false),[disclosure,setDisclosure]=useState(false);
 const [supply,setSupply]=useState({stock:0,capacity:"",minimumQuantity:1,leadDays:0,province:"",city:"",priceNote:"",afterSaleNote:""});
 const own=useQuery({queryKey:["supply-market",scope.userId,scope.organizationId,"own",keyword,after],queryFn:({signal})=>listOwnProducts(scope,{keyword,after,limit:20},signal),retry:false});
 const choice=useQuery({queryKey:["supply-market",scope.userId,scope.organizationId,"choice",item],queryFn:({signal})=>readMarketChoice(scope,item,signal),enabled:!!item,retry:false}),commands=useMarketCommands(scope);
 const blocked=commands.locked||!!commands.saved||uploadLocked||!choice.data||!disclosure||!official&&!choice.data.optimized||!official&&!fileIds.length;
 return <ConsolePage title={official?"建立自营发布草稿":"申请加入硕米优选"} description={official?"平台专员先选择本人的自有商品，再以平台权限明确发布。":"仅限本人/企业自有商品，须已有实际优化结果。由专员人工评估并线下确认合作。"}><MarketLinks/>{commands.notice}{commands.saved?.recordId?<Card className="p-4" role="status">已保存。<Link href={"/workbench/supply/applications/"+commands.saved.recordId} prefetch={false}>查看记录</Link></Card>:null}
  <Card className="space-y-4 p-6"><h2>选择本人自有商品</h2><Input aria-label="筛选本人商品" placeholder="商品关键词" value={keyword} maxLength={80} onChange={e=>{setKeyword(e.target.value);setAfter(undefined)}}/>{own.isPending?<p>正在读取商品</p>:own.error?<MarketReadError error={own.error} retry={()=>void own.refetch()}/>:<><Select aria-label="申请商品" value={item} disabled={commands.locked} onChange={e=>setItem(e.target.value)}><option value="">请选择商品</option>{own.data.items.map(i=><option key={i.id} value={i.id}>{i.title||i.source.productKey}</option>)}</Select><CursorButtons after={after} next={own.data.nextCursor} total={own.data.total} onAfter={setAfter}/></>}
   {choice.isFetching?<p>正在核对商品版本与优化记录</p>:choice.error?<MarketReadError error={choice.error} retry={()=>void choice.refetch()}/>:choice.data?<div className="flex gap-4"><img src={choice.data.product.images[0]} alt={choice.data.product.title} className="h-24 w-24 object-contain"/><div><h3>{choice.data.product.title}</h3><p>{choice.data.optimized?"已有实际 Apply 优化记录":"尚无实际优化结果，暂不能申请优选"}</p></div></div>:null}
  </Card>
  <form className="space-y-4" onSubmit={e=>{e.preventDefault();if(blocked||!choice.data)return;void commands.execute(official?{action:"create_official_draft",selection:choice.data.selection,supply,disclosure:true}:{action:"submit_selected",selection:choice.data.selection,supply,fileIds,disclosure:true})}}><Card className="space-y-4 p-6"><h2>供货声明</h2><div className="grid gap-4 sm:grid-cols-2">{[{key:"stock",label:"现有库存（件）",min:0,max:1e9},{key:"minimumQuantity",label:"最低起订量（件）",min:1,max:1e9},{key:"leadDays",label:"供货周期（天）",min:0,max:3650}].map(f=><label key={f.key}>{f.label}<Input type="number" required min={f.min} max={f.max} value={supply[f.key as "stock"|"minimumQuantity"|"leadDays"]} onChange={e=>setSupply({...supply,[f.key]:Number(e.target.value)})}/></label>)}<label>生产能力<Input value={supply.capacity} required={supply.stock===0} maxLength={4000} onChange={e=>setSupply({...supply,capacity:e.target.value})}/></label><label>发货省份<Input required value={supply.province} maxLength={100} onChange={e=>setSupply({...supply,province:e.target.value})}/></label><label>发货城市<Input required value={supply.city} maxLength={100} onChange={e=>setSupply({...supply,city:e.target.value})}/></label></div><label className="block">供货价格说明<Textarea value={supply.priceNote} maxLength={4000} onChange={e=>setSupply({...supply,priceNote:e.target.value})}/></label><label className="block">售后说明<Textarea value={supply.afterSaleNote} maxLength={4000} onChange={e=>setSupply({...supply,afterSaleNote:e.target.value})}/></label>
   {!official?<Qualifications scope={scope} ids={fileIds} onIDs={setFileIDs} onLocked={setUploadLocked}/>:null}<label className="flex gap-2"><input type="checkbox" checked={disclosure} onChange={e=>setDisclosure(e.target.checked)}/>我确认这是本人/企业自有商品，并允许平台评估；明确发布后，列明的商品字段及供货声明可供其他获授权企业查看。</label><Button type="submit" disabled={blocked}>{official?"保存发布草稿":"提交人工评估"}</Button>
  </Card></form>
 </ConsolePage>;
}
