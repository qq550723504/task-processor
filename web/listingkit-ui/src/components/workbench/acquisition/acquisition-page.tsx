"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { AcquisitionAPIError, acquire1688, readAcquisition, readAcquisitionProduct, verify1688, type AcquisitionContext, type AcquisitionOperation } from "@/lib/api/product-acquisition";
import { canonical1688Source, isAcquisitionUUID, type AcquisitionProduct, type AcquisitionResult } from "@/lib/contracts/product-acquisition";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";
import { ConsolePage, ConsoleState } from "../console/console-page";

type PendingIntent = AcquisitionOperation;

export function AcquisitionPage({ operationId }: { operationId?: string }) {
  const context = useWorkbenchContext();
  const scope = useMemo<AcquisitionContext | null>(() => {
    const userId = context.user?.id;
    const organizationId = context.effectiveOrganization?.id;
    return userId && organizationId ? { userId, organizationId } : null;
  }, [context.effectiveOrganization?.id, context.user?.id]);

  if (context.isLoading || context.isSwitching) return <ConsoleState kind="loading" title="正在确认企业上下文">采集只在服务端确认的当前企业中执行。</ConsoleState>;
  if (!scope || context.selectionRequired || context.error || context.blockingError) return <ConsoleState kind="error" title="企业或登录上下文不可用">请先确认登录身份与当前企业，再提交或读取采集结果。</ConsoleState>;

  // A changed verified actor or organization must discard the prior operation
  // state rather than replaying it under the new server-side scope.
  return <ScopedAcquisitionPage key={`${scope.userId}:${scope.organizationId}:${operationId ?? ""}`} operationId={operationId} scope={scope} />;
}

function ScopedAcquisitionPage({ operationId, scope }: { operationId?: string; scope: AcquisitionContext }) {
  const router = useRouter();
  const [source, setSource] = useState("");
  const [recoveryID, setRecoveryID] = useState(operationId ?? "");
  const [pending, setPending] = useState<PendingIntent | null>(null);
  const [result, setResult] = useState<AcquisitionResult | null>(null);
  const [product, setProduct] = useState<AcquisitionProduct | null>(null);
  const [error, setError] = useState<string | null>(() => operationId && !isAcquisitionUUID(operationId) ? "INVALID_ACQUISITION" : null);
  const [busy, setBusy] = useState(() => Boolean(operationId && isAcquisitionUUID(operationId)));
  const active = useRef(true);
  const inFlight = useRef(false);
  const abort = useRef<AbortController | null>(null);

  useEffect(() => {
    active.current = true;
    return () => { active.current = false; abort.current?.abort(); };
  }, []);

  useEffect(() => {
    if (!operationId || !isAcquisitionUUID(operationId)) return;
    const controller = new AbortController();
    let active = true;
    void readAcquisitionProduct(operationId, scope, controller.signal).then((next) => { if (active) setProduct(next); }).catch((failure: unknown) => { if (active) setError(codeOf(failure)); }).finally(() => { if (active) setBusy(false); });
    return () => { active = false; controller.abort(); };
  }, [operationId, scope]);

  if (operationId) return <ProductDetail operationId={operationId} product={product} busy={busy} error={error} />;

  async function submit(verify = false) {
    if (!active.current || inFlight.current) return;
    const canonical = canonical1688Source(source);
    if (!canonical) { setError("INVALID_ACQUISITION"); return; }
    const intent: AcquisitionOperation = pending ?? { userId: scope.userId, organizationId: scope.organizationId, key: crypto.randomUUID(), source: canonical };
    const controller = new AbortController();
    abort.current = controller; inFlight.current = true; setPending(intent); setBusy(true); setError(null);
    try {
      const next = await (verify ? verify1688(intent, controller.signal) : acquire1688(intent, controller.signal));
      if (!active.current) return;
      setResult(next);
      if (next.outcome === "published") router.push(`/workbench/supply/acquisition/operation/${next.operationId}`);
    } catch (failure) {
      if (active.current) setError(codeOf(failure));
    } finally {
      if (abort.current === controller) abort.current = null;
      inFlight.current = false;
      if (active.current) setBusy(false);
    }
  }
  async function recover() {
    if (!active.current || inFlight.current) return;
    if (!isAcquisitionUUID(recoveryID)) { setError("INVALID_ACQUISITION"); return; }
    const controller = new AbortController();
    abort.current = controller; inFlight.current = true; setBusy(true); setError(null);
    try {
      const next = await readAcquisition(recoveryID, scope, controller.signal);
      if (!active.current) return;
      setResult(next);
      if (next.outcome === "published") router.push(`/workbench/supply/acquisition/operation/${next.operationId}`);
    } catch (failure) {
      if (active.current) setError(codeOf(failure));
    } finally {
      if (abort.current === controller) abort.current = null;
      inFlight.current = false;
      if (active.current) setBusy(false);
    }
  }

  return <ConsolePage title="1688采集" breadcrumbs={findConsoleRoute("/workbench/supply/acquisition")?.trail} description="提交公开商品页或 offer ID。系统仍按当前登录身份和企业授权；不需要源账号或 1688 登录。">
    <Card className="space-y-4 p-5"><h2 className="text-base font-semibold">采集公开商品</h2><form className="space-y-3" onSubmit={(event) => { event.preventDefault(); void submit(); }}><label className="grid gap-2" htmlFor="acquisition-source"><span>1688 商品页或 offer ID</span><Input id="acquisition-source" value={source} onChange={(event) => { setSource(event.target.value); setPending(null); }} disabled={busy} placeholder="https://detail.1688.com/offer/123.html" /></label><div className="flex gap-2"><Button type="submit" disabled={busy}>提交采集</Button>{pending && error === "OUTCOME_UNKNOWN" ? <Button type="button" variant="outline" disabled={busy} onClick={() => void submit(true)}>使用原 key 核实</Button> : null}</div></form>{result ? <ResultState result={result} /> : null}{error ? <Failure code={error} /> : null}</Card>
    <Card className="mt-5 space-y-3 p-5"><h2 className="text-base font-semibold">找回采集结果</h2><p className="text-sm text-muted-foreground">粘贴操作 ID 仅读取该 ID 在当前身份和企业下的结果；不会创建新操作。</p><form className="flex max-w-xl gap-2" onSubmit={(event) => { event.preventDefault(); void recover(); }}><Input aria-label="操作 ID" value={recoveryID} onChange={(event) => setRecoveryID(event.target.value)} disabled={busy} /><Button type="submit" variant="outline" disabled={busy}>读取</Button></form></Card>
  </ConsolePage>;
}

function ProductDetail({ operationId, product, busy, error }: { operationId: string; product: AcquisitionProduct | null; busy: boolean; error: string | null }) {
  return <ConsolePage title="采集商品" breadcrumbs={findConsoleRoute(`/workbench/supply/acquisition/operation/${operationId}`)?.trail} actions={<Button asChild variant="outline"><Link href="/workbench/supply/acquisition">继续采集</Link></Button>} description="此页按操作回执锁定的 Catalog version 显示，不提供通用商品搜索或编辑。">
    {busy ? <ConsoleState kind="loading" title="正在读取已发布的商品">正在确认操作、授权和精确 Catalog version。</ConsoleState> : error ? <Failure code={error} /> : product ? <CatalogFacts product={product} /> : <ConsoleState kind="unavailable" title="没有可读取的采集商品">该操作尚未产生已发布的 Catalog 快照。</ConsoleState>}
  </ConsolePage>;
}

function ResultState({ result }: { result: AcquisitionResult }) {
  const labels: Record<AcquisitionResult["outcome"], string> = { acquiring: "正在采集", prepared: "正在发布", outcome_unknown: "结果待核实", published: "已发布，正在打开商品", failed: "采集失败" };
  return <p role="status">操作 {result.operationId}：{labels[result.outcome]}。</p>;
}
function CatalogFacts({ product }: { product: AcquisitionProduct }) {
  return <div className="space-y-5"><Card className="p-5"><h2 className="text-xl font-semibold">{product.title || "未提供标题"}</h2><p className="mt-2 text-sm text-muted-foreground">操作 {product.operationId} · Catalog 版本 {product.catalogVersion}</p></Card><Card className="p-5"><h2 className="font-semibold">来源与采集图片</h2>{product.sources.length ? <ul className="mt-3 list-disc space-y-1 pl-5">{product.sources.map((source, index) => <li key={`${source.url}:${index}`}>{source.platform || "未标注平台"}{source.url ? <> · <a className="underline" href={source.url} rel="noreferrer" target="_blank">查看来源</a></> : null}</li>)}</ul> : <p className="mt-3 text-sm text-muted-foreground">未提供来源记录。</p>}{product.images.length ? <ul className="mt-3 list-disc space-y-1 pl-5">{product.images.map((image, index) => <li key={`${image.url}:${index}`}><a className="underline" href={image.url} rel="noreferrer" target="_blank">图片候选 {index + 1}</a>{image.role ? `（${image.role}）` : ""}</li>)}</ul> : <p className="mt-3 text-sm text-muted-foreground">未采集到图片候选。</p>}</Card><Card className="p-5"><h2 className="font-semibold">规格与缺失信息</h2>{product.specifications.length ? <dl className="mt-3 grid gap-2 sm:grid-cols-2">{product.specifications.map((item, index) => <div key={`${item.name}:${index}`}><dt className="text-sm text-muted-foreground">{item.name}</dt><dd>{item.value}</dd></div>)}</dl> : <p className="mt-3 text-sm text-muted-foreground">未提供规格。</p>}{product.missingFacts.length ? <ul className="mt-4 list-disc space-y-1 pl-5 text-sm">{product.missingFacts.map((fact, index) => <li key={`${fact.field}:${index}`}>{fact.field}：{fact.reason}</li>)}</ul> : null}</Card></div>;
}
function Failure({ code }: { code: string }) { const labels: Record<string, string> = { OUTCOME_UNKNOWN: "原请求结果尚未确定；仅当仍持有原 key 和来源时可核实。", ACQUISITION_NOT_FOUND: "当前身份和企业下未找到该操作。", FORBIDDEN: "当前身份没有采集或读取权限。", INVALID_ACQUISITION: "请输入合法的 1688 商品页、offer ID 或操作 ID。", ACQUISITION_UNAVAILABLE: "采集结果暂不可用；未据此推断其已失败。" }; return <ConsoleState kind="error" title={labels[code] ?? "采集请求未完成"}>{code}</ConsoleState>; }
function codeOf(failure: unknown) { return failure instanceof AcquisitionAPIError ? failure.code : "ACQUISITION_UNAVAILABLE"; }
