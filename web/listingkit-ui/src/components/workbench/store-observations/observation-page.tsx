"use client";
import { useEffect, useMemo, useRef, useState } from "react";
import Image from "next/image";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { RefreshCw, ExternalLink, Copy, X, ShoppingBag } from "lucide-react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import {
  ConsolePage,
  ConsoleToolbar,
  ConsoleState,
} from "@/components/workbench/console/console-page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import {
  listWorkbenchStores,
  type WorkbenchStore,
} from "@/lib/api/workbench-stores";
import {
  observationRequest,
  observationBeginSchema,
  observationCapabilitiesSchema,
  observationCommandSchema,
  observationID,
  observationListSchema,
  observationRecordSchema,
  observationSyncSchema,
  observationTracksSchema,
  ObservationError,
  orderStatus,
  orderProblems,
  type ObservationKind,
  type ObservationRecord,
  type ObservationScope,
  type ObservationPrice,
  type ObservationSync,
} from "@/lib/api/store-observations";
import styles from "./observation-page.module.css";
const intentSchema = z
  .object({ key: observationID, input: observationBeginSchema })
  .strict();
type Intent = z.infer<typeof intentSchema>;
const terminal = (s: ObservationSync) =>
  !["pending", "running"].includes(s.status);
const statusLabels: Record<ObservationSync["status"], string> = {
  pending: "等待启动",
  running: "正在同步",
  completed: "遍历完成",
  partial: "部分结果",
  failed: "同步失败",
  suspended: "同步暂停",
};
const platformURL = "https://sellerhub.shein.com/";
const inaccessibleReceipt = (error: unknown) =>
  error instanceof ObservationError &&
  ((error.status === 404 && error.code === "NOT_FOUND") ||
    (error.status === 403 && error.code === "PERMISSION_DENIED"));
export function ObservationPage({ kind }: { kind: ObservationKind }) {
  const context = useWorkbenchContext();
  const org = context.effectiveOrganization?.id ?? "",
    user = context.user?.id ?? "";
  if (!org || !user || context.isSwitching)
    return <ConsoleState kind="loading" title="正在切换企业数据…" />;
  return (
    <ScopedPage
      key={`${org}:${user}:${kind}`}
      kind={kind}
      scope={{ organizationId: org, userId: user }}
      permissions={context.permissions}
    />
  );
}
function ScopedPage({
  kind,
  scope,
  permissions,
}: {
  kind: ObservationKind;
  scope: ObservationScope;
  permissions: string[];
}) {
  const queryClient = useQueryClient();
  const prefix = useMemo(
    () => ["store-observations", scope.organizationId, scope.userId, kind],
    [scope.organizationId, scope.userId, kind],
  );
  const lifetime = useRef(new AbortController());
  const [store, setStore] = useState(""),
    [status, setStatus] = useState(""),
    [search, setSearch] = useState(""),
    [keyword, setKeyword] = useState(""),
    [after, setAfter] = useState(""),
    [history, setHistory] = useState<string[]>([]),
    [selected, setSelected] = useState<ObservationRecord | null>(null),
    [partial, setPartial] = useState("");
  const [intent, setIntent] = useState<Intent | null>(null),
    [hydrated, setHydrated] = useState(false),
    [storageFailed, setStorageFailed] = useState(false);
  const storageKey = `store-observations:${scope.organizationId}:${scope.userId}:${kind}`;
  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    lifetime.current = controller;
    void Promise.resolve().then(() => {
      if (!active) return;
      try {
        const raw = sessionStorage.getItem(storageKey);
        if (raw) {
          const parsed = intentSchema.safeParse(JSON.parse(raw));
          if (parsed.success && parsed.data.input.kind === kind)
            setIntent(parsed.data);
        }
      } catch {
        setStorageFailed(true);
      }
      setHydrated(true);
    });
    return () => {
      active = false;
      controller.abort();
      void queryClient.cancelQueries({ queryKey: prefix });
      queryClient.removeQueries({ queryKey: prefix });
    };
  }, [queryClient, storageKey, kind, prefix]);
  const caps = useQuery({
    queryKey: [...prefix, "capabilities", permissions],
    queryFn: ({ signal }) =>
      observationRequest(
        `${kind}/capabilities`,
        scope,
        observationCapabilitiesSchema,
        signal,
      ),
    retry: false,
  });
  const stores = useQuery({
    queryKey: [...prefix, "stores", permissions],
    enabled: caps.data?.available === true,
    queryFn: async ({ signal }) => {
      const out: WorkbenchStore[] = [];
      for (let page = 1; page <= 5; page++) {
        const result = await listWorkbenchStores(
          { page, pageSize: 100, platform: "shein", status: "active" },
          scope.organizationId,
          signal,
        );
        if (result.pagination.total > 500)
          throw new ObservationError("DEPENDENCY_UNAVAILABLE", 503);
        out.push(...result.items);
        if (page * 100 >= result.pagination.total) return out;
      }
      return out;
    },
    retry: false,
  });
  const params = new URLSearchParams({ limit: "20" });
  if (store) params.set("storeId", store);
  if (status) params.set("status", status);
  if (keyword) params.set("keyword", keyword);
  if (after) params.set("after", after);
  if (partial && store) params.set("syncId", partial);
  const list = useQuery({
    queryKey: [...prefix, "list", params.toString(), permissions],
    enabled: caps.data?.available === true,
    queryFn: ({ signal }) =>
      observationRequest(
        `${kind}?${params}`,
        scope,
        observationListSchema,
        signal,
      ),
    retry: false,
    refetchInterval: (q) =>
      q.state.data?.latest.some((s) => !terminal(s)) ? 3000 : false,
  });
  const commandKey = [...prefix, "command", intent?.key];
  const command = useQuery({
    queryKey: commandKey,
    enabled: hydrated && intent !== null && caps.data?.available === true,
    queryFn: ({ signal }) =>
      observationRequest(
        `${kind}/commands/${intent!.key}`,
        scope,
        observationCommandSchema,
        signal,
      ),
    retry: false,
    refetchInterval: (q) =>
      q.state.data?.syncs.some((s) => !terminal(s)) ? 3000 : false,
  });
  const sync = useMutation({
    mutationFn: (captured: Intent) =>
      observationRequest(
        `${kind}/syncs`,
        scope,
        observationCommandSchema,
        lifetime.current.signal,
        { key: captured.key, input: captured.input },
      ),
    retry: false,
    onSuccess: (receipt, captured) => {
      if (lifetime.current.signal.aborted) return;
      queryClient.setQueryData([...prefix, "command", captured.key], receipt);
      void queryClient.invalidateQueries({ queryKey: [...prefix, "list"] });
    },
    onError: () => {
      if (!lifetime.current.signal.aborted) void command.refetch();
    },
  });
  const ensure = useMutation({
    mutationFn: (id: string) =>
      observationRequest(
        `${kind}/syncs/${id}/ensure`,
        scope,
        observationSyncSchema,
        lifetime.current.signal,
        {},
      ),
    onSuccess: () => {
      if (lifetime.current.signal.aborted) return;
      void command.refetch();
      void list.refetch();
    },
    retry: false,
  });
  const receipt = inaccessibleReceipt(command.error) ? undefined : command.data;
  const isActive = receipt?.syncs.some((s) => !terminal(s)) ?? false;
  const canDiscardIntent =
    intent !== null &&
    !receipt &&
    !command.isFetching &&
    inaccessibleReceipt(command.error) &&
    !sync.isPending &&
    (!sync.error || inaccessibleReceipt(sync.error));
  const discardIntent = () => {
    if (!canDiscardIntent) return;
    try {
      sessionStorage.removeItem(storageKey);
    } catch {
      setStorageFailed(true);
      return;
    }
    setStorageFailed(false);
    setIntent(null);
    sync.reset();
    ensure.reset();
    queryClient.removeQueries({ queryKey: commandKey, exact: true });
  };
  const begin = () => {
    if (intent && !receipt) return;
    if (isActive) return;
    const captured: Intent = {
      key: crypto.randomUUID(),
      input: { kind, stores: store ? [store] : [] },
    };
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(captured));
      setStorageFailed(false);
    } catch {
      setStorageFailed(true);
      return;
    }
    setIntent(captured);
    sync.mutate(captured);
  };
  const resetPage = () => {
    setAfter("");
    setHistory([]);
    setPartial("");
  };
  const names = new Map(stores.data?.map((s) => [s.id, s.name]));
  const data = list.data;
  const canStart =
    caps.data?.canSync === true &&
    hydrated &&
    !sync.isPending &&
    !isActive &&
    (!intent || Boolean(receipt));
  const known = Boolean(data?.syncs.length);
  const metric = (n: number | undefined) =>
    known ? `${n ?? 0}${data?.complete ? "" : " 已取得"}` : "—";
  const metrics =
    kind === "products"
      ? [
          ["平台商品（SKC）", metric(data?.summary.total)],
          ["在售", metric(data?.summary.active)],
          ["审核中", "平台未提供"],
          ["已下架", metric(data?.summary.offShelf)],
        ]
      : [
          [
            "今日下单（截至同步）",
            data?.summary.todayUnknown
              ? "平台未提供完整时间"
              : metric(data?.summary.today),
          ],
          ["待处理 / 待发货", metric(data?.summary.pending)],
          ["已发货 / 待揽收", metric(data?.summary.transit)],
          ["平台异常", metric(data?.summary.exceptional)],
        ];
  return (
    <ConsolePage
      title={kind === "products" ? "店铺商品" : "订单履约"}
      breadcrumbs={[
        { label: "店铺中心" },
        { label: kind === "products" ? "店铺商品" : "订单履约" },
      ]}
      description={
        kind === "products"
          ? "查看 SHEIN 美国站的已保存平台商品、售价和逐仓库存。"
          : "同步最近 30 天的消费者订单，查看详情、物流和平台异常。发货与售后到 SHEIN 处理。"
      }
      actions={
        <Button disabled={!canStart} onClick={begin}>
          <RefreshCw aria-hidden="true" />
          {kind === "products" ? "同步商品" : "同步订单"}
        </Button>
      }
    >
      <div className={styles.metrics}>
        {metrics.map(([label, value]) => (
          <Card key={label} className={styles.metric}>
            <span>{label}</span>
            <strong>{value}</strong>
          </Card>
        ))}
      </div>
      <ConsoleToolbar>
        <label>
          平台
          <Select aria-label="平台" disabled value="shein">
            <option value="shein">SHEIN</option>
          </Select>
        </label>
        <label>
          站点
          <Select aria-label="站点" disabled value="us">
            <option value="us">美国站</option>
          </Select>
        </label>
        <label>
          店铺
          <Select
            aria-label="店铺"
            value={store}
            onChange={(e) => {
              setStore(e.target.value);
              resetPage();
            }}
          >
            <option value="">全部授权店铺</option>
            {stores.data?.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        </label>
        <label>
          状态
          <Select
            aria-label="状态"
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              resetPage();
            }}
          >
            <option value="">全部状态</option>
            {(kind === "products"
              ? [
                  ["active", "在售"],
                  ["off", "已下架"],
                  ["unknown", "平台未提供"],
                ]
              : [
                  ["1", "待处理"],
                  ["2", "待发货"],
                  ["3", "待 SHEIN 发货"],
                  ["transit", "已发货 / 待揽收"],
                  ["5", "已签收"],
                  ["6", "已退款"],
                  ["exceptional", "平台异常"],
                  ["unknown", "未识别 / 未提供"],
                ]
            ).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </Select>
        </label>
        <form
          className={styles.search}
          onSubmit={(e) => {
            e.preventDefault();
            setKeyword(search.trim());
            resetPage();
          }}
        >
          <label htmlFor={`observation-search-${kind}`}>
            {kind === "products"
              ? "标题 / SKU / 商品 ID"
              : "订单号 / 商品 / SKU"}
          </label>
          <div>
            <Input
              id={`observation-search-${kind}`}
              maxLength={200}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <Button variant="outline" type="submit">
              搜索
            </Button>
          </div>
        </form>
      </ConsoleToolbar>
      {storageFailed ? (
        <p role="alert" className={styles.notice}>
          无法保存本次同步操作。请允许当前浏览器使用会话存储后重试。
        </p>
      ) : null}
      {intent && caps.data?.available ? (
        <Card className={styles.progress}>
          {receipt ? (
            <>
              <h2>本次同步</h2>
              <ul>
                {receipt.syncs.map((s) => (
                  <li key={s.id}>
                    <span>
                      {names.get(s.storeId) ?? s.storeId} ·{" "}
                      {statusLabels[s.status]} · 已读取 {s.progress.pages} 页
                      {s.errorCode === "unsupported_application"
                        ? " · 全托管应用不支持消费者订单"
                        : s.errorCode
                          ? " · 授权、连接或服务不可用"
                          : ""}
                    </span>
                    {!terminal(s) ? (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={ensure.isPending || !caps.data?.canSync}
                        onClick={() => ensure.mutate(s.id)}
                      >
                        继续同步
                      </Button>
                    ) : null}
                    {s.status === "partial" || s.status === "failed" ? (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => {
                          setStore(s.storeId);
                          setPartial(s.id);
                          setAfter("");
                          setHistory([]);
                        }}
                      >
                        查看部分结果
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            </>
          ) : (
            <p role="status">
              {sync.isPending
                ? "正在提交同步…"
                : canDiscardIntent
                  ? "原同步回执不存在或当前成员无权访问。可清除此页记录后发起新同步；旧同步不会被取消。"
                  : "同步结果待核实。保留原操作，可查询回执或重试原同步。"}
            </p>
          )}
          {!receipt && !sync.isPending ? (
            <div className={styles.actions}>
              <Button
                variant="outline"
                disabled={!caps.data.canSync}
                onClick={() => sync.mutate(intent)}
              >
                重试原同步
              </Button>
              <Button variant="ghost" onClick={() => void command.refetch()}>
                查询原回执
              </Button>
              {canDiscardIntent ? (
                <Button variant="ghost" onClick={discardIntent}>
                  清除此页同步记录
                </Button>
              ) : null}
            </div>
          ) : null}
          {ensure.isError ? (
            <p role="alert">恢复暂未成功，请稍后用原同步重试。</p>
          ) : null}
        </Card>
      ) : null}
      {caps.isPending ? (
        <ConsoleState kind="loading" title="正在检查当前企业权限…" />
      ) : caps.isError ? (
        <ErrorState error={caps.error} retry={() => void caps.refetch()} />
      ) : !caps.data?.available ? (
        <ConsoleState kind="unavailable" title="平台同步暂未开放">
          当前实例尚未完成商品和订单同步接线。
        </ConsoleState>
      ) : list.isPending ? (
        <ConsoleState kind="loading" title="正在加载已保存数据…" />
      ) : list.isError ? (
        <ErrorState
          error={list.error}
          retry={() => {
            setAfter("");
            setHistory([]);
            void list.refetch();
          }}
        />
      ) : data ? (
        <>
          <p className={styles.coverage} role="status">
            {data.complete
              ? "已完成所选店铺的范围遍历。"
              : "覆盖不完整：含未同步、部分结果或当前不可用的店铺。"}
            {partial ? " 当前查看指定同步的部分结果。" : ""}
            统计基于全部筛选结果；平台列表不保证取得期间的原子快照。
            {kind === "orders" ? "今日按 UTC+8 下单时间统计。" : ""}
          </p>
          {data.syncs.length ? (
            <div className={styles.sources}>
              {data.syncs.map((s) => (
                <span key={s.id}>
                  {names.get(s.storeId) ?? s.storeId} ·{" "}
                  {s.observedAt
                    ? `取得于 ${timestamp(s.observedAt)}`
                    : "尚未取得"}
                  {s.range
                    ? ` · 范围 ${timestamp(s.range.start)} 至 ${timestamp(s.range.end)}`
                    : ""}
                  {s.progress.notes.length
                    ? ` · ${s.progress.notes.map(coverageNote).join("、")}`
                    : ""}
                </span>
              ))}
            </div>
          ) : null}
          {data.latest.some((s) => s.errorCode || s.status === "partial") ? (
            <div className={styles.notice}>
              <p>最近同步有未完成结果，列表可能保留上次完整观察。</p>
              <ul>
                {data.latest
                  .filter((s) => s.errorCode || s.status === "partial")
                  .map((s) => (
                    <li key={s.id}>
                      {names.get(s.storeId) ?? s.storeId} ·{" "}
                      {statusLabels[s.status]}
                      {s.errorCode === "unsupported_application"
                        ? " · 全托管应用不支持消费者订单"
                        : s.errorCode
                          ? " · 授权、连接或服务不可用"
                          : " · 仅取得部分结果"}
                    </li>
                  ))}
              </ul>
            </div>
          ) : null}
          {data.items.length === 0 ? (
            <ConsoleState
              kind="empty"
              title={
                data.syncs.length
                  ? "没有已取得且符合筛选的记录"
                  : data.latest.length
                    ? "最近同步未取得可显示的记录"
                    : "尚未同步平台数据"
              }
            >
              使用页面上方同步按钮取得平台数据。只有完整遍历后的空结果才表示此范围内未取得记录。
            </ConsoleState>
          ) : kind === "products" ? (
            <ProductTable rows={data.items} names={names} open={setSelected} />
          ) : (
            <OrderTable rows={data.items} names={names} open={setSelected} />
          )}
          <div className={styles.pagination}>
            <span>
              每页最多 20 条 · {data.complete ? "筛选结果" : "已取得的筛选结果"}{" "}
              {data.summary.total} 条
            </span>
            <div>
              <Button
                variant="outline"
                disabled={!history.length}
                onClick={() => {
                  setAfter(history.at(-1) ?? "");
                  setHistory(history.slice(0, -1));
                }}
              >
                上一页
              </Button>
              <Button
                variant="outline"
                disabled={!data.next}
                onClick={() => {
                  setHistory([...history, after]);
                  setAfter(data.next);
                }}
              >
                下一页
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  setAfter("");
                  setHistory([]);
                  void list.refetch();
                }}
              >
                <RefreshCw aria-hidden="true" />
                刷新
              </Button>
            </div>
          </div>
        </>
      ) : null}
      {selected && caps.data?.available && !caps.isError ? (
        <Detail
          key={`${selected.storeId}:${selected.syncId}:${selected.id}`}
          kind={kind}
          scope={scope}
          selected={selected}
          close={() => setSelected(null)}
        />
      ) : null}
    </ConsolePage>
  );
}
function ProductTable({
  rows,
  names,
  open,
}: {
  rows: ObservationRecord[];
  names: Map<string, string>;
  open: (v: ObservationRecord) => void;
}) {
  return (
    <Card className={styles.table}>
      <Table>
        <TableHeader>
          <TableRow>
            {[
              "商品信息",
              "SKU",
              "平台 / 站点",
              "店铺",
              "售价",
              "库存（逐仓）",
              "状态",
              "取得时间",
              "操作",
            ].map((h) => (
              <TableHead key={h}>{h}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => {
            const skc = r.product!.skcs[0];
            return (
              <TableRow key={`${r.storeId}:${r.id}:${skc.id}`}>
                <TableCell>
                  <div className={styles.identity}>
                    <Thumbnail src={skc.imageUrl} />
                    <div>
                      <strong>{skc.title || "平台未提供标题"}</strong>
                      <small>
                        SPU {r.id}
                        <br />
                        SKC {skc.id}
                      </small>
                    </div>
                  </div>
                </TableCell>
                <TableCell>
                  {skc.skus.slice(0, 3).map((s) => (
                    <small key={s.id}>{s.sellerSku || s.id}</small>
                  ))}
                  {skc.skus.length > 3 ? "更多见详情" : null}
                </TableCell>
                <TableCell>
                  SHEIN
                  <br />
                  {skc.site === "shein-us" ? "美国站" : "站点未提供"}
                </TableCell>
                <TableCell>{names.get(r.storeId) ?? r.storeId}</TableCell>
                <TableCell>
                  {priceList(skc.skus.flatMap((s) => s.prices))}
                </TableCell>
                <TableCell>
                  {skc.skus.some((s) => s.inventory.length)
                    ? "按 SKU / 仓库查看详情"
                    : "平台未提供"}
                </TableCell>
                <TableCell>
                  <span className={styles.pill}>
                    {skc.siteStatus === 1
                      ? "在售"
                      : skc.siteStatus === 0
                        ? "已下架"
                        : skc.siteStatus === null
                          ? "平台未提供"
                          : `未识别 (${skc.siteStatus})`}
                  </span>
                </TableCell>
                <TableCell>{timestamp(r.observedAt)}</TableCell>
                <TableCell>
                  <Button size="sm" variant="ghost" onClick={() => open(r)}>
                    查看详情
                  </Button>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </Card>
  );
}
function OrderTable({
  rows,
  names,
  open,
}: {
  rows: ObservationRecord[];
  names: Map<string, string>;
  open: (v: ObservationRecord) => void;
}) {
  return (
    <Card className={styles.table}>
      <Table>
        <TableHeader>
          <TableRow>
            {[
              "订单号",
              "商品信息",
              "店铺 / 站点",
              "订单金额",
              "下单时间",
              "平台状态",
              "物流 / 异常",
              "发货时限",
              "操作",
            ].map((h) => (
              <TableHead key={h}>{h}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => {
            const o = r.order!;
            const problems = orderProblems(o);
            return (
              <TableRow key={`${r.storeId}:${r.id}`}>
                <TableCell>
                  <strong>{r.id}</strong>
                </TableCell>
                <TableCell>
                  {o.items.slice(0, 2).map((it) => (
                    <small key={it.id}>
                      {it.title || it.sellerSku || it.sku || it.id}
                    </small>
                  ))}
                  {o.items.length > 2 ? (
                    <small>等 {o.items.length} 个单件记录</small>
                  ) : null}
                </TableCell>
                <TableCell>
                  {names.get(r.storeId) ?? r.storeId}
                  <small>SHEIN · 美国站</small>
                </TableCell>
                <TableCell>{price(o.amount)}</TableCell>
                <TableCell>{timestamp(o.createdAt)}</TableCell>
                <TableCell>
                  <span className={styles.pill}>{orderStatus(o.status)}</span>
                </TableCell>
                <TableCell>
                  {problems.length
                    ? problems.slice(0, 2).join("；")
                    : o.status === 4
                      ? "已发货，轨迹见详情"
                      : o.status === 7
                        ? "待揽收，轨迹见详情"
                        : "轨迹见详情"}
                </TableCell>
                <TableCell>{timestamp(o.needDeliveryAt)}</TableCell>
                <TableCell>
                  <div className={styles.actions}>
                    <Button size="sm" variant="ghost" onClick={() => open(r)}>
                      详情 / 物流
                    </Button>
                    <PlatformLink />
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </Card>
  );
}
function Detail({
  kind,
  scope,
  selected,
  close,
}: {
  kind: ObservationKind;
  scope: ObservationScope;
  selected: ObservationRecord;
  close: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [pkg, setPkg] = useState("");
  const [copied, setCopied] = useState(false);
  const base = `${kind}/stores/${selected.storeId}/syncs/${selected.syncId}/records/${encodeURIComponent(selected.id)}`;
  const prefix = [
    "store-observations",
    scope.organizationId,
    scope.userId,
    kind,
    "detail",
    selected.storeId,
    selected.syncId,
    selected.id,
  ];
  const detail = useQuery({
    queryKey: prefix,
    queryFn: ({ signal }) =>
      observationRequest(base, scope, observationRecordSchema, signal),
    retry: false,
  });
  const order = detail.data?.order;
  const trackable = order?.packages.filter((p) => p.id !== "") ?? [];
  const packageID = trackable.some((p) => p.id === pkg)
    ? pkg
    : (trackable[0]?.id ?? "");
  const tracks = useQuery({
    queryKey: [...prefix, "tracks", packageID],
    enabled:
      kind === "orders" &&
      Boolean(packageID) &&
      !detail.isError &&
      !detail.data?.stale,
    queryFn: ({ signal }) =>
      observationRequest(
        `${base}/packages/${encodeURIComponent(packageID)}/track`,
        scope,
        observationTracksSchema,
        signal,
      ),
    retry: false,
  });
  useEffect(() => {
    const element = dialog.current,
      focus = document.activeElement as HTMLElement | null;
    if (element && !element.open) element.showModal();
    return () => {
      element?.close();
      focus?.focus();
    };
  }, []);
  return (
    <dialog
      ref={dialog}
      className={styles.dialog}
      aria-labelledby="observation-detail-title"
      onCancel={close}
      onClose={close}
    >
      <header>
        <div>
          <h2 id="observation-detail-title">
            {kind === "products" ? "商品详情" : "订单详情与物流"}
          </h2>
          <p>{selected.id}</p>
        </div>
        <Button variant="ghost" aria-label="关闭详情" onClick={close}>
          <X aria-hidden="true" />
        </Button>
      </header>
      {detail.isPending ? (
        <ConsoleState kind="loading" title="正在读取详情…" />
      ) : detail.isError ? (
        <ErrorState error={detail.error} retry={() => void detail.refetch()} />
      ) : detail.data ? (
        <div className={styles.detailBody}>
          <p className={styles.coverage}>
            取得于 {timestamp(detail.data.observedAt)}。
            {kind === "orders"
              ? detail.data.stale
                ? "平台详情暂不可用，以下为上次保存的陈旧观察；包裹与物流须待平台详情恢复后重新核对。"
                : "当前详情来自平台只读查询；列表保留原同步观察。"
              : "库存和价格按平台原维度展示，不推算未提供字段。"}
          </p>
          {detail.data.stale ? (
            <Button
              size="sm"
              variant="outline"
              disabled={detail.isFetching}
              onClick={() => void detail.refetch()}
            >
              重试平台详情
            </Button>
          ) : null}
          {detail.data.product?.skcs.map((skc) => (
            <section key={skc.id}>
              <h3>{skc.title || skc.id}</h3>
              <p>
                SKC {skc.id} · {skc.site || "站点未提供"}
              </p>
              {skc.skus.map((sku) => (
                <Card className={styles.sku} key={sku.id}>
                  <h4>SKU {sku.id}</h4>
                  <p>卖家 SKU：{sku.sellerSku || "平台未提供"}</p>
                  <dl>
                    <dt>售价</dt>
                    <dd>{priceList(sku.prices, false)}</dd>
                    <dt>供货价</dt>
                    <dd>{priceList(sku.costs, false)}</dd>
                    <dt>库存</dt>
                    <dd>
                      {sku.inventory.length ? (
                        <ul>
                          {sku.inventory.map((w, i) => (
                            <li key={`${w.warehouseId}:${i}`}>
                              仓库 {w.warehouseId}：
                              {w.quantity === null ? "平台未提供" : w.quantity}
                            </li>
                          ))}
                        </ul>
                      ) : (
                        "平台未提供"
                      )}
                    </dd>
                  </dl>
                </Card>
              ))}
            </section>
          ))}
          {order ? (
            <>
              <div className={styles.actions}>
                <PlatformLink />
                <Button
                  variant="outline"
                  onClick={() =>
                    void navigator.clipboard
                      .writeText(order.id)
                      .then(() => setCopied(true))
                      .catch(() => setCopied(false))
                  }
                >
                  <Copy aria-hidden="true" />
                  {copied ? "已复制订单号" : "复制订单号"}
                </Button>
              </div>
              <p>发货、拆包、售后与退款请到 SHEIN 平台完成。</p>
              <dl className={styles.facts}>
                <dt>平台状态</dt>
                <dd>{orderStatus(order.status)}</dd>
                <dt>订单金额</dt>
                <dd>{price(order.amount)}</dd>
                <dt>供货价</dt>
                <dd>{price(order.supplyCost)}</dd>
                <dt>下单时间</dt>
                <dd>{timestamp(order.createdAt)}</dd>
                <dt>列表下发时间</dt>
                <dd>{timestamp(order.issuedAt)}</dd>
                <dt>平台更新时间</dt>
                <dd>{timestamp(order.updatedAt)}</dd>
                <dt>发货时限</dt>
                <dd>{timestamp(order.needDeliveryAt)}</dd>
                <dt>交接时限</dt>
                <dd>{timestamp(order.handoverAt)}</dd>
                <dt>预计揽收</dt>
                <dd>{timestamp(order.expectedCollectAt)}</dd>
                <dt>库存模式</dt>
                <dd>
                  {order.stockMode === 1
                    ? "SHEIN 自营库存"
                    : order.stockMode === 2
                      ? "SHEIN 仓履行"
                      : order.stockMode === 3
                        ? "商家仓"
                        : order.stockMode === null
                          ? "平台未提供"
                          : `未识别 (${order.stockMode})`}
                </dd>
                <dt>订单类型</dt>
                <dd>
                  {order.type === 1
                    ? "正常订单"
                    : order.type === 2
                      ? "换货订单"
                      : order.type === 4
                        ? "认证仓转自发货"
                        : order.type === 5
                          ? "认证仓订单"
                          : order.type === null
                            ? "平台未提供"
                            : `未识别 (${order.type})`}
                </dd>
              </dl>
              <section>
                <h3>平台异常</h3>
                {orderProblems(order).length ? (
                  <ul>
                    {orderProblems(order).map((p, i) => (
                      <li key={`${p}:${i}`}>{p}</li>
                    ))}
                  </ul>
                ) : (
                  <p>当前详情没有平台异常标签。</p>
                )}
              </section>
              <section>
                <h3>商品单件记录</h3>
                {order.items.map((it) => (
                  <Card key={it.id} className={styles.sku}>
                    <strong>{it.title || "平台未提供标题"}</strong>
                    <p>
                      单件 ID {it.id} · SKU {it.sku || "未提供"} · 卖家 SKU{" "}
                      {it.sellerSku || "未提供"}
                    </p>
                    <p>
                      {orderStatus(it.status)} · 换货标识{" "}
                      {it.exchangeTag ?? "平台未提供"}
                    </p>
                  </Card>
                ))}
              </section>
              <section>
                <h3>包裹与物流</h3>
                {order.packages
                  .filter((p) => !p.id)
                  .map((p, i) => (
                    <p key={`unknown:${i}`}>
                      包裹号尚未提供，暂不能查询该包裹物流。
                      {p.carrier ? `承运商：${p.carrier}。` : ""}
                      {p.waybill ? `运单：${p.waybill}。` : ""}
                      {p.label ? `标签：${p.label}。` : ""}
                    </p>
                  ))}
                {trackable.length ? (
                  <>
                    <label>
                      选择包裹
                      <Select
                        aria-label="选择包裹"
                        value={packageID}
                        onChange={(e) => setPkg(e.target.value)}
                      >
                        {trackable.map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.id} · {p.waybill || "运单尚未提供"}
                          </option>
                        ))}
                      </Select>
                    </label>
                    {detail.data.stale ? (
                      <p>平台详情恢复后可查询当前包裹物流。</p>
                    ) : tracks.isPending ? (
                      <p role="status">正在查询物流…</p>
                    ) : tracks.isError ? (
                      <ErrorState
                        error={tracks.error}
                        retry={() => void tracks.refetch()}
                      />
                    ) : tracks.data?.some((t) => t.nodes.length) ? (
                      tracks.data.map((t, i) => (
                        <div key={`${t.waybill}:${i}`}>
                          <p>
                            {t.carrier || "承运商未提供"} ·{" "}
                            {t.waybill || "运单未提供"}
                          </p>
                          <ol className={styles.timeline}>
                            {t.nodes.map((n, j) => (
                              <li key={`${n.atMillis}:${j}`}>
                                <time>
                                  {n.atMillis
                                    ? timestamp(
                                        new Date(n.atMillis).toISOString(),
                                      )
                                    : "时间未提供"}
                                </time>
                                <strong>
                                  {n.name || n.code || "平台物流节点"}
                                </strong>
                                <p>{n.description}</p>
                              </li>
                            ))}
                          </ol>
                        </div>
                      ))
                    ) : (
                      <p>平台尚未提供物流轨迹。</p>
                    )}
                  </>
                ) : !order.packages.length ? (
                  <p>平台尚未提供包裹，暂不能查询物流。</p>
                ) : null}
              </section>
            </>
          ) : null}
        </div>
      ) : null}
    </dialog>
  );
}
function PlatformLink() {
  return (
    <a
      className={styles.platformLink}
      href={platformURL}
      target="_blank"
      rel="noopener noreferrer"
    >
      <ExternalLink size={14} aria-hidden="true" />到 SHEIN 处理
    </a>
  );
}
function Thumbnail({ src }: { src: string }) {
  return src ? (
    <Image
      src={src}
      alt=""
      width={48}
      height={48}
      unoptimized
      className={styles.thumbnail}
      loading="lazy"
      referrerPolicy="no-referrer"
    />
  ) : (
    <span className={styles.thumbnail} aria-hidden="true">
      <ShoppingBag size={20} />
    </span>
  );
}
function price(p: ObservationPrice | null | undefined) {
  return p && p.value !== ""
    ? `${p.currency || "币种未提供"} ${p.value}${p.special ? `（促销 ${p.special}）` : ""}`
    : "平台未提供";
}
function priceList(prices: ObservationPrice[], compact = true) {
  const values = [...new Set(prices.map(price))];
  return values.length
    ? `${(compact ? values.slice(0, 3) : values).join(" / ")}${compact && values.length > 3 ? " · 更多见详情" : ""}`
    : "平台未提供";
}
function timestamp(value: string) {
  if (!value) return "平台未提供";
  return (
    new Intl.DateTimeFormat("zh-CN", {
      timeZone: "Asia/Shanghai",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(new Date(value)) + " UTC+8"
  );
}
function coverageNote(code: string) {
  const notes: Record<string, string> = {
    product_site_unknown: "商品站点或状态未提供",
    product_count_changed: "平台计数变化",
    product_count_mismatch: "取得数量与平台计数不符",
    duplicate_platform_identity: "分页成员重复",
    order_site_missing: "部分订单未提供站点",
    order_window_saturated: "订单时间区间达到平台上限",
    duration_limit: "达到同步时长上限",
    page_limit: "达到页数上限",
  };
  return notes[code] ?? code;
}
function ErrorState({ error, retry }: { error: unknown; retry: () => void }) {
  const code =
    error instanceof ObservationError ? error.code : "DEPENDENCY_UNAVAILABLE";
  const message =
    code === "PERMISSION_DENIED"
      ? "没有查看当前企业数据的权限"
      : code === "REVISION_CONFLICT"
        ? "同步范围已变化，请从第一页刷新"
        : code === "NOT_FOUND"
          ? "当前店铺、数据来源或同步已不可访问"
          : code.includes("CONTEXT_CHANGED")
            ? "企业或身份已变化，请重新进入页面"
            : "平台数据暂时不可用";
  return (
    <ConsoleState kind="error" title={message}>
      <Button className="mt-3" variant="outline" onClick={retry}>
        <RefreshCw aria-hidden="true" />
        重试
      </Button>
    </ConsoleState>
  );
}
