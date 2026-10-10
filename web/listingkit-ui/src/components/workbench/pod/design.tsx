"use client";
import {useState} from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import {useQuery} from "@tanstack/react-query";
import {ConsolePage} from "../console/console-page";
import {MarketBoundary,MarketReadError} from "../supply-market/shared";
import {readCollectionItem} from "@/lib/api/product-collection";
import {readPODManifest,readPODArtwork} from "@/lib/api/pod";
import type {MarketScope} from "@/lib/api/supply-market";
import {podApprovalSchema,podProgressSchema,type PODInput} from "@/lib/contracts/pod";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {CollectionPicker} from "./collection-picker";
import {usePODCommands} from "./commands";
import type {LayoutTransform} from "./canvas";
const DesignCanvas=dynamic(()=>import("./canvas").then(m=>m.DesignCanvas),{ssr:false});

export function PODDesignPage({templateID="",variantID=""}:{templateID?:string;variantID?:string}){return <MarketBoundary>{scope=><Design scope={scope} initialTemplate={templateID} initialVariant={variantID}/>}</MarketBoundary>}
function Design({scope,initialTemplate,initialVariant}:{scope:MarketScope;initialTemplate:string;initialVariant:string}){
 const commands=usePODCommands(scope),key=["pod",scope.userId,scope.organizationId];
 const [templateID,setTemplate]=useState(initialTemplate),[variantID,setVariant]=useState(initialVariant),[artworkID,setArtwork]=useState(""),[imageID,setImage]=useState("");
 const [name,setName]=useState(""),[transforms,setTransforms]=useState<Record<string,LayoutTransform>>({}),[layerID,setLayer]=useState(""),[ready,setReady]=useState(false);
 const template=useQuery({queryKey:[...key,"template-item",templateID],queryFn:({signal})=>readCollectionItem(scope,templateID,signal),enabled:!!templateID});
 const isTemplate=template.data?.item.source.kind==="sds_template"&&!template.data.item.archivedAt;
 const variants=isTemplate?template.data?.product.variants?.filter(v=>v.source_id&&/^[1-9][0-9]{0,63}$/.test(v.source_id))??[]:[];
 const variant=variants.find(v=>v.source_id===variantID)?.source_id??variants[0]?.source_id??"";
 const manifest=useQuery({queryKey:[...key,"manifest",templateID,variant],queryFn:({signal})=>readPODManifest(scope,templateID,variant,signal),enabled:!!variant&&!!isTemplate});
 const artwork=useQuery({queryKey:[...key,"artwork",artworkID],queryFn:({signal})=>readPODArtwork(scope,artworkID,signal),enabled:!!artworkID});
 const savedApproval=commands.saved?.intent.kind==="approval"&&commands.saved.intent.body.selection.itemId===artworkID&&commands.saved.intent.body.images[0]?.id===imageID&&JSON.stringify(commands.saved.intent.body.selection)===JSON.stringify(artwork.data?.selection)?podApprovalSchema.safeParse(commands.saved.result):null;
 const approval=savedApproval?.success?savedApproval.data:null;
 const submitted=commands.saved?.intent.kind==="design"?podProgressSchema.safeParse(commands.saved.result):null;
 const layers=manifest.data?.manifest.layers??[],region=layers.find(l=>l.id===layerID)??layers[0];
 const transform=(id:string):LayoutTransform=>transforms[id]??{layerId:id,x:.5,y:.5,scale:1,angle:0};
 const chosen=artwork.data?.images.find(i=>i.id===imageID);
 const input:PODInput|undefined=isTemplate&&template.data?{itemId:template.data.item.id,revision:template.data.item.revision,source:template.data.item.source}:undefined;
 const locked=commands.locked||!!submitted?.success;
 return <ConsolePage title="SDS 图案定制" description="使用我的数据中的模板和图案。布局预览用于调整印花区域，SDS 保存后生成成品效果图。" breadcrumbs={[{label:"货盘集成",href:"/workbench/supply/catalogs"},{label:"SDS 模板",href:"/workbench/supply/catalogs/sds"},{label:"定制"}]}>
 {commands.notice}{submitted?.success?<Card className="space-y-3 p-4" role="status"><p>原设计操作已保存。请在结果页核实成品。</p><Link href={"/workbench/supply/catalogs/sds/operations/"+submitted.data.id} prefetch={false}>查看原操作进展</Link></Card>:null}
 <div className="grid gap-5 xl:grid-cols-2"><Card className="space-y-4 p-5"><h2>1. 选择已保存模板</h2><CollectionPicker scope={scope} kind="template" disabled={locked} onSelect={item=>{setTemplate(item.id);setVariant("");setTransforms({});setLayer("")}}/>
 {template.error?<MarketReadError error={template.error} retry={()=>void template.refetch()}/>:template.data?<><h3>{template.data.item.title||template.data.product.title}</h3>{!isTemplate?<p role="alert">该商品不是可用的 SDS 模板。</p>:<label className="block">款式<select className="mt-1 block w-full rounded-md border bg-background p-2" value={variant} disabled={locked} onChange={e=>{setVariant(e.target.value);setTransforms({});setLayer("")}}>{variants.map(v=><option key={v.source_id} value={v.source_id}>{v.title||v.sku||v.source_id}</option>)}</select></label>}</>:null}
 {manifest.isFetching&&variant?<p role="status">正在核对 SDS 可编辑区域…</p>:null}{manifest.error?<MarketReadError error={manifest.error} retry={()=>void manifest.refetch()}/>:manifest.data?<p>{layers.length} 个印花区域，保存后生成 {manifest.data.manifest.renderFiles.length} 张效果图。</p>:null}
 </Card><Card className="space-y-4 p-5"><h2>2. 选择并批准图案</h2><p>请确认图案的使用权。当前支持 3 MiB 以内的 JPG、PNG。</p><CollectionPicker scope={scope} kind="artwork" disabled={locked} onSelect={item=>{setArtwork(item.id);setImage("");setReady(false)}}/>
 {artwork.error?<MarketReadError error={artwork.error} retry={()=>void artwork.refetch()}/>:artwork.data?<><p>选择商品图片作为本次图案：</p><div className="grid grid-cols-3 gap-2">{artwork.data.images.map(image=><button type="button" key={image.id} disabled={locked} aria-label={"选择图案 "+image.id} aria-pressed={imageID===image.id} className={"rounded border-2 p-1 "+(imageID===image.id?"border-primary":"border-transparent")} onClick={()=>{setImage(image.id);setReady(false)}}><img src={image.url} className="aspect-square w-full object-contain" alt="可选图案"/></button>)}</div>{artwork.data.images.length===0?<p>该商品没有可用图片，请先在我的数据中补充图案。</p>:null}<Button disabled={!chosen||locked||!!approval} onClick={()=>void commands.execute({kind:"approval",body:{actionId:crypto.randomUUID(),selection:artwork.data!.selection,images:[{id:imageID,role:"design"}],approved:[]}})}>批准选中图案用于 SDS 定制</Button></>:null}
 {approval?<p role="status">图案已批准，布局使用该批准记录。</p>:null}
 {commands.saved?.intent.kind==="approval"&&!approval?<Button variant="outline" disabled={locked} onClick={()=>{const original=commands.saved!.intent;if(original.kind==="approval"){setArtwork(original.body.selection.itemId);setImage(original.body.images[0]!.id);setReady(false)}}}>查看原操作批准的图案</Button>:null}
 </Card></div>
 {region&&approval?<Card className="space-y-4 p-5"><h2>3. 调整印花位置</h2><label className="block">印花区域<select className="mt-1 rounded-md border bg-background p-2" value={region.id} disabled={locked} onChange={e=>setLayer(e.target.value)}>{layers.map((l,i)=><option key={l.id} value={l.id}>区域 {i+1} · {l.printWidth} × {l.printHeight} 像素</option>)}</select></label><div className="grid items-start gap-5 lg:grid-cols-[minmax(0,600px)_minmax(200px,1fr)]"><DesignCanvas key={approval.actionId+region.id} region={region} image={approval.image} transform={transform(region.id)} onChange={t=>setTransforms({...transforms,[t.layerId]:t})} onReady={setReady} disabled={locked}/><div className="space-y-4"><p>拖动图案或使用数值控制。边框为印花区域，超出部分会被裁切。</p>{[{field:"x",label:"水平位置（%）",min:0,max:100,step:1,factor:100},{field:"y",label:"垂直位置（%）",min:0,max:100,step:1,factor:100},{field:"scale",label:"缩放（%）",min:5,max:400,step:1,factor:100},{field:"angle",label:"旋转（度）",min:-180,max:180,step:1,factor:1}].map(control=><label className="block" key={control.field}>{control.label}<Input type="number" min={control.min} max={control.max} step={control.step} disabled={locked} value={Number((transform(region.id)[control.field as "x"|"y"|"scale"|"angle"]*control.factor).toFixed(2))} onChange={e=>{const v=Number(e.target.value);if(Number.isFinite(v))setTransforms({...transforms,[region.id]:{...transform(region.id),[control.field]:Math.max(control.min,Math.min(control.max,v))/control.factor}})}}/></label>)}<Button variant="outline" disabled={locked} onClick={()=>setTransforms({...transforms,[region.id]:{layerId:region.id,x:.5,y:.5,scale:1,angle:0}})}>居中并适配区域</Button></div></div>
 <form className="space-y-3" onSubmit={e=>{e.preventDefault();if(!input||!manifest.data||!artwork.data||!ready||locked)return;void commands.execute({kind:"design",body:{templateItem:input,variantId:variant,manifestHash:manifest.data.hash,artworkHash:approval.image.hash,artworkItem:artwork.data.item,effectiveVersion:artwork.data.item.source.version,actionId:approval.actionId,assetId:approval.assetIds[0]!,transforms:layers.map(l=>transform(l.id)),name:name.trim()}})}}><label className="block">成品名称<Input required maxLength={60} value={name} disabled={locked} onChange={e=>setName(e.target.value)}/></label><p>提交将向硕米平台 SDS 账号上传本图案并保存成品，操作和成品归当前企业及本人所有。结果不明确时保留原操作并核实。</p><Button type="submit" disabled={locked||!ready||!name.trim()||manifest.isFetching||artwork.isFetching}>生成效果图并保存 SDS 成品</Button></form>
 </Card>:null}
 </ConsolePage>;
}
