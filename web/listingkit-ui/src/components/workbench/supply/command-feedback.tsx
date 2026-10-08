"use client";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { SupplyCommandState } from "./use-supply-command";

export const supplyFailureText:Record<string,string>={INVALID_REQUEST:"请检查填写内容。",PERMISSION_DENIED:"当前权限不足，请确认原身份与企业权限。",NOT_FOUND:"尚未读取到原操作。",NOT_READY:"资料、图片或当前店铺尚未就绪。",REVISION_CONFLICT:"资料已发生变化，请重新读取后补齐。",OUTCOME_UNKNOWN:"操作结果待核实。",DEPENDENCY_UNAVAILABLE:"当前服务暂时不可用，请稍后重新读取。",ORGANIZATION_CONTEXT_CHANGED:"当前企业已变化，请回到原企业核实。",IDENTITY_CONTEXT_CHANGED:"当前登录身份已变化，请回到原身份核实。",BROWSER_STORAGE_UNAVAILABLE:"浏览器无法保存或清除原操作键，请检查浏览器存储空间后重试。"};
export function SupplyCommandFeedback({state}:{state:SupplyCommandState}){
 return <>{state.pending?<Card className="mb-4 space-y-3 border-amber-200 bg-amber-50 p-4" role="status"><h2 className="font-semibold">结果待核实</h2><p className="break-all text-sm">原操作键：{state.pending.key}</p>{state.foreign?<p>请回到原登录身份与企业后核实。</p>:<div className="flex flex-wrap gap-2"><Button disabled={state.busy} onClick={state.verify}>核实原操作</Button>{state.error==="NOT_FOUND"?<Button variant="outline" disabled={state.busy} onClick={state.retry}>重试原请求</Button>:null}</div>}</Card>:null}{state.error && state.error!=="OUTCOME_UNKNOWN"?<p className="mb-4 text-sm text-red-700" role="alert">{supplyFailureText[state.error]??"当前操作未完成，请保留原操作键。"}</p>:null}</>;
}
