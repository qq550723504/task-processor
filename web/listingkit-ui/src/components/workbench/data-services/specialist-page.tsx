"use client";
import { useEffect, useMemo, useState } from "react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { CollectionDialog } from "@/components/workbench/collections/collection-page";
import { dataRequest, DataAPIError, encodeDataFile } from "@/lib/api/data-services";
import { adminCustomSchema, adminSummarySchema, adminPageSchema, customSpecSchema, type CustomRequest } from "@/lib/contracts/data-services";
import { CustomDetails, Field, DataNotice, stateNames, moment } from "./data-services-page";
import { useDataCommand } from "./use-data-command";
export function SpecialistPage({ userId }: {
    userId: string;
}) {
    const scope = useMemo(() => ({ userId, organizationId: "platform" }), [userId]);
    const [requests, setRequests] = useState<z.infer<typeof adminSummarySchema>[]>([]), [selected, setSelected] = useState<z.infer<typeof adminCustomSchema> | null>(null), [state, setState] = useState(""), [cursor, setCursor] = useState(""), [next, setNext] = useState(""), [error, setError] = useState(""), [refresh, setRefresh] = useState(0);
    const command = useDataCommand(scope, true);
    useEffect(() => {
        const controller = new AbortController();
        const query = new URLSearchParams();
        if (state)
            query.set("state", state);
        if (cursor)
            query.set("cursor", cursor);
        dataRequest(scope, query.size ? `?${query}` : "", adminPageSchema, undefined, controller.signal, true).then(p => { setRequests(p.items); setNext(p.nextCursor ?? ""); }).catch(e => {
            if (!controller.signal.aborted)
                setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
        });
        return () => controller.abort();
    }, [scope, state, cursor, refresh]);
    function received(value: unknown) {
        const parsed = adminCustomSchema.safeParse(value);
        if (parsed.success)
            setSelected(parsed.data);
        setRefresh(n => n + 1);
    }
    return <main className="mx-auto max-w-6xl space-y-6 p-5 md:p-8"><header className="flex flex-wrap items-center justify-between gap-4"><div><h1 className="text-2xl font-semibold">数据集定制工作台</h1><p className="mt-2 text-sm text-muted-foreground">记录线下规格与报价确认、制作进度，并向原申请人交付数据。</p></div><Button variant="outline" onClick={() => setRefresh(n => n + 1)}>刷新</Button></header><DataNotice error={error || command.error}/>{command.pending ? <Card className="gap-3 border-amber-200 bg-amber-50 p-4"><p className="text-sm">原操作 {command.pending.key} 待核实，继续时保留原文件及条件。</p><Button disabled={command.busy} variant="outline" onClick={async () => {
                const value = await command.recover();
                if (value !== undefined)
                    received(value);
            }}>核实／继续原操作</Button></Card> : null}
  <Card className="gap-4 p-6"><Field title="进度筛选"><Select value={state} onChange={e => { setState(e.target.value); setCursor(""); }}><option value="">全部进度</option>{["SUBMITTED", "EVALUATING", "SPEC_CONFIRMED", "PREPARING", "DELIVERED", "CLOSED"].map(s => <option key={s} value={s}>{stateNames[s]}</option>)}</Select></Field><div className="space-y-3">{requests.map(r => <button className="flex w-full flex-wrap justify-between gap-3 rounded-lg border p-4 text-left hover:bg-muted" key={r.id} onClick={async () => {
                try {
                    setSelected(await dataRequest(scope, r.id, adminCustomSchema, undefined, undefined, true));
                }
                catch (e) {
                    setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
                }
            }}><span><strong>{r.name}</strong><p className="mt-1 text-xs text-muted-foreground">申请编号 {r.id} · {moment(r.createdAt)}</p></span><span className="text-sm">{stateNames[r.state]}</span></button>)}{!requests.length && !error ? <p className="py-6 text-center text-sm text-muted-foreground">暂无符合条件的申请。</p> : null}</div><div className="flex gap-3">{cursor ? <Button variant="outline" onClick={() => setCursor("")}>返回首页</Button> : null}{next ? <Button variant="outline" onClick={() => setCursor(next)}>下一页</Button> : null}</div></Card>
  {selected ? <CollectionDialog title={selected.input.name} onClose={() => setSelected(null)}><p className="text-sm">原企业：{selected.applicant.organizationId} · 原创建者：{selected.applicant.actorId}</p><CustomDetails request={selected}/><p className="rounded-lg bg-muted p-3 text-sm">交付企业与创建者由原申请绑定，上传不能改变归属。交付记录和数据批次一起保存，最多 200 条，文件不超过 2 MiB。</p>{!['DELIVERED', 'CLOSED'].includes(selected.state) ? <SpecialistActions key={`${selected.id}:${selected.revision}`} request={selected} disabled={command.busy || !!command.pending} onChange={async (patch) => {
                    const value = await command.run(`${selected.id}/changes`, { expectedRevision: selected.revision, patch });
                    if (value !== undefined)
                        received(value);
                }} onDeliver={async (file) => {
                    if (file.size < 1 || file.size > 2 * 1024 * 1024) {
                        setError("INVALID_DATA_REQUEST");
                        return;
                    }
                    ;
                    const format = selected.spec!.format;
                    if (!file.name.toLowerCase().endsWith(`.${format}`)) {
                        setError("INVALID_DATA_REQUEST");
                        return;
                    }
                    ;
                    const bytes = new Uint8Array(await file.arrayBuffer());
                    const value = await command.run(`${selected.id}/delivery`, undefined, encodeDataFile(bytes), { "X-Data-Format": format, "X-Expected-Revision": String(selected.revision), "X-Spec-Revision": String(selected.specRevision) });
                    if (value !== undefined)
                        received(value);
                }}/> : null}<Button variant="outline" onClick={async () => {
                try {
                    setSelected(await dataRequest(scope, selected.id, adminCustomSchema, undefined, undefined, true));
                }
                catch (e) {
                    setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
                }
            }}>刷新申请</Button></CollectionDialog> : null}
 </main>;
}
function SpecialistActions({ request, disabled, onChange, onDeliver }: {
    request: CustomRequest;
    disabled: boolean;
    onChange: (patch: unknown) => void;
    onDeliver: (file: File) => void;
}) {
    const [state, setState] = useState(request.state === "SUBMITTED" ? "EVALUATING" : request.state === "EVALUATING" ? "SPEC_CONFIRMED" : request.state === "SPEC_CONFIRMED" ? "PREPARING" : "PREPARING"), [note, setNote] = useState(""), [description, setDescription] = useState(request.spec?.description ?? ""), [quote, setQuote] = useState(request.spec?.quoteNote ?? ""), [confirmation, setConfirmation] = useState(request.spec?.confirmationNote ?? ""), [format, setFormat] = useState(request.spec?.format ?? request.input.format), [rows, setRows] = useState(String(request.spec?.maximumRows ?? request.input.query.limit)), [file, setFile] = useState<File | null>(null), [error, setError] = useState("");
    return <div className="space-y-5 border-t pt-5"><form className="space-y-4" onSubmit={e => {
            e.preventDefault();
            let spec: z.infer<typeof customSpecSchema> | undefined;
            if (state === "SPEC_CONFIRMED") {
                const parsed = customSpecSchema.safeParse({ description, quoteNote: quote, confirmationNote: confirmation, format, maximumRows: Number(rows) });
                if (!parsed.success) {
                    setError("INVALID_DATA_REQUEST");
                    return;
                }
                ;
                spec = parsed.data;
            }
            onChange({ state, note, ...(spec ? { spec } : {}) });
        }}><Field title="记录下一步进度"><Select value={state} onChange={e => setState(e.target.value)}>{(request.state === "SUBMITTED" ? ["EVALUATING", "CLOSED"] : request.state === "EVALUATING" ? ["EVALUATING", "SPEC_CONFIRMED", "CLOSED"] : request.state === "SPEC_CONFIRMED" ? ["SPEC_CONFIRMED", "PREPARING", "CLOSED"] : ["PREPARING", "CLOSED"]).map(s => <option value={s} key={s}>{stateNames[s]}</option>)}</Select></Field><Field title="进度说明"><Textarea required maxLength={4000} value={note} onChange={e => setNote(e.target.value)}/></Field>{state === "SPEC_CONFIRMED" ? <><Field title="已确认的规格"><Textarea required maxLength={8000} value={description} onChange={e => setDescription(e.target.value)}/></Field><Field title="线下报价记录"><Textarea required maxLength={4000} value={quote} onChange={e => setQuote(e.target.value)}/></Field><Field title="线下确认记录"><Textarea required maxLength={4000} value={confirmation} onChange={e => setConfirmation(e.target.value)}/></Field><div className="grid gap-4 sm:grid-cols-2"><Field title="交付格式"><Select value={format} onChange={e => setFormat(e.target.value as typeof format)}>{["csv", "json", "xlsx"].map(f => <option key={f}>{f}</option>)}</Select></Field><Field title="确认最大条数"><Input required type="number" min={1} max={200} value={rows} onChange={e => setRows(e.target.value)}/></Field></div></> : null}<DataNotice error={error}/><Button disabled={disabled} type="submit">保存进度与确认记录</Button></form>
  {request.state === "PREPARING" && request.spec ? <form className="space-y-4 rounded-lg border p-4" onSubmit={e => {
                e.preventDefault();
                if (file)
                    onDeliver(file);
            }}><h3 className="font-semibold">交付数据文件</h3><p className="text-sm">规格版本 {request.specRevision} · {request.spec.format} · 最多 {request.spec.maximumRows} 条</p><p className="text-xs text-muted-foreground">CSV/XLSX 必须使用英文表头，依次为 title、description、brand、images、sku、currency、price、stock。JSON 使用商品对象数组（title、description、brand、images、variants、attributes）；内容必须与确认规格一致。</p><Field title="选择文件"><Input required type="file" accept={`.${request.spec.format}`} onChange={e => setFile(e.target.files?.[0] ?? null)}/></Field><Button type="submit" disabled={disabled || !file}>确认交付到原申请人的我的数据</Button></form> : null}
 </div>;
}
