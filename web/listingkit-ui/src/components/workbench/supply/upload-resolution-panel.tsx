"use client";
import {useEffect,useState} from "react";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {readSupplyUploadAttempt,type SupplyScope} from "@/lib/api/supply-chain";
import type {SupplyOperationItem,uploadAttemptSchema} from "@/lib/contracts/supply-chain";
import type {z} from "zod";
import type {SupplyCommandState} from "./use-supply-command";
export function UploadResolutionPanel({scope,item,command,disabled}:{scope:SupplyScope;item:SupplyOperationItem;command:SupplyCommandState;disabled:boolean}){
 const [view,setView]=useState<z.infer<typeof uploadAttemptSchema>>(),[spu,setSPU]=useState(""),[error,setError]=useState(false);
 useEffect(()=>{const c=new AbortController();void readSupplyUploadAttempt(scope,item.recordId!,item.resultReference!,c.signal).then(v=>{if(!c.signal.aborted)setView(v)}).catch(()=>{if(!c.signal.aborted)setError(true)});return()=>c.abort()},[scope,item]);
 return <div className="space-y-4"><p className="break-all text-xs text-slate-500">原操作 {item.resultReference}</p>{view?<><p className="text-sm text-slate-600">{view.message}</p>{view.effectKind==="publish"?<><p className="text-sm text-slate-500">官方查询只返回审核通过的商品；未找到或资料不匹配时继续保持待核实。</p><label className="grid gap-2 text-sm">原商品 SPU<Input value={spu} onChange={e=>setSPU(e.target.value)} maxLength={128}/></label><Button disabled={disabled||!spu.trim()} onClick={()=>command.execute("resolve-upload",{recordId:item.recordId,attemptId:item.resultReference,spu:spu.trim()})}>从官方查询并核实</Button></>:null}</>:error?<p role="alert">暂时无法读取原操作，请刷新后重试。</p>:<p role="status">正在读取原操作…</p>}</div>;
}
