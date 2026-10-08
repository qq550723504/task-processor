"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import Image from "next/image";
import Link from "next/link";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Table, TableHeader, TableHead, TableBody, TableRow, TableCell } from "@/components/ui/table";
import { CollectionAPIError, listCollectionBatches, listCollectionItems, listOwnProducts, readCollectionItem, mutateCollection, readCollectionOperation, type CollectionScope, type CollectionIntent } from "@/lib/api/product-collection";
import { ownProductSchema, type CollectionBatch, type CollectionItem, type CollectionDetail, type CollectionCommand } from "@/lib/contracts/product-collection";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";
import { useSupplyCommand } from "../supply/use-supply-command";
import { SupplyCommandFeedback } from "../supply/command-feedback";
import { transferReceiptSchema } from "@/lib/contracts/supply-chain";
import { ConsolePage, ConsoleState } from "../console/console-page";

const kindLabels = { acquisition: "在线采集", own: "自有商品", manual: "手动分组" };
const failureText: Record<string, string> = { OUTCOME_UNKNOWN: "结果待核实", REVISION_CONFLICT: "资料已发生变化，请刷新后重试。", PERMISSION_DENIED: "当前权限不足。", NOT_FOUND: "未找到当前身份下的记录。", ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化。", IDENTITY_CONTEXT_CHANGED: "登录身份已变化。", INVALID_REQUEST: "请检查填写内容。" };

export function CollectionPage({supplyAvailable=false}:{supplyAvailable?:boolean}) {
  const context = useWorkbenchContext();
  const userId = context.user?.id;
  const organizationId = context.effectiveOrganization?.id;
  const scope = useMemo(() => userId && organizationId ? { userId, organizationId } : null, [userId, organizationId]);
  if (context.isLoading || context.isSwitching) return <ConsoleState kind="loading" title="正在确认当前企业" />;
  if (!scope || context.selectionRequired || context.error || context.blockingError) return <ConsoleState kind="error" title="请先确认登录身份与当前企业" />;
  return <ScopedCollectionPage key={`${scope.userId}:${scope.organizationId}`} scope={scope} supplyAvailable={supplyAvailable} />;
}

function ScopedCollectionPage({ scope,supplyAvailable }: { scope: CollectionScope;supplyAvailable:boolean }) {
  const context = useWorkbenchContext();
  const [tab, setTab] = useState<"batches" | "own">("batches");
  const [batches, setBatches] = useState<CollectionBatch[]>([]);
  const [items, setItems] = useState<CollectionItem[]>([]);
  const [batchId, setBatchId] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [next, setNext] = useState<string>();
  const [after, setAfter] = useState<string>();
  const [search, setSearch] = useState("");
  const [keyword, setKeyword] = useState("");
  const [reload, setReload] = useState(0);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState("");
  const [transferred,setTransferred]=useState<string|null>(null);
  const [dialog, setDialog] = useState<"manage" | "own" | "rename" | "archive-batch" | "move" | "archive-item" | "detail" | "transfer" | null>(null);
  const [selectedBatch, setSelectedBatch] = useState<CollectionBatch | null>(null);
  const [selectedItem, setSelectedItem] = useState<CollectionItem | null>(null);
  const [detail, setDetail] = useState<CollectionDetail | null>(null);
  const [name, setName] = useState("");
  const [targetBatch, setTargetBatch] = useState("");
  const [pending, setPending] = useState<CollectionIntent | null>(() => context.pendingCollectionIntent);
  const alive = useRef(true);
  const inFlight = useRef(false);
  const commandAbort = useRef<AbortController | null>(null);
  const canManage = context.permissions.includes("workbench.collection.manage");
  const foreignPending = !!pending && (pending.userId !== scope.userId || pending.organizationId !== scope.organizationId);
  const supply=useSupplyCommand(scope,(result,intent)=>{
    if(intent.route!=="transfer"){setNotice("已核实原供应链操作");return;}
    const receipt=transferReceiptSchema.parse(result);setTransferred(receipt.preparation.id);setDialog(null);setNotice(`已加入我的供应链，共 ${receipt.preparation.count} 件商品。`);
  });
  const transferBlocked=busy||!!pending||supply.busy||!!supply.pending||!supply.ready||!context.permissions.includes("workbench.supply.manage");
  const writeBlocked = busy || !!pending || supply.busy || !!supply.pending || !canManage;

  useEffect(() => { alive.current = true; return () => { alive.current = false; commandAbort.current?.abort(); }; }, []);
  useEffect(() => context.registerOrganizationSwitchGuard(() => !inFlight.current), [context]);
  useEffect(() => {
    const controller = new AbortController();
    let current = true;
    const query = { keyword, after, limit: 50 };
    const request = tab === "own" ? listOwnProducts(scope, query, controller.signal) : batchId ? listCollectionItems(scope, batchId, query, controller.signal) : listCollectionBatches(scope, query, controller.signal);
    void request.then(page => {
      if (!current) return;
      if (tab === "batches" && !batchId) setBatches(page.items as CollectionBatch[]);
      else setItems(page.items as CollectionItem[]);
      setTotal(page.total); setNext(page.nextCursor); setError(null);
    }).catch((failure: unknown) => { if (current) setError(codeOf(failure)); }).finally(() => { if (current) setLoading(false); });
    return () => { current = false; controller.abort(); };
  }, [scope, tab, batchId, keyword, after, reload]);

  function refresh() { setLoading(true); setReload(value => value + 1); }
  function changeView(nextTab: "batches" | "own", selected: string | null = null) { setTab(nextTab); setBatchId(selected); setAfter(undefined); setKeyword(""); setSearch(""); setLoading(true); setItems([]); setError(null); }
  async function command(input: CollectionCommand, original?: CollectionIntent, verify = false) {
    if (!alive.current || inFlight.current || foreignPending || pending && !original) return;
    const intent = original ?? { ...scope, key: crypto.randomUUID(), command: input };
    const controller = new AbortController(); commandAbort.current = controller;
    inFlight.current = true; setBusy(true); setError(null); setPending(intent); context.setPendingCollectionIntent(intent);
    try {
      const receipt = await (verify ? readCollectionOperation(intent, controller.signal) : mutateCollection(intent, controller.signal));
      if (!alive.current) return;
      setPending(null); context.setPendingCollectionIntent(null); setDialog(null); setNotice(`已保存 · 操作 ${receipt.operationId}`); refresh();
    } catch (failure) {
      if (!alive.current) return;
      const code = codeOf(failure);
      if (!verify && failure instanceof CollectionAPIError && failure.status >= 400 && failure.status < 500 && code !== "OUTCOME_UNKNOWN") {
        setPending(null); context.setPendingCollectionIntent(null); setError(code);
      } else { setError(verify ? code : "OUTCOME_UNKNOWN"); }
    } finally { inFlight.current = false; if (commandAbort.current === controller) commandAbort.current = null; if (alive.current) setBusy(false); }
  }
  async function showDetail(item: CollectionItem) {
    if (inFlight.current) return;
    const controller = new AbortController(); commandAbort.current = controller;
    setSelectedItem(item); setDetail(null); setDialog("detail"); setBusy(true); inFlight.current = true; setError(null);
    try { const value = await readCollectionItem(scope, item.id, controller.signal); if (alive.current) setDetail(value); }
    catch (failure) { if (alive.current) setError(codeOf(failure)); }
    finally { inFlight.current = false; if (alive.current) setBusy(false); }
  }
  const displayingItems = tab === "own" || !!batchId;
  return <ConsolePage title="我的数据" breadcrumbs={findConsoleRoute("/workbench/data/mine")?.trail}>
    <div className="mb-5 rounded-xl border border-blue-100 bg-blue-50/70 px-5 py-3 text-sm text-slate-600">原始商品数据 <span aria-hidden="true">→</span> 转入待适配 <span aria-hidden="true">→</span> 整理商品资料 <span aria-hidden="true">→</span> 上架其他平台</div>
    <div className="mb-5 flex gap-7 border-b border-slate-200 text-sm" role="tablist" aria-label="我的数据分类">
      <button role="tab" aria-selected={tab === "batches"} className={`border-b-2 px-1 pb-3 ${tab === "batches" ? "border-primary font-semibold text-primary" : "border-transparent text-slate-500"}`} onClick={() => changeView("batches")}>采集批次</button>
      <button role="tab" aria-selected={tab === "own"} className={`border-b-2 px-1 pb-3 ${tab === "own" ? "border-primary font-semibold text-primary" : "border-transparent text-slate-500"}`} onClick={() => changeView("own")}>自有商品库</button>
      <button role="tab" aria-selected={false} disabled className="pb-3 text-slate-400" title="数据集尚未开放">数据集</button>
    </div>
    <Card className="mb-5 flex flex-wrap items-center gap-3 p-4">
      <form className="flex min-w-60 flex-1 gap-2" onSubmit={event => { event.preventDefault(); setAfter(undefined); setKeyword(search); setLoading(true); }}>
        <Input aria-label={displayingItems ? "搜索商品名称或标识" : "搜索批次名称"} placeholder={displayingItems ? "搜索商品名称或标识" : "搜索批次名称"} value={search} onChange={event => setSearch(event.target.value)} />
        <Button type="submit" variant="outline">搜索</Button>
      </form>
      {batchId ? <Button variant="ghost" onClick={() => changeView("batches")}>返回批次</Button> : null}
      <Button variant="outline" disabled={busy} onClick={refresh}>刷新</Button>
      <Button variant="outline" disabled={!canManage} onClick={() => { setName(""); setDialog("manage"); }}>管理批次</Button>
      {tab === "own" ? <Button disabled={writeBlocked} onClick={() => setDialog("own")}>添加自有商品</Button> : null}
    </Card>
    {supplyAvailable ? <SupplyCommandFeedback state={supply} /> : null}
    {transferred ? <Button asChild variant="outline" className="mb-4"><Link href={`/workbench/supply/mine?preparation=${transferred}`}>查看我的供应链</Link></Button> : null}
    {foreignPending ? <ConsoleState kind="unavailable" title="原身份下有待核实的操作">请回到原身份与企业后核实，操作键：{pending?.key}</ConsoleState> : pending ? <Card className="mb-4 space-y-3 border-amber-200 bg-amber-50 p-4"><h2 className="font-semibold">结果待核实</h2><p className="break-all text-sm">操作键：{pending.key}</p><div className="flex gap-2"><Button disabled={busy} onClick={() => void command(pending.command, pending, true)}>核实原操作</Button>{error === "NOT_FOUND" ? <Button disabled={busy} variant="outline" onClick={() => void command(pending.command, pending)}>重试原请求</Button> : null}</div></Card> : null}
    {error && error !== "OUTCOME_UNKNOWN" ? <p role="alert" className="mb-4 text-sm text-red-700">{failureText[error] ?? "暂时无法完成请求，请刷新或核实原操作。"}</p> : null}
    {notice ? <p role="status" className="mb-4 break-all text-sm text-emerald-700">{notice}</p> : null}
    <Card className="overflow-hidden rounded-xl border-slate-200">
      <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4"><h2 className="font-semibold">{tab === "own" ? "自有商品库" : batchId ? "批次商品" : "商品批次"}</h2><span className="text-sm text-slate-500">共 {total} {displayingItems ? "件商品" : "个批次"}</span></div>
      {loading ? <div className="p-6" role="status">正在读取商品资料…</div> : !total ? <div className="p-10 text-center text-sm text-slate-500">{tab === "own" ? "还没有自有商品，添加商品后可继续适配。" : "暂无商品批次，采集商品后会保存到这里。"}<div className="mt-4"><Button asChild variant="outline"><Link href="/workbench/supply/acquisition">去采集商品</Link></Button></div></div> : displayingItems ?
        <Table><TableHeader className="bg-slate-50"><TableRow><TableHead>商品</TableHead><TableHead>来源</TableHead><TableHead>保存时间</TableHead><TableHead>操作</TableHead></TableRow></TableHeader><TableBody>{items.map(item => <TableRow key={item.id}><TableCell><button className="flex items-center gap-3 text-left" onClick={() => void showDetail(item)}>{item.thumbnailUrl ? <Image unoptimized src={`/api/image-proxy?url=${encodeURIComponent(item.thumbnailUrl)}`} width={56} height={56} className="rounded-md object-cover" alt="" /> : <span aria-hidden="true" className="h-14 w-14 rounded-md bg-slate-100" />}<span><span className="block max-w-sm font-medium">{item.title || "未命名商品"}</span><span className="text-xs text-slate-400">{item.source.productKey}</span></span></button></TableCell><TableCell>{item.source.kind === "own" ? "自有商品" : "在线采集"}</TableCell><TableCell>{date(item.createdAt)}</TableCell><TableCell><div className="flex gap-2"><Button size="sm" variant="outline" onClick={() => void showDetail(item)}>查看</Button><Button size="sm" variant="ghost" disabled={writeBlocked} onClick={() => { setSelectedItem(item); setTargetBatch(""); setDialog("move"); }}>移动批次</Button><Button size="sm" variant="ghost" disabled={writeBlocked} onClick={() => { setSelectedItem(item); setDialog("archive-item"); }}>归档</Button></div></TableCell></TableRow>)}</TableBody></Table> :
        <Table><TableHeader className="bg-slate-50"><TableRow><TableHead>批次名称</TableHead><TableHead>来源方式</TableHead><TableHead>商品总量</TableHead><TableHead>创建时间</TableHead><TableHead>操作</TableHead></TableRow></TableHeader><TableBody>{batches.map(batch => <TableRow key={batch.id}><TableCell><button className="font-medium text-slate-800 hover:text-primary" onClick={() => changeView("batches", batch.id)}>{batch.name}</button></TableCell><TableCell><span className="rounded-full bg-emerald-50 px-3 py-1 text-xs text-emerald-700">{kindLabels[batch.kind]}</span></TableCell><TableCell>{batch.count}</TableCell><TableCell>{date(batch.createdAt)}</TableCell><TableCell><div className="flex gap-2">{supplyAvailable ? <Button size="sm" variant="outline" disabled={transferBlocked || batch.count===0} onClick={()=>{setSelectedBatch(batch);setDialog("transfer")}}>加入我的供应链</Button> : null}<Button size="sm" variant="outline" onClick={() => changeView("batches", batch.id)}>查看商品</Button><Button size="sm" variant="ghost" disabled={writeBlocked} onClick={() => { setSelectedBatch(batch); setName(batch.name); setDialog("rename"); }}>重命名</Button><Button size="sm" variant="ghost" disabled={writeBlocked} onClick={() => { setSelectedBatch(batch); setDialog("archive-batch"); }}>归档</Button></div></TableCell></TableRow>)}</TableBody></Table>}
      <div className="flex justify-end gap-2 border-t border-slate-100 p-4"><Button size="sm" variant="outline" disabled={!after || loading} onClick={() => { setAfter(undefined); setLoading(true); }}>首页</Button><Button size="sm" variant="outline" disabled={!next || loading} onClick={() => { setAfter(next); setLoading(true); }}>下一页</Button></div>
    </Card>
    {dialog ? <CollectionDialog title={dialog === "transfer" ? "加入我的供应链" : dialog === "manage" ? "批次管理" : dialog === "own" ? "添加自有商品" : dialog === "rename" ? "重命名批次" : dialog === "move" ? "移动批次" : dialog === "detail" ? "原始商品资料" : "归档确认"} onClose={() => { if (!busy) setDialog(null); }}>
      {dialog === "manage" ? <><p className="text-sm text-slate-500">批次可重命名或归档，原始商品资料保留。</p><form className="flex gap-3" onSubmit={event => { event.preventDefault(); void command({ action: "create_batch", name }); }}><Input aria-label="新批次名称" placeholder="输入新批次名称" value={name} onChange={event => setName(event.target.value)} /><Button type="submit" disabled={writeBlocked || !name.trim()}>新建批次</Button></form><div className="flex flex-wrap gap-2">{batches.map(batch => <Button key={batch.id} variant="outline" disabled={writeBlocked} onClick={() => { setSelectedBatch(batch); setName(batch.name); setDialog("rename"); }}>{batch.name} · 重命名</Button>)}</div></> : null}
      {dialog === "rename" && selectedBatch ? <form className="space-y-4" onSubmit={event => { event.preventDefault(); void command({ action: "rename_batch", batchId: selectedBatch.id, expectedRevision: selectedBatch.revision, name }); }}><Input aria-label="批次名称" value={name} onChange={event => setName(event.target.value)} /><Button type="submit" disabled={writeBlocked || !name.trim()}>保存名称</Button></form> : null}
      {dialog === "archive-batch" && selectedBatch ? <><p>归档“{selectedBatch.name}”后将从列表隐藏，原始资料和发布记录保留。</p><Button disabled={writeBlocked} onClick={() => void command({ action: "archive_batch", batchId: selectedBatch.id, expectedRevision: selectedBatch.revision })}>确认归档</Button></> : null}
      {dialog === "archive-item" && selectedItem ? <><p>归档此商品后将从列表隐藏，原始资料和发布记录保留。</p><Button disabled={writeBlocked} onClick={() => void command({ action: "archive_item", itemId: selectedItem.id, expectedRevision: selectedItem.revision })}>确认归档</Button></> : null}
      {dialog === "move" && selectedItem ? <form className="space-y-4" onSubmit={event => { event.preventDefault(); void command({ action: "move_item", itemId: selectedItem.id, targetBatchId: targetBatch, expectedRevision: selectedItem.revision }); }}><label className="grid gap-2">目标批次<Select value={targetBatch} onChange={event => setTargetBatch(event.target.value)}><option value="">选择批次</option>{batches.filter(batch => batch.id !== selectedItem.batchId).map(batch => <option key={batch.id} value={batch.id}>{batch.name}</option>)}</Select></label><p className="text-sm text-slate-500">移动只改变分组，原始资料保留。</p><Button type="submit" disabled={writeBlocked || !targetBatch}>确认移动</Button></form> : null}
      {dialog === "transfer" && selectedBatch ? <><p>将“{selectedBatch.name}”的全部 {selectedBatch.count} 件商品加入我的供应链。</p><p className="text-sm text-slate-500">确认后保存当前批次的原始资料，后续可选择店铺并完成适配。</p><Button disabled={transferBlocked} onClick={()=>supply.execute("transfer",{batchId:selectedBatch.id,expectedRevision:selectedBatch.revision})}>确认加入</Button></> : null}
      {dialog === "own" ? <OwnProductForm disabled={writeBlocked} onSubmit={product => void command({ action: "create_product", product })} /> : null}
      {dialog === "detail" ? detail ? <ProductFacts detail={detail} /> : <p role="status">正在读取原始资料…</p> : null}
    </CollectionDialog> : null}
  </ConsolePage>;
}

export function CollectionDialog({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => { const dialog = ref.current; if (dialog && !dialog.open) { if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", ""); } }, []);
  return <dialog ref={ref} aria-label={title} onCancel={event => { event.preventDefault(); onClose(); }} className="m-auto max-h-[85vh] w-[min(960px,calc(100vw-32px))] overflow-auto rounded-2xl border border-slate-200 bg-white p-6 text-slate-800 shadow-2xl backdrop:bg-slate-900/30"><div className="mb-5 flex items-center justify-between"><h2 className="text-xl font-bold">{title}</h2><Button variant="ghost" size="icon" aria-label="关闭弹窗" onClick={onClose}>×</Button></div><div className="space-y-4">{children}</div></dialog>;
}
function OwnProductForm({ disabled, onSubmit }: { disabled: boolean; onSubmit: (product: ReturnType<typeof ownProductSchema.parse>) => void }) {
  const [title, setTitle] = useState(""); const [description, setDescription] = useState(""); const [brand, setBrand] = useState(""); const [images, setImages] = useState(""); const [sku, setSKU] = useState(""); const [error, setError] = useState(false);
  return <form className="space-y-4" onSubmit={event => { event.preventDefault(); const parsed = ownProductSchema.safeParse({ title, description, brand, images: images.split(/\r?\n/).map(value => value.trim()).filter(Boolean), ...(sku ? { variants: [{ sourceId: sku, title, sku, attributes: {}, currency: "", price: 0, stock: 0 }] } : {}) }); if (!parsed.success) { setError(true); return; } setError(false); onSubmit(parsed.data); }}>
    <label className="grid gap-2 text-sm">商品名称<Input required value={title} onChange={event => setTitle(event.target.value)} /></label>
    <div className="grid gap-4 sm:grid-cols-2"><label className="grid gap-2 text-sm">SKU<Input value={sku} onChange={event => setSKU(event.target.value)} /></label><label className="grid gap-2 text-sm">品牌<Input value={brand} onChange={event => setBrand(event.target.value)} /></label></div>
    <label className="grid gap-2 text-sm">商品描述<Textarea rows={4} value={description} onChange={event => setDescription(event.target.value)} /></label>
    <label className="grid gap-2 text-sm">原始图片链接<Textarea rows={3} placeholder="每行一个 HTTPS 图片链接" value={images} onChange={event => setImages(event.target.value)} /></label>
    <p className="text-xs text-slate-500">保存用户提供的原始资料；图片在适配时由你确认，保存不会自动上传到平台。</p>
    {error ? <p role="alert" className="text-sm text-red-700">请填写商品名称并检查图片链接。</p> : null}<Button type="submit" disabled={disabled}>保存商品</Button>
  </form>;
}
function ProductFacts({ detail }: { detail: CollectionDetail }) {
  const product = detail.product;
  return <div className="space-y-5"><h3 className="text-lg font-semibold">{product.title || "未命名商品"}</h3><p className="break-all text-xs text-slate-500">{detail.item.source.productKey} · 原始版本 {detail.item.source.version}</p><p className="whitespace-pre-wrap text-sm">{product.description || "没有商品描述"}</p><dl className="grid grid-cols-2 gap-3 text-sm">{product.attributes?.map((attribute, index) => <div key={index}><dt className="text-slate-500">{attribute.name}</dt><dd>{attribute.value}</dd></div>)}</dl><div className="flex flex-wrap gap-3">{product.images?.map((image, index) => <Image key={`${image.url}:${index}`} unoptimized src={`/api/image-proxy?url=${encodeURIComponent(image.url)}`} width={128} height={128} className="rounded-lg object-contain" alt={`原始商品图片 ${index + 1}`} />)}</div><Button variant="outline" onClick={() => downloadJSON(detail)}>下载原始资料</Button></div>;
}
function downloadJSON(detail: CollectionDetail) { const url = URL.createObjectURL(new Blob([JSON.stringify(detail, null, 2)], { type: "application/json" })); const link = document.createElement("a"); link.href = url; link.download = `product-${detail.item.id}.json`; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); }
function date(value: string) { return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(new Date(value)); }
function codeOf(error: unknown) { return error instanceof CollectionAPIError ? error.code : "DEPENDENCY_UNAVAILABLE"; }
