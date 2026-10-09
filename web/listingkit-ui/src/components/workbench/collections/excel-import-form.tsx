"use client";
import {useEffect,useRef,useState} from "react";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {CollectionAPIError,previewCollectionImport,readImportTemplate,type CollectionScope} from "@/lib/api/product-collection";
import {importPreviewSchema,type CollectionCommand} from "@/lib/contracts/product-collection";
type Preview=ReturnType<typeof importPreviewSchema.parse>;
export function ExcelImportForm({scope,disabled,onSubmit}:{scope:CollectionScope;disabled:boolean;onSubmit:(v:CollectionCommand)=>void}){
 const context=useWorkbenchContext(),flight=useRef(false),abort=useRef<AbortController|null>(null);
 const [preview,setPreview]=useState<Preview|null>(null),[name,setName]=useState("Excel 商品批次"),[busy,setBusy]=useState(false),[error,setError]=useState("");
 useEffect(()=>context.registerOrganizationSwitchGuard(()=>!flight.current),[context]);
 useEffect(()=>()=>abort.current?.abort(),[]);
 async function run(file?:File){
  if(flight.current||disabled)return;const controller=new AbortController();abort.current=controller;flight.current=true;setBusy(true);setError("");setPreview(null);
  try{
   if(file){const data=await previewCollectionImport(scope,file,controller.signal);if(!controller.signal.aborted){setPreview(data);setName(file.name.replace(/\.xlsx$/i,"").slice(0,60)||"Excel 商品批次");}}
   else{const data=await readImportTemplate(scope,controller.signal);if(controller.signal.aborted)return;const bytes=Uint8Array.from(atob(data.content),v=>v.charCodeAt(0));const url=URL.createObjectURL(new Blob([bytes]));const a=document.createElement("a");a.href=url;a.download="自有商品模板.xlsx";a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
  }catch(e){if(!controller.signal.aborted)setError(e instanceof CollectionAPIError&&e.code==="INVALID_REQUEST"?"文件不符合模板。请检查全部行、必填 SKU 资料及图片链接；公式、宏和外部链接不支持。":"暂时无法读取，请重试。");}
  finally{flight.current=false;if(!controller.signal.aborted)setBusy(false);}
 }
 return <div className="space-y-4"><p className="text-sm text-slate-500">使用模板上传 XLSX，最多 200 件商品、2 MB。图片链接用换行或 | 分隔；填写 SKU 时须同时填写币种、价格和库存。全部行通过后才可保存。</p><Button variant="outline" disabled={disabled||busy} onClick={()=>void run()}>下载 Excel 模板</Button><label className="grid gap-2">选择 Excel 文件<Input type="file" accept=".xlsx" disabled={disabled||busy} onChange={e=>{const file=e.target.files?.[0];if(file)void run(file);e.target.value="";}}/></label>{busy?<p role="status">正在读取全部商品…</p>:null}{error?<p role="alert" className="text-sm text-red-700">{error}</p>:null}
 {preview?<><label className="grid gap-2">批次名称<Input value={name} onChange={e=>setName(e.target.value)} maxLength={60}/></label><p>已校验 {preview.products.length} 件商品</p><div className="max-h-72 overflow-auto rounded-lg border border-slate-200"><table className="w-full text-sm"><thead className="bg-slate-50"><tr><th className="p-2 text-left">商品名称</th><th>SKU</th><th>图片</th></tr></thead><tbody>{preview.products.map((p,i)=><tr key={i} className="border-t border-slate-100"><td className="p-2">{p.title}</td><td>{p.variants?.[0]?.sku||"未提供"}</td><td>{p.images.length}</td></tr>)}</tbody></table></div><Button disabled={disabled||busy||!name.trim()} onClick={()=>onSubmit({action:"import_products",name:name.trim(),products:preview.products})}>确认保存整个批次</Button></>:null}</div>;
}
