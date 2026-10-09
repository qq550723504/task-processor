"use client";
import {useEffect,useRef,useState} from "react";
import {Button} from "@/components/ui/button";
import {useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {readSupplyRecord,type SupplyScope} from "@/lib/api/supply-chain";
import {fetchProductTitleProposal} from "@/lib/api/product-title-review-client";
import {parseProductTitleProposal,type ProductTitleProposal} from "@/lib/api/product-title-review";
import type {SupplyOperationItem,SupplyTarget} from "@/lib/contracts/supply-chain";
import type {SupplyCommandState} from "./use-supply-command";
import {validateSupplyTitleReview} from "./title-review-binding";
import {TitleReviewDetail} from "../task-center/title-review-detail";
type Entry={item:SupplyOperationItem;record:SupplyTarget;proposal:ProductTitleProposal};
export function BulkSupplyReviewPanel({scope,storeId,items,command,permissions}:{scope:SupplyScope;storeId:string;items:SupplyOperationItem[];command:SupplyCommandState;permissions:readonly string[]}){
 const context=useWorkbenchContext(),[entries,setEntries]=useState<Entry[]>(),[error,setError]=useState(""),[confirmed,setConfirmed]=useState(false),[running,setRunning]=useState(false),[message,setMessage]=useState("");
 const active=useRef(false),live=useRef(true),requestKey=JSON.stringify([scope,storeId,items]);
 useEffect(()=>context.registerOrganizationSwitchGuard(()=>!active.current),[context]);
 useEffect(()=>{live.current=true;const c=new AbortController();
  void Promise.all(items.map(async item=>{if(!item.recordId||!item.resultReference||item.status!=="review")throw new Error("NOT_READY");const [record,proposal]=await Promise.all([readSupplyRecord(scope,item.recordId,c.signal),fetchProductTitleProposal({...scope,proposalId:item.resultReference,signal:c.signal})]);validateSupplyTitleReview(proposal,record,item,scope.userId,storeId);if(proposal.state!=="pending"&&proposal.state!=="accepted"&&proposal.state!=="applied")throw new Error("NOT_READY");return {item,record,proposal};})).then(value=>{if(live.current)setEntries(value)}).catch(()=>{if(live.current)setError("无法核对全部原商品与提案，请刷新后重新选择。");});
  return()=>{live.current=false;c.abort()};
 },[scope,storeId,items,requestKey]);
 const blocked=running||!confirmed||!entries?.length||!permissions.includes("listingkit.admin.write")||command.busy||!!command.pending||!command.ready||!!message;
 async function approve(){if(blocked||active.current||!entries)return;active.current=true;setRunning(true);let completed=0;
  try{for(const entry of entries){if(!live.current)return;if(entry.proposal.state!=="pending")continue;
   const p=entry.proposal,result=await command.execute("review-decision",{proposalId:p.proposal_id,sourceId:entry.item.sourceId,recordId:entry.record.id,productKey:p.input.product_key,baseVersion:p.input.base_version,input:{action:"accept",expected_revision:p.revision}});
   if(!live.current)return;const saved=parseProductTitleProposal(result);
   if(!saved||saved.state!=="accepted"||saved.proposal_id!==p.proposal_id||saved.owner!==scope.userId||saved.input.product_key!==p.input.product_key||saved.input.base_version!==p.input.base_version||BigInt(saved.revision)!==BigInt(p.revision)+BigInt(1)){setMessage(`已批准 ${completed} 件，处理已停止。请先核实原操作，再刷新提案。`);return;}
   completed++;setEntries(prior=>prior?.map(value=>value.proposal.proposal_id===saved.proposal_id?{...value,proposal:saved}:value));
  }setMessage(`已批准 ${completed} 件建议；请分别确认应用并补全目标资料。`);
  }catch{if(live.current)setMessage(`已批准 ${completed} 件，处理已停止。请先核实原操作，再刷新提案。`);}finally{active.current=false;if(live.current)setRunning(false)}
 }
 return <div className="space-y-4"><p className="text-sm">本页所选 {items.length} 件。批准后，请在原审核入口单独确认应用，再保存目标资料。</p>{error?<p role="alert">{error}</p>:!entries?<p role="status">正在核对全部原提案</p>:<>{entries.map(entry=><details open key={entry.item.sourceId} className="rounded-lg border border-border p-4"><summary className="cursor-pointer text-sm font-medium">商品 {entry.record.source.source.productKey}</summary><TitleReviewDetail proposal={entry.proposal}/></details>)}<label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={confirmed} disabled={running||!!message} onChange={e=>setConfirmed(e.target.checked)}/>我已核对以上全部建议</label><Button disabled={blocked} onClick={()=>void approve()}>确认批量审核通过</Button>{!permissions.includes("listingkit.admin.write")?<p className="text-sm">当前企业需要管理员读写权限才能批准。</p>:null}</>}{message?<p role="status">{message}</p>:running?<p role="status">正在逐件批准，请保留当前页面。</p>:null}</div>;
}
