"use client";
import {useEffect,useState} from "react";
import {Button} from "@/components/ui/button";
import {fetchProductTitleProposal} from "@/lib/api/product-title-review-client";
import type {ProductTitleProposal} from "@/lib/api/product-title-review";
import {readSupplyRecord,type SupplyScope} from "@/lib/api/supply-chain";
import type {SupplyOperationItem,SupplyTarget} from "@/lib/contracts/supply-chain";
import {TitleReviewControls,TitleReviewDetail} from "../task-center/title-review-detail";
import type {SupplyCommandState} from "./use-supply-command";
import {validateSupplyTitleReview} from "./title-review-binding";
export function SupplyReviewPanel({scope,storeId,item,command,saved,permissions,roles,onApplied}:{scope:SupplyScope;storeId:string;item:SupplyOperationItem;command:SupplyCommandState;saved:number;permissions:readonly string[];roles:readonly string[];onApplied:(p:ProductTitleProposal,r:SupplyTarget)=>void}){
 const [proposal,setProposal]=useState<ProductTitleProposal>(),[record,setRecord]=useState<SupplyTarget>(),[error,setError]=useState<string>(),[refresh,setRefresh]=useState(0),[loaded,setLoaded]=useState("");
 const key=JSON.stringify([scope,item,storeId,saved,refresh]);
 useEffect(()=>{let live=true;const c=new AbortController();
  if(!item.recordId||!item.resultReference)return;
  void Promise.all([readSupplyRecord(scope,item.recordId,c.signal),fetchProductTitleProposal({...scope,proposalId:item.resultReference,signal:c.signal})]).then(([r,p])=>{validateSupplyTitleReview(p,r,item,scope.userId,storeId);if(live){setRecord(r);setProposal(p);setError(undefined);setLoaded(key)}}).catch(()=>{if(live){setError("无法核对原商品与审核提案，请刷新原操作。");setLoaded(key)}});return()=>{live=false;c.abort()};
 },[scope,storeId,item,saved,refresh,key]);
 const blocked=loaded!==key||command.busy||!!command.pending||!command.ready;
 const binding=proposal&&record?{proposalId:proposal.proposal_id,sourceId:item.sourceId,recordId:record.id,productKey:proposal.input.product_key,baseVersion:proposal.input.base_version}:undefined;
 return <div className="space-y-4">{!item.recordId||!item.resultReference?<p role="alert">原操作缺少审核绑定</p>:loaded!==key?<p role="status">正在读取原审核提案</p>:error?<p role="alert">{error}</p>:!proposal?<p role="status">正在读取原审核提案</p>:<><TitleReviewDetail proposal={proposal}/><TitleReviewControls key={`${proposal.proposal_id}:${proposal.revision}:${proposal.state}`} proposal={proposal} roles={roles} permissions={permissions} userId={scope.userId} pending={blocked} decide={(action,title)=>command.execute("review-decision",{...binding,input:{action,expected_revision:proposal.revision,...action==="edit"?{title}:{}}})} apply={()=>command.execute("review-apply",{...binding,input:{expected_revision:proposal.revision}})}/>{proposal.state==="applied"&&record?<Button disabled={blocked} onClick={()=>onApplied(proposal,record)}>使用已应用标题，重新补全平台资料</Button>:null}</>}<Button variant="outline" disabled={command.busy} onClick={()=>setRefresh(n=>n+1)}>刷新原提案</Button></div>;
}
