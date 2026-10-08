"use client";
import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { SupplyAPIError, supplyCommand, readSupplyCommand, type SupplyScope, type SupplyIntent } from "@/lib/api/supply-chain";
import { parseSupplyIntent } from "@/lib/api/supply-intent";

export function useSupplyCommand(scope:SupplyScope,onSaved:(result:unknown,intent:SupplyIntent)=>void){
 const context=useWorkbenchContext();
 const pending=context.pendingSupplyIntent;
 const [busy,setBusy]=useState(false);const [error,setError]=useState<string|null>(null);
 const alive=useRef(true);const active=useRef(false);const abort=useRef<AbortController|null>(null);
 const saved=useRef(onSaved);
 useEffect(()=>{saved.current=onSaved},[onSaved]);
 const foreign=!!pending && (pending.userId!==scope.userId||pending.organizationId!==scope.organizationId);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;abort.current?.abort()}},[]);
 useEffect(()=>context.registerOrganizationSwitchGuard(()=>!active.current),[context]);
 async function dispatch(intent:SupplyIntent,verify:boolean){
  if(!alive.current||active.current||intent.userId!==scope.userId||intent.organizationId!==scope.organizationId)return;
  if(!context.setPendingSupplyIntent(intent)){setError("BROWSER_STORAGE_UNAVAILABLE");return}
  setBusy(true);setError(null);active.current=true;
  const controller=new AbortController();abort.current=controller;
  try{
   const result=await(verify?readSupplyCommand(intent,controller.signal):supplyCommand(intent,controller.signal));
   if(!alive.current)return;
   if(!context.setPendingSupplyIntent(null)){setError("BROWSER_STORAGE_UNAVAILABLE");return}
   saved.current(result,intent);
  }catch(failure){
   if(!alive.current)return;
   const code=failure instanceof SupplyAPIError?failure.code:"OUTCOME_UNKNOWN";
   if(!verify && failure instanceof SupplyAPIError && failure.status>=400 && failure.status<500 && code!=="OUTCOME_UNKNOWN"){
    context.setPendingSupplyIntent(null);
   }
   setError(code);
  }finally{active.current=false;if(abort.current===controller)abort.current=null;if(alive.current)setBusy(false)}
 }
 function execute(route:SupplyIntent["route"],command:unknown){
  if(pending||active.current||!context.supplyIntentReady)return;
  const intent=parseSupplyIntent(JSON.stringify({...scope,key:crypto.randomUUID(),route,command}));
  if(!intent){setError("INVALID_REQUEST");return}void dispatch(intent,false);
 }
 return {execute,pending,busy,error,foreign,ready:context.supplyIntentReady,verify:()=>{if(pending)void dispatch(pending,true)},retry:()=>{if(pending)void dispatch(pending,false)}};
}
export type SupplyCommandState=ReturnType<typeof useSupplyCommand>;
