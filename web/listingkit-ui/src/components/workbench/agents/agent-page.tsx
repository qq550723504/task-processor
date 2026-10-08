"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ConsolePage, ConsoleState } from "../console/console-page";
import {
  configurationRequest,
  ConfigurationError,
  type ConfigurationScope,
} from "@/lib/api/agent-configuration";
import {
  catalogEntrySchema,
  catalogPageSchema,
  configReceiptSchema,
  recentRunsSchema,
  templateSchema,
  templatesPageSchema,
  configVersion,
  type AgentTemplate,
  type CatalogEntry,
} from "@/lib/contracts/agent-configuration";
import { basesSchema, knowledgeRequest } from "@/lib/api/knowledge";
import styles from "./agents.module.css";

const agentId = "product.title.agent";
const labels: Record<string, string> = {
  NOT_ENABLED: "尚未启用",
  ENABLED: "已启用",
  DISABLED: "已停用",
  AVAILABLE: "可用",
  NEEDS_CONFIGURATION: "需要配置",
  REQUIRES_AUTHORIZATION: "需要授权",
  UNAVAILABLE: "不可用",
  NOT_SUPPORTED: "不支持",
  REQUIRED: "必需",
  OPTIONAL: "可选",
  running: "执行中",
  interrupted: "等待继续",
  human_review_required: "待人工审核",
  stopped: "已停止",
};
const capabilityNames: Record<string, string> = {
  "text.generate": "文本生成",
  "product.source-evidence": "采集证据读取",
  "knowledge.context": "企业知识引用",
  "image.generate": "图片生成",
  "platform.write": "平台写入",
};
function configurationError(error: unknown) {
  const code =
    error instanceof ConfigurationError ? error.code : "DEPENDENCY_UNAVAILABLE";
  return (
    (
      {
        FORBIDDEN: "当前身份没有操作权限。",
        IDENTITY_CONTEXT_CHANGED: "登录身份已变化，请重新确认。",
        ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化，请重新确认。",
        REVISION_MISMATCH: "配置已被更新，请刷新当前状态后再次确认。",
        TEMPLATE_IS_DEFAULT: "此模板仍是企业默认，请先清除或替换默认模板。",
        TEMPLATE_ARCHIVED: "模板已归档，无法再用于新执行。",
        IDEMPOTENCY_CONFLICT: "本次请求编号已用于其他内容，请刷新后重新确认。",
        CONFIGURATION_CHANGED:
          "配置准入已变化，请重新确认；不会自动发起新执行。",
        AGENT_NOT_ENABLED: "请先由管理员在当前企业启用智能体。",
        OUTCOME_UNKNOWN: "操作结果尚未确认，请重试同一次操作以读取原回执。",
        INVALID_REQUEST: "请检查名称、平台、知识选择和版本。",
        NOT_FOUND: "该配置在当前企业不可用。",
      } as Record<string, string>
    )[code] ?? "当前能力或依赖不可用，请稍后重试。"
  );
}

function useRead<T>(
  scope: ConfigurationScope,
  path: string,
  schema: z.ZodType<T>,
  nonce = 0,
  enabled = true,
) {
  const { userId, organizationId } = scope;
  const [result, setResult] = useState<{
    path: string;
    data?: T;
    error?: unknown;
  }>({ path: "" });
  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    configurationRequest({ userId, organizationId }, path, schema, {
      signal: controller.signal,
    })
      .then((data) => {
        if (!controller.signal.aborted)
          setResult({ path: `${path}:${nonce}`, data });
      })
      .catch((error) => {
        if (!controller.signal.aborted)
          setResult({ path: `${path}:${nonce}`, error });
      });
    return () => controller.abort();
  }, [userId, organizationId, path, schema, nonce, enabled]);
  return result.path === `${path}:${nonce}` && enabled ? result : { path };
}
type Intent = {
  path: string;
  payload: unknown;
  method: "POST" | "PUT";
  key: string;
  revision?: string;
  absent?: boolean;
};
export function AgentPage({
  mode,
  id,
}: {
  mode: "market" | "mine";
  id?: string;
}) {
  const ctx = useWorkbenchContext();
  if (ctx.error || ctx.blockingError)
    return (
      <ConsoleState kind="error" title="企业上下文不可用">
        <Button onClick={() => void ctx.retry()}>重新确认</Button>
      </ConsoleState>
    );
  if (ctx.isLoading || ctx.isSwitching)
    return <ConsoleState kind="loading" title="正在确认当前企业" />;
  if (!ctx.user || !ctx.effectiveOrganization || ctx.selectionRequired)
    return <ConsoleState kind="unavailable" title="请先选择企业" />;
  return (
    <ScopedPage
      key={`${ctx.user.id}:${ctx.effectiveOrganization.id}:${[...ctx.permissions].sort().join(":")}:${mode}:${id ?? ""}`}
      scope={{
        userId: ctx.user.id,
        organizationId: ctx.effectiveOrganization.id,
      }}
      organization={ctx.effectiveOrganization.name}
      mode={mode}
      id={id}
    />
  );
}
function ScopedPage({
  scope,
  organization,
  mode,
  id,
}: {
  scope: ConfigurationScope;
  organization: string;
  mode: "market" | "mine";
  id?: string;
}) {
  const [filter, setFilter] = useState(""),
    [after, setAfter] = useState(""),
    [nonce, setNonce] = useState(0),
    [intent, setIntent] = useState<Intent | null>(null),
    [busy, setBusy] = useState(false),
    [failure, setFailure] = useState<unknown>(null),
    [message, setMessage] = useState("");
  const [templateAfter, setTemplateAfter] = useState(""),
    [recentAfter, setRecentAfter] = useState("");
  const [disable, setDisable] = useState<CatalogEntry | null>(null),
    [editor, setEditor] = useState<AgentTemplate | "new" | null>(null),
    [chosen, setChosen] = useState<AgentTemplate | null>(null),
    [history, setHistory] = useState("");
  const active = useRef(true),
    flight = useRef(false),
    abort = useRef<AbortController | null>(null);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
      abort.current?.abort();
    };
  }, []);
  const path = id
    ? agentId
    : `${mode}?pageSize=20${filter ? "&activation=" + filter : ""}${after ? "&cursor=" + after : ""}`;
  const list = useRead<CatalogEntry | z.infer<typeof catalogPageSchema>>(
    scope,
    path,
    id ? catalogEntrySchema : catalogPageSchema,
    nonce,
  );
  const data = list.data;
  const detail = id && data ? (data as CatalogEntry) : null;
  const entries =
    !id && data ? (data as z.infer<typeof catalogPageSchema>).items : [];
  const templates = useRead(
    scope,
    `${agentId}/templates?pageSize=100${templateAfter ? "&cursor=" + templateAfter : ""}`,
    templatesPageSchema,
    nonce,
    !!detail && detail.agent.activation !== "NOT_ENABLED",
  );
  const recent = useRead(
    scope,
    `${agentId}/recent-runs?pageSize=20${recentAfter ? "&cursor=" + recentAfter : ""}`,
    recentRunsSchema,
    nonce,
    !!detail?.canReadRuns,
  );
  const defaultRef = detail?.agent.defaultTemplate;
  const defaultTemplate = useRead(
    scope,
    defaultRef
      ? `${agentId}/templates/${defaultRef.templateId}/revisions/${defaultRef.revision}`
      : "",
    templateSchema,
    nonce,
    !!defaultRef,
  );
  const old = useRead(
    scope,
    chosen && history && configVersion.safeParse(history).success
      ? `${agentId}/templates/${chosen.templateId}/revisions/${history}`
      : "",
    templateSchema,
    nonce,
    !!chosen && !!history && configVersion.safeParse(history).success,
  );
  async function send(command: Intent) {
    if (flight.current || !active.current) return;
    flight.current = true;
    setBusy(true);
    setFailure(null);
    setIntent(command);
    const controller = new AbortController();
    abort.current = controller;
    const headers = new Headers({
      "Content-Type": "application/json",
      "Idempotency-Key": command.key,
    });
    if (command.absent) headers.set("If-None-Match", "*");
    else if (command.revision) headers.set("If-Match", `"${command.revision}"`);
    try {
      await configurationRequest(scope, command.path, configReceiptSchema, {
        method: command.method,
        headers,
        body: JSON.stringify(command.payload),
        signal: controller.signal,
      });
      if (!active.current) return;
      setIntent(null);
      setEditor(null);
      setDisable(null);
      setChosen(null);
      setHistory("");
      setMessage("操作已保存，正在读取当前状态。");
      setNonce((n) => n + 1);
    } catch (error) {
      if (!active.current) return;
      setFailure(error);
      setMessage("");
      if (
        !(error instanceof ConfigurationError) ||
        error.code !== "OUTCOME_UNKNOWN"
      )
        setIntent(null);
    } finally {
      flight.current = false;
      if (active.current) setBusy(false);
    }
  }
  function mutate(
    path: string,
    payload: unknown = {},
    revision?: string,
    absent = false,
    method: "POST" | "PUT" = "POST",
  ) {
    if (intent || busy) return;
    void send({
      path,
      payload,
      revision,
      absent,
      method,
      key: crypto.randomUUID(),
    });
  }
  const mutationDisabled = busy || !!intent;
  function card(entry: CatalogEntry) {
    return (
      <Card key={entry.agent.agentId} className={styles.agentCard}>
        <h2>{entry.name}</h2>
        <p>{entry.description}</p>
        <p className={styles.support}>标题优化 · 人工审核</p>
        <div className={styles.cardActions}>
          <span
            className={styles.status}
            data-active={entry.agent.activation === "ENABLED"}
          >
            {labels[entry.agent.activation]}
          </span>
          {entry.canConfigure && entry.agent.activation !== "ENABLED" ? (
            <Button
              disabled={mutationDisabled}
              onClick={() =>
                mutate(
                  `${entry.agent.agentId}/enable`,
                  {},
                  entry.agent.revision || undefined,
                  entry.agent.activation === "NOT_ENABLED",
                )
              }
            >
              启用到当前企业
            </Button>
          ) : (
            <Link href={`/workbench/agents/mine/${entry.agent.agentId}`}>
              查看配置 →
            </Link>
          )}
        </div>
        {mode === "mine" && (
          <div className={styles.cardActions}>
            <Link href={`/workbench/agents/mine/${entry.agent.agentId}`}>
              能力与模板
            </Link>
            {entry.canUse ? (
              <Link href="/workbench/supply/acquisition">进入使用 →</Link>
            ) : (
              <span className="text-muted-foreground">当前不能执行</span>
            )}
          </div>
        )}
        {!entry.canUse &&
          entry.capabilities
            .filter(
              (cap) =>
                cap.support === "REQUIRED" && cap.readiness !== "AVAILABLE",
            )
            .map((cap) => (
              <p key={cap.id} className="text-muted-foreground">
                <strong>{capabilityNames[cap.id]}</strong>：
                <span>{cap.reason}</span>
              </p>
            ))}
      </Card>
    );
  }
  return (
    <ConsolePage
      title={
        id ? "智能体配置" : mode === "market" ? "智能体市场" : "我的智能体"
      }
      description={
        <>
          当前企业 · {organization}。
          {mode === "mine"
            ? "管理企业智能体，查看配置状态和本人最近任务。"
            : "选择适合业务场景的智能体，显式启用到当前企业。"}
        </>
      }
      breadcrumbs={[
        { label: "智能市场" },
        {
          label: id
            ? "我的智能体"
            : mode === "market"
              ? "智能体市场"
              : "我的智能体",
          href: id ? "/workbench/agents/mine" : undefined,
        },
        ...(id ? [{ label: "智能体配置" }] : []),
      ]}
      actions={
        <Link
          href={
            mode === "market"
              ? "/workbench/agents/mine"
              : "/workbench/agents/market"
          }
        >
          {mode === "market" ? "我的智能体 →" : "前往智能体市场 →"}
        </Link>
      }
    >
      {failure != null && (
        <div role="alert" className={styles.notice}>
          {configurationError(failure)}
          {intent && (
            <Button disabled={busy} onClick={() => void send(intent)}>
              重试同一次操作
            </Button>
          )}
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => {
              setNonce((n) => n + 1);
              if (!intent) setFailure(null);
            }}
          >
            刷新当前状态
          </Button>
        </div>
      )}
      {message && <p role="status">{message}</p>}
      {!data && !list.error ? (
        <ConsoleState kind="loading" title="正在读取企业智能体" />
      ) : list.error ? (
        <ConsoleState kind="error" title="智能体配置不可用">
          <p>{configurationError(list.error)}</p>
          <Button onClick={() => setNonce((n) => n + 1)}>重新读取</Button>
        </ConsoleState>
      ) : id && detail ? (
        <>
          <Card className={styles.detailHead}>
            <div>
              <h2>{detail.name}</h2>
              <p>{detail.description}</p>
              <p>
                {labels[detail.agent.activation]} · {detail.definitionVersion}
              </p>
            </div>
            <div className={styles.actions}>
              {detail.canUse && (
                <Link href="/workbench/supply/acquisition">
                  选择商品并进入使用 →
                </Link>
              )}
              {detail.canConfigure &&
                (detail.agent.activation === "ENABLED" ? (
                  <Button
                    variant="outline"
                    disabled={mutationDisabled}
                    onClick={() => setDisable(detail)}
                  >
                    停用智能体
                  </Button>
                ) : (
                  <Button
                    disabled={mutationDisabled}
                    onClick={() =>
                      mutate(
                        `${agentId}/enable`,
                        {},
                        detail.agent.revision || undefined,
                        detail.agent.activation === "NOT_ENABLED",
                      )
                    }
                  >
                    启用到当前企业
                  </Button>
                ))}
            </div>
          </Card>
          <div className={styles.detailGrid}>
            <Card className={styles.panel}>
              <h2>当前能力</h2>
              <p>配置状态不代表执行许可，执行时重新确认当前权限与预算。</p>
              {detail.capabilities.map((cap) => (
                <section key={cap.id} className={styles.capability}>
                  <div>
                    <h3>
                      {capabilityNames[cap.id]} · {labels[cap.support]}
                    </h3>
                    <p>{cap.reason}</p>
                  </div>
                  <span>
                    {
                      labels[
                        cap.support === "NOT_SUPPORTED"
                          ? cap.support
                          : cap.readiness
                      ]
                    }
                  </span>
                </section>
              ))}
            </Card>
            <Card className={styles.panel}>
              <div className={styles.cardActions}>
                <h2>默认模板</h2>
                {detail.canConfigure && defaultRef && (
                  <Button
                    variant="outline"
                    disabled={mutationDisabled}
                    onClick={() =>
                      mutate(
                        `${agentId}/default-template`,
                        { templateId: null, revision: null },
                        detail.agent.revision,
                        false,
                        "PUT",
                      )
                    }
                  >
                    清除默认
                  </Button>
                )}
              </div>
              {!defaultRef ? (
                <p>暂无默认模板；执行时仍可显式选择平台和是否引用知识。</p>
              ) : defaultTemplate.error ? (
                <p role="alert">
                  默认模板当前不可用。执行前须明确选择其他模板或不使用模板。
                </p>
              ) : defaultTemplate.data ? (
                <>
                  <h3>
                    {defaultTemplate.data.name} · v
                    {defaultTemplate.data.version}
                  </h3>
                  <p>素材平台：{defaultTemplate.data.targetPlatform}</p>
                  <p>
                    {defaultTemplate.data.knowledgeAvailability ===
                    "UNAVAILABLE"
                      ? "默认知识当前不可用，执行前须重新选择或明确不使用知识。"
                      : defaultTemplate.data.defaultKnowledgeBaseId
                        ? "保存了默认企业知识；执行时仍需明确确认。"
                        : "默认不引用企业知识。"}
                  </p>
                  <p>模板更新不会自动移动此精确版本。</p>
                </>
              ) : (
                <p role="status">正在读取默认版本…</p>
              )}
            </Card>
          </div>
          <Card className={styles.panel}>
            <div className={styles.cardActions}>
              <h2>版本化模板</h2>
              {detail.canConfigure && (
                <Button
                  disabled={
                    mutationDisabled ||
                    detail.agent.activation === "NOT_ENABLED"
                  }
                  onClick={() => setEditor("new")}
                >
                  新建模板
                </Button>
              )}
            </div>
            {templates.error ? (
              <p role="alert">{configurationError(templates.error)}</p>
            ) : templates.data?.items.length ? (
              <div className={styles.templateList}>
                {templates.data.items.map((t) => (
                  <section key={t.templateId} className={styles.templateRow}>
                    <div>
                      <h3>
                        {t.name} · v{t.version}
                      </h3>
                      <p>
                        {t.targetPlatform} ·{" "}
                        {t.lifecycle === "ACTIVE" ? "可选用" : "已归档"}
                      </p>
                      <p>
                        {t.knowledgeAvailability === "UNAVAILABLE"
                          ? "默认知识不可用"
                          : t.defaultKnowledgeBaseId
                            ? "含默认知识选择"
                            : "不引用企业知识"}
                      </p>
                    </div>
                    <div className={styles.actions}>
                      <Button
                        variant="outline"
                        onClick={() => {
                          setChosen(t);
                          setHistory(t.version);
                        }}
                      >
                        查看版本
                      </Button>
                      {detail.canConfigure && t.lifecycle === "ACTIVE" && (
                        <>
                          <Button
                            variant="outline"
                            disabled={mutationDisabled}
                            onClick={() => setEditor(t)}
                          >
                            编辑
                          </Button>
                          <Button
                            disabled={mutationDisabled}
                            onClick={() =>
                              mutate(
                                `${agentId}/default-template`,
                                {
                                  templateId: t.templateId,
                                  revision: t.version,
                                },
                                detail.agent.revision,
                                false,
                                "PUT",
                              )
                            }
                          >
                            设为默认 v{t.version}
                          </Button>
                          <Button
                            variant="outline"
                            disabled={mutationDisabled}
                            onClick={() =>
                              mutate(
                                `${agentId}/templates/${t.templateId}/archive`,
                                {},
                                t.revision,
                              )
                            }
                          >
                            归档
                          </Button>
                        </>
                      )}
                    </div>
                  </section>
                ))}
              </div>
            ) : (
              <p>
                {detail.agent.activation === "NOT_ENABLED"
                  ? "请先启用到当前企业。"
                  : "当前企业尚无模板。"}
              </p>
            )}
            {(templateAfter || templates.data?.nextCursor) && (
              <nav aria-label="模板分页" className={styles.actions}>
                {templateAfter && (
                  <Button
                    variant="outline"
                    onClick={() => setTemplateAfter("")}
                  >
                    回到模板首页
                  </Button>
                )}
                {templates.data?.nextCursor && (
                  <Button
                    variant="outline"
                    onClick={() => setTemplateAfter(templates.data!.nextCursor)}
                  >
                    下一页模板
                  </Button>
                )}
              </nav>
            )}
            {chosen && (
              <section className={styles.version}>
                <label>
                  查看不可变版本
                  <Input
                    value={history}
                    onChange={(e) => setHistory(e.target.value)}
                    inputMode="numeric"
                  />
                </label>
                {old.error ? (
                  <p role="alert">该版本在当前企业不可用。</p>
                ) : old.data ? (
                  <>
                    <h3>
                      {old.data.name} · v{old.data.version}
                    </h3>
                    <p>
                      素材平台 {old.data.targetPlatform} ·{" "}
                      {old.data.knowledgeAvailability === "UNAVAILABLE"
                        ? "知识不可用"
                        : old.data.defaultKnowledgeBaseId
                          ? "默认企业知识"
                          : "不使用知识"}
                    </p>
                    {detail.canConfigure && old.data.lifecycle === "ACTIVE" && (
                      <Button
                        disabled={mutationDisabled}
                        onClick={() =>
                          mutate(
                            `${agentId}/default-template`,
                            {
                              templateId: old.data!.templateId,
                              revision: old.data!.version,
                            },
                            detail.agent.revision,
                            false,
                            "PUT",
                          )
                        }
                      >
                        设为默认 v{old.data.version}
                      </Button>
                    )}
                  </>
                ) : null}
                <Button
                  variant="outline"
                  onClick={() => {
                    setChosen(null);
                    setHistory("");
                  }}
                >
                  关闭版本
                </Button>
              </section>
            )}
          </Card>
          <Card className={styles.panel}>
            <h2>本人最近发起的任务</h2>
            <p>
              仅展示当前企业中你有商品访问权限的任务；生成建议仍需人工审核。
            </p>
            {!detail.canReadRuns ? (
              <p>当前身份无权读取商品智能体任务。</p>
            ) : recent.error ? (
              <p>{configurationError(recent.error)}</p>
            ) : recent.data?.items.length ? (
              <ul className={styles.recent}>
                {recent.data.items.map((run) => (
                  <li key={run.runId}>
                    <span>
                      {run.productKey} · {labels[run.phase]}
                    </span>
                    <Link
                      href={`/workbench/supply/acquisition/operation/${run.operationId}?agent_key=${run.requestKey}&agent_actor=${encodeURIComponent(scope.userId)}&agent_org=${encodeURIComponent(scope.organizationId)}&agent_platform=${run.targetPlatform}`}
                    >
                      查看任务 →
                    </Link>
                  </li>
                ))}
              </ul>
            ) : recent.data ? (
              <p>暂无本人已领取执行的任务。</p>
            ) : (
              <p role="status">正在读取最近任务…</p>
            )}
            {(recentAfter || recent.data?.nextCursor) && (
              <nav aria-label="最近任务分页" className={styles.actions}>
                {recentAfter && (
                  <Button variant="outline" onClick={() => setRecentAfter("")}>
                    最近任务首页
                  </Button>
                )}
                {recent.data?.nextCursor && (
                  <Button
                    variant="outline"
                    onClick={() => setRecentAfter(recent.data!.nextCursor)}
                  >
                    更早的任务
                  </Button>
                )}
              </nav>
            )}
          </Card>
        </>
      ) : (
        <>
          {mode === "mine" && (
            <div className={styles.filters}>
              {[
                ["", "全部"],
                ["ENABLED", "已启用"],
                ["DISABLED", "已停用"],
              ].map(([value, label]) => (
                <Button
                  key={value}
                  variant="outline"
                  aria-pressed={filter === value}
                  onClick={() => {
                    setFilter(value);
                    setAfter("");
                  }}
                >
                  {label}
                </Button>
              ))}
            </div>
          )}
          <div
            className={mode === "market" ? styles.marketGrid : styles.mineGrid}
          >
            <div
              className={mode === "market" ? styles.cards : styles.mineCards}
            >
              {entries.length ? (
                entries.map(card)
              ) : (
                <ConsoleState kind="empty" title="当前企业尚无符合条件的智能体">
                  <Link href="/workbench/agents/market">前往市场启用</Link>
                </ConsoleState>
              )}
            </div>
            {mode === "mine" && (
              <Card className={styles.panel}>
                <h2>使用流程</h2>
                <ol className={styles.steps}>
                  <li>选择企业已启用的智能体</li>
                  <li>从 1688 采集已保存商品中选择商品</li>
                  <li>确认精确模板版本、平台和企业知识选择</li>
                  <li>查看生成建议，经过 Human Review 后显式应用</li>
                </ol>
                <p>启用智能体不会创建执行任务，也不会自动使用知识。</p>
              </Card>
            )}
          </div>
          {!id && data && "nextCursor" in data && data.nextCursor && (
            <Button variant="outline" onClick={() => setAfter(data.nextCursor)}>
              下一页
            </Button>
          )}
        </>
      )}
      {disable && (
        <ConfirmDisable
          busy={busy}
          onCancel={() => setDisable(null)}
          onConfirm={() =>
            mutate(`${agentId}/disable`, {}, disable.agent.revision)
          }
        />
      )}
      {editor && (
        <TemplateEditor
          scope={scope}
          template={editor === "new" ? undefined : editor}
          busy={mutationDisabled}
          onCancel={() => setEditor(null)}
          onSave={(payload) =>
            mutate(
              editor === "new"
                ? `${agentId}/templates`
                : `${agentId}/templates/${editor.templateId}`,
              payload,
              editor === "new" ? undefined : editor.revision,
              false,
              editor === "new" ? "POST" : "PUT",
            )
          }
        />
      )}
    </ConsolePage>
  );
}
function ConfirmDisable({
  busy,
  onConfirm,
  onCancel,
}: {
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const e = dialog.current;
    e?.showModal();
    return () => e?.close();
  }, []);
  return (
    <dialog
      ref={dialog}
      className={styles.dialog}
      aria-labelledby="disable-agent-title"
      onCancel={(e) => {
        e.preventDefault();
        onCancel();
      }}
    >
      <h2 id="disable-agent-title">停用当前企业智能体</h2>
      <p>
        停用后阻止新启动和继续执行。已经领取的有限执行可以完成；原结果、人工审核及应用不会因此删除。
      </p>
      <div className={styles.actions}>
        <Button variant="outline" disabled={busy} onClick={onCancel}>
          取消
        </Button>
        <Button disabled={busy} onClick={onConfirm}>
          确认停用
        </Button>
      </div>
    </dialog>
  );
}
function TemplateEditor({
  scope,
  template,
  busy,
  onSave,
  onCancel,
}: {
  scope: ConfigurationScope;
  template?: AgentTemplate;
  busy: boolean;
  onSave: (v: unknown) => void;
  onCancel: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [name, setName] = useState(template?.name ?? ""),
    [platform, setPlatform] = useState(template?.targetPlatform ?? "shein"),
    [base, setBase] = useState(
      template?.knowledgeAvailability === "UNAVAILABLE"
        ? "unresolved"
        : (template?.defaultKnowledgeBaseId ?? ""),
    ),
    [bases, setBases] = useState<z.infer<typeof basesSchema>["items"]>([]),
    [knowledgeError, setKnowledgeError] = useState(""),
    [basePage, setBasePage] = useState(1),
    [baseTotal, setBaseTotal] = useState(0);
  useEffect(() => {
    const e = dialog.current;
    e?.showModal();
    const controller = new AbortController();
    knowledgeRequest(
      scope,
      `knowledge-bases?pageSize=100&page=${basePage}`,
      basesSchema,
      { signal: controller.signal },
    )
      .then((result) => {
        if (!controller.signal.aborted) {
          setBases(result.items.filter((b) => b.state === "ACTIVE"));
          setBaseTotal(result.pagination.total);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted)
          setKnowledgeError("企业知识当前不可用；仍可明确保存为不引用知识。");
      });
    return () => {
      e?.close();
      controller.abort();
    };
  }, [scope, basePage]);
  return (
    <dialog
      ref={dialog}
      className={styles.dialog}
      aria-labelledby="template-editor-title"
      onCancel={(e) => {
        e.preventDefault();
        onCancel();
      }}
    >
      <h2 id="template-editor-title">
        {template ? "保存模板新版本" : "保存标题模板"}
      </h2>
      <p>模板保存默认配置，执行前仍需确认当前企业权限、预算和知识版本。</p>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (base !== "unresolved")
            onSave({
              name,
              targetPlatform: platform,
              defaultKnowledgeBaseId: base || null,
            });
        }}
      >
        <label>
          模板名称
          <Input
            required
            maxLength={120}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="例如：品牌标题优化"
          />
        </label>
        <label>
          素材平台
          <select
            value={platform}
            onChange={(e) => setPlatform(e.target.value as typeof platform)}
          >
            <option value="shein">SHEIN</option>
            <option value="temu">TEMU</option>
            <option value="amazon">Amazon</option>
          </select>
        </label>
        <label>
          默认企业知识
          <select value={base} onChange={(e) => setBase(e.target.value)}>
            {base === "unresolved" && (
              <option value="unresolved">
                原默认知识不可用，请明确重新选择
              </option>
            )}
            <option value="">不引用企业知识</option>
            {template?.defaultKnowledgeBaseId &&
              !bases.some((b) => b.id === template.defaultKnowledgeBaseId) && (
                <option value={template.defaultKnowledgeBaseId}>
                  原默认知识选择（保存时重新确认）
                </option>
              )}
            {bases.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </label>
        {baseTotal > 100 && (
          <nav aria-label="模板知识库分页" className={styles.actions}>
            <Button
              type="button"
              variant="outline"
              disabled={basePage === 1}
              onClick={() => {
                setBase("unresolved");
                setBases([]);
                setBasePage((n) => n - 1);
              }}
            >
              上一页知识库
            </Button>
            <span>第 {basePage} 页</span>
            <Button
              type="button"
              variant="outline"
              disabled={basePage * 100 >= baseTotal}
              onClick={() => {
                setBase("unresolved");
                setBases([]);
                setBasePage((n) => n + 1);
              }}
            >
              下一页知识库
            </Button>
          </nav>
        )}
        {knowledgeError && <p role="status">{knowledgeError}</p>}
        <p className={styles.notice}>
          保存新版本不会移动企业默认版本；“设为默认”需单独确认。
        </p>
        <div className={styles.actions}>
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={onCancel}
          >
            取消
          </Button>
          <Button
            type="submit"
            disabled={busy || base === "unresolved" || !name.trim()}
          >
            保存模板
          </Button>
        </div>
      </form>
    </dialog>
  );
}
