"use client";
import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { useResourcePending } from "../resources/resource-pending";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  downloadTool,
  toolRequest,
  ToolMarketError,
  type ToolScope,
} from "@/lib/api/tool-market";
import {
  marketSchema,
  requestsSchema,
  detailSchema,
  toolReceipt,
  toolVersion,
  toolEndpoint,
  demandInput,
  progressInput,
  type Tool,
  type ToolRequest,
} from "@/lib/contracts/tool-market";
import styles from "./tools.module.css";

type Mode = "official" | "mine" | "custom" | "admin";
const stageNames: Record<string, string> = {
  SUBMITTED: "待评估",
  EVALUATING: "需求评估",
  PLAN_CONFIRMED: "方案确认",
  DEVELOPING: "开发联调",
  DELIVERED: "专员已记录交付",
  CLOSED: "已关闭",
};
const kindNames: Record<string, string> = {
  DATA: "数据采集",
  CONNECTION: "系统连接",
  AUTOMATION: "自动化流程",
  OUTPUT: "输出与权限",
};
function errorText(e: unknown) {
  const code = e instanceof ToolMarketError ? e.code : "";
  return (
    (
      {
        FORBIDDEN: "当前身份没有操作权限。",
        ORGANIZATION_CONTEXT_CHANGED: "企业已变化，请恢复原企业后确认原操作。",
        IDENTITY_CONTEXT_CHANGED: "登录身份已变化，请恢复原身份后确认原操作。",
        REVISION_MISMATCH: "状态已被更新，请刷新后再次确认。",
        NOT_FOUND: "此需求不属于当前企业，或已不可用。",
        INVALID_REQUEST: "请检查填写内容。",
        IDEMPOTENCY_CONFLICT:
          "原请求编号与内容不一致，请保留原操作并联系平台处理。",
        OUTCOME_UNKNOWN: "结果尚未确认，请重试原操作读取真实回执。",
      } as Record<string, string>
    )[code] ??
    (e instanceof Error && !code ? e.message : "当前能力暂不可用，请稍后重试。")
  );
}
const pendingSchema = z
  .strictObject({
    path: z.string().max(256),
    key: z.uuid(),
    body: z.string().max(16384),
    revision: toolVersion.optional(),
    absent: z.boolean().optional(),
  })
  .refine((i) => {
    const r = toolEndpoint(
      new URL("/api/tool-market/" + i.path, "https://pending.invalid"),
      i.path.startsWith("activations/") ? "PUT" : "POST",
    );
    if (
      !r?.input ||
      !r.operation ||
      (i.absent && i.revision) ||
      (!i.absent && !i.revision) ||
      (r.operation === "create" && !i.absent) ||
      (r.operation === "progress" && i.absent)
    )
      return false;
    try {
      return r.input.safeParse(JSON.parse(i.body)).success;
    } catch {
      return false;
    }
  });
type Intent = z.infer<typeof pendingSchema>;
function useCommands(scope: ToolScope, admin: boolean) {
  const context = useWorkbenchContext(),
    live = useRef(context),
    client = useQueryClient(),
    running = useRef(false),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  useEffect(() => {
    live.current = context;
  }, [context]);
  const pending = useResourcePending(
    {
      expectedUserId: scope.userId,
      expectedOrganizationId: scope.organizationId,
    },
    ["tool-market", admin ? "platform" : "enterprise"],
    pendingSchema,
    { maxLength: 32768 },
  );
  useEffect(
    () =>
      context.registerOrganizationSwitchGuard(
        () => !running.current && !busy && !pending.error,
      ),
    [context, busy, pending.error],
  );
  async function execute(i: Intent) {
    try {
      if (live.current.user?.id !== scope.userId)
        throw new ToolMarketError("IDENTITY_CONTEXT_CHANGED");
      if (
        live.current.error ||
        live.current.blockingError ||
        live.current.isLoading ||
        live.current.isSwitching ||
        (!admin &&
          live.current.effectiveOrganization?.id !== scope.organizationId)
      )
        throw new ToolMarketError("ORGANIZATION_CONTEXT_CHANGED");
      if (running.current || !pending.ready)
        throw new Error("当前操作尚未确认");
      const command = pendingSchema.parse(i);
      if (command.path.startsWith("admin/") !== admin)
        throw new ToolMarketError("FORBIDDEN");
      pending.persist(command);
      running.current = true;
      setBusy(true);
      setMessage("");
      const headers = new Headers({
        "Content-Type": "application/json",
        "Idempotency-Key": command.key,
      });
      if (command.absent) headers.set("If-None-Match", "*");
      else headers.set("If-Match", '"' + command.revision + '"');
      const receipt = await toolRequest(scope, command.path, toolReceipt, {
        method: command.path.startsWith("activations/") ? "PUT" : "POST",
        headers,
        body: command.body,
      });
      if (receipt.commandId !== command.key)
        throw new ToolMarketError("OUTCOME_UNKNOWN");
      pending.clear(command);
      setMessage("已保存，正在读取真实状态。");
      await client.invalidateQueries({
        queryKey: ["tool-market", scope.userId, scope.organizationId],
      });
      return receipt;
    } catch (e) {
      setMessage(errorText(e));
      if (
        e instanceof ToolMarketError &&
        ((e.status === 400 && e.code === "INVALID_REQUEST") ||
          (e.status === 404 && e.code === "NOT_FOUND") ||
          (e.status === 412 && e.code === "REVISION_MISMATCH"))
      ) {
        try {
          pending.clear(i);
        } catch {}
      }
      throw e;
    } finally {
      running.current = false;
      setBusy(false);
    }
  }
  const locked = !pending.ready || !!pending.command || busy || pending.error;
  const notice =
    message || pending.command || pending.error ? (
      <Card role="status" className={styles.notice}>
        <p>
          {pending.error
            ? "无法读取待确认操作，已暂停新提交。请保留浏览器数据并联系平台处理。"
            : message || "原操作尚未确认，请重试原操作。"}
        </p>
        {pending.command && !busy && !pending.error ? (
          <Button
            variant="outline"
            onClick={() =>
              void execute(pending.command!).catch(() => undefined)
            }
          >
            重试原操作
          </Button>
        ) : null}
      </Card>
    ) : null;
  return { locked, notice, execute, message };
}
export function ToolPage({ mode }: { mode: Mode }) {
  const c = useWorkbenchContext();
  if (c.error || c.blockingError)
    return (
      <ConsoleState kind="error" title="身份上下文不可用">
        <Button onClick={() => void c.retry()}>重新确认</Button>
      </ConsoleState>
    );
  if (c.isLoading || c.isSwitching)
    return <ConsoleState kind="loading" title="正在确认当前身份" />;
  if (
    !c.user ||
    (mode !== "admin" && (!c.effectiveOrganization || c.selectionRequired))
  )
    return (
      <ConsoleState
        kind="unavailable"
        title={mode === "admin" ? "请先登录专员身份" : "请先选择企业"}
      />
    );
  const scope = {
    userId: c.user.id,
    organizationId: mode === "admin" ? "" : c.effectiveOrganization!.id,
  };
  return (
    <ScopedPage
      key={JSON.stringify([scope, mode, c.permissions])}
      mode={mode}
      scope={scope}
    />
  );
}
function ScopedPage({ mode, scope }: { mode: Mode; scope: ToolScope }) {
  const admin = mode === "admin",
    commands = useCommands(scope, admin),
    [filter, setFilter] = useState("全部"),
    [view, setView] = useState<"landing" | "list" | "submit">("landing"),
    [cursor, setCursor] = useState(""),
    [selected, setSelected] = useState("");
  const market = useQuery({
    queryKey: [
      "tool-market",
      scope.userId,
      scope.organizationId,
      mode === "mine" ? "mine" : "market",
    ],
    queryFn: ({ signal }) =>
      toolRequest(scope, mode === "mine" ? "mine" : "market", marketSchema, {
        signal,
      }),
    enabled: !admin,
  });
  const requests = useQuery({
    queryKey: [
      "tool-market",
      scope.userId,
      scope.organizationId,
      "requests",
      cursor,
    ],
    queryFn: ({ signal }) =>
      toolRequest(
        scope,
        (admin ? "admin/" : "") +
          "requests" +
          (cursor ? "?cursor=" + cursor : ""),
        requestsSchema,
        { signal },
      ),
    enabled: admin || (mode === "custom" && view === "list"),
  });
  const title =
    mode === "official"
      ? "官方工具"
      : mode === "mine"
        ? "我的工具"
        : admin
          ? "工具定制 · 专员处理"
          : "工具定制";
  const actions =
    mode === "official" ? (
      <Link href="/workbench/tools/mine">查看我的工具 →</Link>
    ) : mode === "mine" ? (
      <Link href="/workbench/tools/official">前往官方工具</Link>
    ) : admin ? (
      <Link href="/workbench/tools/custom">返回工具定制</Link>
    ) : (
      <>
        <Button
          variant="outline"
          onClick={() => setView("list")}
          disabled={commands.locked}
        >
          查看定制进度
        </Button>
        <Button
          onClick={() => setView("submit")}
          disabled={commands.locked || !market.data?.canCustomize}
        >
          提交定制需求
        </Button>
      </>
    );
  function activate(t: Tool, enabled: boolean) {
    return commands.execute({
      path: "activations/" + t.id,
      key: crypto.randomUUID(),
      body: JSON.stringify({ enabled }),
      ...(t.activation
        ? { revision: t.activation.revision }
        : { absent: true }),
    });
  }
  return (
    <ConsolePage
      className={styles.page}
      title={title}
      description={
        mode === "official"
          ? "启用官方能力，让工具服务当前业务。"
          : mode === "mine"
            ? "当前企业共用的启用清单；使用仍需对应业务权限。"
            : admin
              ? "平台授权独立核实。报价付款线下办理，进度由专员据实记录。"
              : "已有工具无法覆盖你的业务？提交需求，由硕米专员评估。"
      }
      breadcrumbs={[{ label: "工具市场" }, { label: title }]}
      actions={actions}
    >
      {commands.notice}
      {!admin && market.isPending ? (
        <ConsoleState kind="loading" title="正在读取企业工具" />
      ) : !admin && market.error ? (
        <ConsoleState kind="error" title={errorText(market.error)}>
          <Button onClick={() => void market.refetch()}>重新读取</Button>
        </ConsoleState>
      ) : null}
      {mode === "official" && market.data ? (
        <>
          <div className={styles.filters} role="group" aria-label="工具分类">
            {["全部", "采集", "图片", "环境", "数据", "流程", "其他"].map(
              (f) => (
                <Button
                  key={f}
                  variant="outline"
                  aria-pressed={f === filter}
                  onClick={() => setFilter(f)}
                >
                  {f === filter ? (
                    <span className={styles.dot} aria-hidden="true" />
                  ) : null}
                  {f}
                </Button>
              ),
            )}
          </div>
          <div className={styles.cards}>
            {market.data.tools
              .filter((t) => filter === "全部" || t.category === filter)
              .map((t) => (
                <Card key={t.id} className={styles.toolCard}>
                  <div className={styles.cardHeading}>
                    <h2>{t.name}</h2>
                  </div>
                  <p>{t.description}</p>
                  <small>
                    {t.category} ·{" "}
                    {t.id === "product-acquisition" ? "我的数据" : "待开放"}
                  </small>
                  <div className={styles.cardActions}>
                    {t.status === "AVAILABLE" ? (
                      <Button
                        disabled={
                          commands.locked ||
                          !market.data.canManage ||
                          !!t.activation?.enabled
                        }
                        onClick={() =>
                          void activate(t, true).catch(() => undefined)
                        }
                      >
                        {t.activation?.enabled ? "已启用" : "立即启用"}
                      </Button>
                    ) : null}
                    {t.activation?.enabled ? (
                      <Link href="/workbench/tools/mine">使用工具 →</Link>
                    ) : null}
                    <span
                      className={styles.badge}
                      data-ready={t.status === "AVAILABLE"}
                    >
                      {t.status === "AVAILABLE"
                        ? "已开放"
                        : t.status === "DEVELOPING"
                          ? "开发中"
                          : "暂不可用"}
                    </span>
                  </div>
                </Card>
              ))}
          </div>
          {!market.data.canManage ? (
            <p className={styles.muted}>请由当前企业管理员启用工具。</p>
          ) : null}
        </>
      ) : null}
      {mode === "mine" && market.data ? (
        <div className={styles.mineGrid}>
          <div>
            <p>已启用 {market.data.tools.length} 个工具</p>
            {!market.data.tools.length ? (
              <ConsoleState kind="empty" title="当前企业尚未启用工具">
                <Link href="/workbench/tools/official">前往官方工具</Link>
              </ConsoleState>
            ) : (
              market.data.tools.map((t) => (
                <Card key={t.id} className={styles.mineCard}>
                  <div className={styles.cardHeading}>
                    <h2>{t.name}</h2>
                    {market.data.canManage ? (
                      <Button
                        variant="outline"
                        disabled={commands.locked}
                        onClick={() =>
                          void activate(t, false).catch(() => undefined)
                        }
                      >
                        从清单停用
                      </Button>
                    ) : null}
                  </div>
                  <p>采集1688商品资料，保存到当前企业。</p>
                  {t.status !== "AVAILABLE" ? (
                    <p role="status">{t.reason}</p>
                  ) : null}
                  <div className={styles.channels}>
                    <div>
                      <h3>本地插件采集</h3>
                      <span className={styles.free}>
                        免费 · 不消耗 DATA_ROW
                      </span>
                      <p>
                        在 Edge 或 Chrome 中采集当前1688商品，回到应用确认导入。
                      </p>
                      {t.localCapture ? (
                        <>
                          <DownloadButton scope={scope} enabled={t.download} />
                          <Link href="/capture/1688">打开插件接收页</Link>
                        </>
                      ) : (
                        <p role="status">本地插件采集尚未开放</p>
                      )}
                    </div>
                    <div>
                      <h3>在线采集</h3>
                      <span className={styles.paid}>成功采集消耗 DATA_ROW</span>
                      <p>
                        使用当前企业授权账号采集1688商品；计价与额度由采集页面确认。
                      </p>
                      {t.onlineCapture ? (
                        <Link
                          className={styles.primaryLink}
                          href="/workbench/supply/acquisition"
                        >
                          在线采集 →
                        </Link>
                      ) : (
                        <Button disabled>当前安装未开放在线采集</Button>
                      )}
                    </div>
                  </div>
                  <p className={styles.muted}>
                    停用只移出企业清单；已安装插件和已开始的采集按原权限继续处理。
                  </p>
                </Card>
              ))
            )}
          </div>
          <Card className={styles.side}>
            <h2>使用说明</h2>
            <ol>
              <li>下载压缩包并解压到固定文件夹。</li>
              <li>打开 Edge 扩展页面或 Chrome 扩展管理，启用开发者模式。</li>
              <li>选择“加载解压缩的扩展”，选中刚才的文件夹。</li>
              <li>打开1688商品详情页，点击插件采集，返回接收页确认保存。</li>
            </ol>
            <p>
              插件只采集你主动选择的商品资料，请勿在定制需求中提交账号密码或凭据。
            </p>
            <Link href="/workbench/data/mine">查看我的数据 →</Link>
          </Card>
        </div>
      ) : null}
      {mode === "custom" && view === "landing" ? (
        <>
          <Card className={styles.intro}>
            <h2>让工具适配你的业务</h2>
            <p>
              说明你的操作场景、数据来源和期望结果。硕米专员人工评估方案与交付范围，线下报价付款。
            </p>
            <Button
              disabled={commands.locked || !market.data?.canCustomize}
              onClick={() => setView("submit")}
            >
              提交定制需求
            </Button>
          </Card>
          <div className={styles.cards}>
            {Object.entries(kindNames).map(([k, n]) => (
              <Card key={k} className={styles.customCard}>
                <h2>{n}</h2>
                <p>
                  {
                    (
                      {
                        DATA: "指定来源、字段与保存方式",
                        CONNECTION: "连接已有业务系统与服务",
                        AUTOMATION: "减少重复操作与人工传递",
                        OUTPUT: "明确结果格式、使用者与权限",
                      } as Record<string, string>
                    )[k]
                  }
                </p>
              </Card>
            ))}
          </div>
          <h2 className={styles.sectionTitle}>定制流程</h2>
          <div className={styles.steps}>
            {["提交需求", "需求评估", "方案确认", "开发联调", "交付使用"].map(
              (n, i) => (
                <Card key={n}>
                  <span>0{i + 1}</span>
                  <h3>{n}</h3>
                  <p>
                    {
                      [
                        "保存需求，等待专员联系",
                        "人工评估范围与可行性",
                        "线下确认方案与报价",
                        "按确认范围开发与联调",
                        "实际交付另行核实",
                      ][i]
                    }
                  </p>
                </Card>
              ),
            )}
          </div>
          <Link
            href="/workbench/tools/custom/review"
            className={styles.specialist}
          >
            硕米专员处理入口 →
          </Link>
        </>
      ) : null}
      {mode === "custom" && view === "submit" ? (
        <DemandForm
          locked={commands.locked || !market.data?.canCustomize}
          submit={async (body) => {
            const r = await commands.execute({
              path: "requests",
              key: crypto.randomUUID(),
              body: JSON.stringify(body),
              absent: true,
            });
            setSelected(r!.id);
            setView("list");
          }}
          cancel={() => setView("landing")}
        />
      ) : null}
      {admin || (mode === "custom" && view === "list") ? (
        <>
          <Card className={styles.requestList}>
            <h2>{admin ? "所有企业的定制需求" : "当前企业的定制需求"}</h2>
            {requests.isPending ? (
              <p role="status">正在读取需求与权限</p>
            ) : requests.error ? (
              <p role="alert">
                {errorText(requests.error)}{" "}
                <Button onClick={() => void requests.refetch()}>
                  重新读取
                </Button>
              </p>
            ) : (
              <>
                {!requests.data?.items.length ? (
                  <p>暂无定制需求。</p>
                ) : (
                  requests.data.items.map((r) => (
                    <button
                      className={styles.requestRow}
                      key={r.id}
                      onClick={() => setSelected(r.id)}
                    >
                      <span>
                        {r.title}
                        <small>
                          {kindNames[r.kind]}
                          {admin ? " · 企业 " + r.organizationId : ""}
                        </small>
                      </span>
                      <span>{stageNames[r.stage]} →</span>
                    </button>
                  ))
                )}
                <div className={styles.cardActions}>
                  <Button
                    variant="outline"
                    disabled={!cursor}
                    onClick={() => setCursor("")}
                  >
                    返回第一页
                  </Button>
                  <Button
                    variant="outline"
                    disabled={!requests.data?.nextCursor}
                    onClick={() => setCursor(requests.data!.nextCursor)}
                  >
                    下一页
                  </Button>
                </div>
              </>
            )}
          </Card>
          {selected ? (
            <RequestDetail
              key={selected}
              scope={scope}
              id={selected}
              admin={admin}
              commands={commands}
            />
          ) : null}
        </>
      ) : null}
    </ConsolePage>
  );
}
function DownloadButton({
  scope,
  enabled,
}: {
  scope: ToolScope;
  enabled: boolean;
}) {
  const [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  return (
    <>
      <Button
        disabled={!enabled || busy}
        onClick={() => {
          setBusy(true);
          setMessage("");
          void downloadTool(scope)
            .catch((e) => setMessage(errorText(e)))
            .finally(() => setBusy(false));
        }}
      >
        {busy ? "正在下载" : enabled ? "下载插件" : "当前安装未配置插件包"}
      </Button>
      {message ? <p role="alert">{message}</p> : null}
    </>
  );
}
function DemandForm({
  locked,
  submit,
  cancel,
}: {
  locked: boolean;
  submit: (v: z.infer<typeof demandInput>) => Promise<void>;
  cancel: () => void;
}) {
  const [kind, setKind] = useState("DATA"),
    [title, setTitle] = useState(""),
    [description, setDescription] = useState(""),
    [error, setError] = useState("");
  const send = (e: FormEvent) => {
    e.preventDefault();
    const result = demandInput.safeParse({ kind, title, description });
    if (!result.success) {
      setError("请填写120字以内的标题与4000字以内的需求说明。");
      return;
    }
    setError("");
    void submit(result.data).catch((e) => setError(errorText(e)));
  };
  return (
    <Card className={styles.form}>
      <h2>提交工具定制需求</h2>
      <p>
        提交后保存为待评估。报价与付款线下办理，请勿填写密码、token或账号凭据。
      </p>
      <form onSubmit={send}>
        <fieldset disabled={locked}>
          <label>
            需求类型
            <select value={kind} onChange={(e) => setKind(e.target.value)}>
              {Object.entries(kindNames).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </select>
          </label>
          <label>
            需求标题
            <Input
              required
              maxLength={120}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </label>
          <label>
            需求说明
            <textarea
              required
              maxLength={4000}
              rows={8}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="说明使用者、当前操作、数据来源和希望得到的结果"
            />
          </label>
          <div className={styles.cardActions}>
            <Button type="submit">保存需求，等待评估</Button>
            <Button type="button" variant="outline" onClick={cancel}>
              返回
            </Button>
          </div>
        </fieldset>
        {error ? <p role="alert">{error}</p> : null}
      </form>
    </Card>
  );
}
function nextStages(r: ToolRequest) {
  const order = [
    "SUBMITTED",
    "EVALUATING",
    "PLAN_CONFIRMED",
    "DEVELOPING",
    "DELIVERED",
  ];
  const next = order[order.indexOf(r.stage) + 1];
  return [r.stage, ...(next ? [next] : []), "CLOSED"];
}
function RequestDetail({
  scope,
  id,
  admin,
  commands,
}: {
  scope: ToolScope;
  id: string;
  admin: boolean;
  commands: ReturnType<typeof useCommands>;
}) {
  const detail = useInfiniteQuery({
    queryKey: ["tool-market", scope.userId, scope.organizationId, "detail", id],
    initialPageParam: "",
    queryFn: ({ signal, pageParam }) =>
      toolRequest(
        scope,
        (admin ? "admin/" : "") +
          "requests/" +
          id +
          (pageParam ? "?eventsBefore=" + pageParam : ""),
        detailSchema,
        { signal },
      ),
    getNextPageParam: (page) => page.nextEventsBefore || undefined,
  });
  const [stage, setStage] = useState(""),
    [note, setNote] = useState("");
  if (detail.isPending)
    return <ConsoleState kind="loading" title="正在读取原需求与进度" />;
  const historyUnavailable =
    detail.isFetchNextPageError &&
    detail.error instanceof ToolMarketError &&
    detail.error.status >= 500;
  if ((detail.error && !historyUnavailable) || !detail.data)
    return <ConsoleState kind="error" title={errorText(detail.error)} />;
  const r = detail.data.pages[0].request;
  const events = detail.data.pages.slice().reverse().flatMap((page) => page.events);
  return (
    <Card className={styles.detail}>
      <h2>{r.title}</h2>
      <p>
        {kindNames[r.kind]} · {stageNames[r.stage]}
      </p>
      <p className={styles.description}>{r.description}</p>
      <h3>专员记录的进度</h3>
      <ol>
        {events.map((e) => (
          <li key={e.revision}>
            <span>
              {stageNames[e.stage]} ·{" "}
              {new Date(e.occurredAt).toLocaleString("zh-CN")}
            </span>
            <p>{e.note}</p>
          </li>
        ))}
      </ol>
      {detail.hasNextPage ? (
        <Button
          variant="outline"
          disabled={detail.isFetching}
          onClick={() => void detail.fetchNextPage()}
        >
          {detail.isFetchingNextPage ? "正在读取更早进度" : "加载更早进度"}
        </Button>
      ) : null}
      {detail.isFetchNextPageError ? (
        <p role="alert">更早进度暂时无法读取，请重试；已读取进度仍可查看。</p>
      ) : null}
      <p>进度记录不代表付款或工具已安装。实际工具交付及开放另行核实。</p>
      {admin && !["DELIVERED", "CLOSED"].includes(r.stage) ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const value = progressInput.safeParse({
              stage: stage || r.stage,
              note,
            });
            if (!value.success) return;
            void commands
              .execute({
                path: "admin/requests/" + id + "/progress",
                key: crypto.randomUUID(),
                body: JSON.stringify(value.data),
                revision: r.revision,
              })
              .then(() => {
                setStage("");
                setNote("");
              })
              .catch(() => undefined);
          }}
        >
          <fieldset disabled={commands.locked}>
            <label>
              处理阶段
              <select
                value={stage || r.stage}
                onChange={(e) => setStage(e.target.value)}
              >
                {nextStages(r).map((s) => (
                  <option key={s} value={s}>
                    {s === r.stage ? "追加本阶段进度" : stageNames[s]}
                  </option>
                ))}
              </select>
            </label>
            <label>
              真实处理进度 / 关闭原因
              <textarea
                required
                maxLength={2000}
                rows={4}
                value={note}
                onChange={(e) => setNote(e.target.value)}
              />
            </label>
            <Button type="submit" disabled={!note.trim()}>
              记录进度
            </Button>
          </fieldset>
        </form>
      ) : null}
    </Card>
  );
}
