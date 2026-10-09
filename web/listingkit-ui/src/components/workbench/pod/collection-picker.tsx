"use client";
import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {listCollectionBatches,listCollectionItems} from "@/lib/api/product-collection";
import type {MarketScope} from "@/lib/api/supply-market";
import type {CollectionItem} from "@/lib/contracts/product-collection";
import {Button} from "@/components/ui/button";
import {ConsoleState} from "@/components/workbench/console/console-page";
export function CollectionPicker({scope,kind,onSelect,disabled=false}:{scope:MarketScope;kind:"template"|"artwork";onSelect:(item:CollectionItem)=>void;disabled?:boolean}){
 const [batch,setBatch]=useState(""),[batchesAfter,setBatchesAfter]=useState<string>(),[itemsAfter,setItemsAfter]=useState<string>();
 const key=["pod",scope.userId,scope.organizationId,"collections"];
 const batches=useQuery({queryKey:[...key,"batches",batchesAfter],queryFn:({signal})=>listCollectionBatches(scope,{after:batchesAfter,limit:50},signal)});
 const items=useQuery({queryKey:[...key,"items",batch,itemsAfter],queryFn:({signal})=>listCollectionItems(scope,batch,{after:itemsAfter,limit:50},signal),enabled:!!batch});
 const available=items.data?.items.filter(i=>!i.archivedAt&&(kind==="template"?i.source.kind==="sds_template":i.source.kind!=="sds_template"));
 return <div className="space-y-3"><label className="block">从我的数据选择批次<select className="mt-1 block w-full rounded-md border bg-background p-2" aria-label="我的数据批次" disabled={disabled||batches.isPending} value={batch} onChange={e=>{setBatch(e.target.value);setItemsAfter(undefined)}}><option value="">请选择批次</option>{batches.data?.items.map(b=><option key={b.id} value={b.id}>{b.name}（{b.count} 项）</option>)}</select></label>
 {batches.error?<ConsoleState kind="error" title="无法读取我的数据"><Button variant="outline" onClick={()=>void batches.refetch()}>重新读取</Button></ConsoleState>:null}
 <div className="flex gap-2">{batchesAfter?<Button variant="outline" disabled={disabled} onClick={()=>{setBatchesAfter(undefined);setBatch("")}}>批次首页</Button>:null}{batches.data?.nextCursor?<Button variant="outline" disabled={disabled} onClick={()=>{setBatchesAfter(batches.data!.nextCursor);setBatch("")}}>下一页批次</Button>:null}</div>
 {batch&&items.isPending?<p role="status">正在读取商品…</p>:null}{items.error?<ConsoleState kind="error" title="无法读取批次商品"><Button variant="outline" onClick={()=>void items.refetch()}>重新读取</Button></ConsoleState>:null}
 {available?.length===0?<p>当前页没有{kind==="template"?"已保存的 SDS 模板":"可用图案商品"}。</p>:null}
 <div className="grid gap-2 sm:grid-cols-2">{available?.map(i=><Button variant="outline" className="h-auto justify-start whitespace-normal p-3 text-left" key={i.id} disabled={disabled} onClick={()=>onSelect(i)}>{i.title||i.source.productKey}</Button>)}</div>
 <div className="flex gap-2">{itemsAfter?<Button variant="outline" disabled={disabled} onClick={()=>setItemsAfter(undefined)}>商品首页</Button>:null}{items.data?.nextCursor?<Button variant="outline" disabled={disabled} onClick={()=>setItemsAfter(items.data!.nextCursor)}>下一页商品</Button>:null}</div>
 </div>;
}
