"use client";
import {useEffect,useRef,useState} from "react";
import {z} from "zod";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {CollectionAPIError,uploadSourceImage,readSourceImage,type CollectionScope,type MediaIntent} from "@/lib/api/product-collection";
import {mediaHashSchema,type mediaImageSchema} from "@/lib/contracts/product-collection";
const key="shuomi_source_media_intent";
const pendingSchema=z.object({userId:z.string().min(1).max(128),organizationId:z.string().min(1).max(128),hash:mediaHashSchema,bytes:z.number().int().positive().max(3*1024*1024)}).strict();
function load():MediaIntent|null{try{if(typeof window==="undefined")return null;const raw=localStorage.getItem(key);return raw?pendingSchema.parse(JSON.parse(raw)):null;}catch{return null;}}
export function SourceImageUploader({scope,disabled,onImage,onBlocked,onMedia}:{scope:CollectionScope;disabled:boolean;onImage:(url:string)=>void;onBlocked:(v:boolean)=>void;onMedia?:(image:z.infer<typeof mediaImageSchema>)=>void}){
 const context=useWorkbenchContext(),flight=useRef(false),abort=useRef<AbortController|null>(null),raw=useRef<ArrayBuffer|null>(null);
 const [pending,setPending]=useState<MediaIntent|null>(load),[selected,setSelected]=useState<MediaIntent|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(""),[missing,setMissing]=useState(false),[fileName,setFileName]=useState("");
 const foreign=!!pending&&(pending.userId!==scope.userId||pending.organizationId!==scope.organizationId);
 useEffect(()=>context.registerOrganizationSwitchGuard(()=>!flight.current),[context]);useEffect(()=>()=>abort.current?.abort(),[]);
 useEffect(()=>{onBlocked(busy||!!pending);},[busy,pending,onBlocked]);
 async function choose(file:File){
  if(flight.current||disabled||foreign)return;const controller=new AbortController();abort.current=controller;flight.current=true;setBusy(true);setError("");raw.current=null;setSelected(null);
  try{
   if(file.size<1||file.size>3*1024*1024)throw new Error("每张图片最多 3 MB。");
   const bytes=await file.arrayBuffer();const digest=await crypto.subtle.digest("SHA-256",bytes);const hash=Array.from(new Uint8Array(digest),v=>v.toString(16).padStart(2,"0")).join("");
   if(controller.signal.aborted)return;
   const intent={...scope,hash,bytes:bytes.byteLength};if(pending&&(hash!==pending.hash||intent.bytes!==pending.bytes))throw new Error("待核实的操作只允许重新选择原文件。");
   raw.current=bytes;setSelected(intent);setFileName(file.name);
  }catch(e){if(!controller.signal.aborted)setError(e instanceof Error?e.message:"无法读取图片。");}
  finally{flight.current=false;if(!controller.signal.aborted)setBusy(false);}
 }
 async function run(verify:boolean){
  const intent=pending??selected;if(!intent||flight.current||disabled||foreign||!verify&&!raw.current)return;
  const controller=new AbortController();abort.current=controller;flight.current=true;setBusy(true);setError("");
  try{
   if(!verify){localStorage.setItem(key,JSON.stringify(intent));setPending(intent);}
   const result=verify?await readSourceImage(intent,controller.signal):await uploadSourceImage(intent,raw.current!,controller.signal);
   if(controller.signal.aborted)return;
   localStorage.removeItem(key);setPending(null);setSelected(null);raw.current=null;setFileName("");setMissing(false);onImage(result.url);onMedia?.(result);
  }catch(e){
   if(controller.signal.aborted)return;
   if(e instanceof CollectionAPIError){setMissing(verify&&e.code==="NOT_FOUND");if(!verify&&e.status>=400&&e.status<500&&e.code!=="OUTCOME_UNKNOWN"){localStorage.removeItem(key);setPending(null);}
    setError(verify&&e.code==="NOT_FOUND"?"未发现原图片，可重新选择同一文件后重试。":e.code==="INVALID_REQUEST"?"请选择有效 JPEG 或 PNG 图片，每张最多 3 MB。":"上传结果尚未确认，请核实原图片。");
   }else{setError("无法保存上传意图，请检查浏览器存储是否可用。");}
  }finally{flight.current=false;if(!controller.signal.aborted)setBusy(false);}
 }
 return <div className="space-y-3 rounded-lg border border-slate-200 bg-slate-50 p-4"><p className="text-sm text-slate-600">上传 JPEG / PNG 文件后会生成可公开访问的商品图片链接。每张最多 3 MB；图片仍需在适配时确认。</p>
 {foreign?<p role="alert">原身份与企业下有待核实的图片，请切回后处理。</p>:<><label className="grid gap-2 text-sm">图片文件<Input type="file" accept="image/jpeg,image/png" disabled={disabled||busy||!!pending&&!missing} onChange={e=>{const file=e.target.files?.[0];if(file)void choose(file);e.target.value="";}}/></label>{fileName?<p className="break-all text-xs">{fileName}</p>:null}
 {pending?<div className="space-y-2"><p className="text-xs">原图片待核实：{pending.hash.slice(0,16)}</p><div className="flex gap-2"><Button type="button" size="sm" disabled={disabled||busy} onClick={()=>void run(true)}>核实原图片</Button>{missing&&selected?<Button type="button" size="sm" variant="outline" disabled={disabled||busy} onClick={()=>void run(false)}>重试原文件</Button>:null}</div></div>:<Button type="button" size="sm" disabled={disabled||busy||!selected} onClick={()=>void run(false)}>上传并添加图片链接</Button>}</>}
 {busy?<p role="status" className="text-sm">正在处理图片…</p>:null}{error?<p role="alert" className="text-sm text-red-700">{error}</p>:null}</div>;
}
