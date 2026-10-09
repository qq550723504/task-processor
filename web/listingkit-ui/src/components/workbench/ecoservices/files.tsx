"use client";
import {useEffect,useRef,useState} from "react";
import {Button} from "@/components/ui/button";
import {ecoFileSchema,type EcoScope,EcoservicesError} from "@/lib/api/ecoservices";
import {errorText,type EcoCommands} from "./shared";
export function FileUpload({commands,path,ids,onFiles,maxBytes=10*1024*1024,maxFiles=10,accept=".pdf,.png,.jpg,.jpeg,.txt"}:{commands:EcoCommands;path:string;ids:string[];onFiles:(ids:string[])=>void;maxBytes?:number;maxFiles?:number;accept?:string}){
 const resolved=commands.resolved;
 const handled=useRef(resolved?.intent.key);
 useEffect(()=>{if(resolved?.intent.output!=="file"||resolved.intent.path!==path||handled.current===resolved.intent.key)return;handled.current=resolved.intent.key;const file=ecoFileSchema.safeParse(resolved.data);if(file.success&&!ids.includes(file.data.id))onFiles([...ids,file.data.id])},[resolved,path,ids,onFiles]);
 return <label>私有材料（最多{maxFiles}份，每份{maxBytes/(1024*1024)} MiB）<input type="file" accept={accept} disabled={commands.locked||ids.length>=maxFiles} onChange={e=>{const file=e.target.files?.[0];e.target.value="";if(!file)return;if(file.size>maxBytes){commands.setMessage("每份材料不能超过"+maxBytes/(1024*1024)+" MiB");return}const form=new FormData();form.set("file",file);void commands.execute({path,key:crypto.randomUUID(),body:form,output:"file"}).catch(()=>undefined)}}/><small>已确认保存 {ids.length} 份材料</small></label>;
}
export function FileLinks({scope,ids,application=false,admin=false}:{scope:EcoScope;ids:string[];application?:boolean;admin?:boolean}){
 const [pending,setPending]=useState(false),[error,setError]=useState("");
 async function download(id:string){setPending(true);setError("");try{const headers=new Headers({"X-Expected-User-ID":scope.userId});if(!admin)headers.set("X-Expected-Organization-ID",scope.organizationId);const r=await fetch((admin?"/api/admin/ecoservices/":"/api/ecoservices/")+(application&&!admin?"applications/":"")+"files/"+id,{headers,cache:"no-store",credentials:"same-origin"});if(!r.ok)throw new EcoservicesError(r.status===403?"ECOSERVICES_FORBIDDEN":"ECOSERVICES_UNAVAILABLE",r.status);const blob=await r.blob();if(blob.size>10*1024*1024)throw new Error("材料大小超过限制");const url=URL.createObjectURL(blob),a=document.createElement("a");a.href=url;a.download="服务材料-"+id;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}catch(e){setError(errorText(e))}finally{setPending(false)}}
 return <div className="eco-actions">{ids.map((id,i)=><Button key={id} variant="outline" disabled={pending} onClick={()=>void download(id)}>下载材料 {i+1}</Button>)}{error?<p role="alert">{error}</p>:null}</div>;
}
