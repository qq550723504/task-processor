"use client";

import { useMemo, useState } from "react";
import {
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  ChevronRight,
  Clock3,
  KeyRound,
  LoaderCircle,
  RefreshCw,
  ShieldAlert,
  Store,
} from "lucide-react";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import {
  useClearSheinCookie,
  useClearSheinLastFailure,
  useLoginSheinAccount,
  useSheinLastFailure,
  useSheinLoginAccounts,
  useSubmitSheinVerifyCode,
} from "@/lib/query/use-shein-login";
import { cn } from "@/lib/utils/cn";
import type {
  SheinLoginAccountStatus,
  SheinLoginFailureDetail,
  SheinLoginFailureSummary,
} from "@/lib/types/shein-login";

type AccountFilter = "all" | "attention" | "verify" | "failed" | "healthy";

function formatDateTime(value?: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function formatTTL(ttl?: number) {
  const value = Number(ttl ?? 0);
  if (value <= 0) {
    return "未持久化";
  }
  if (value < 60) {
    return `${value} 秒`;
  }
  if (value < 3600) {
    return `${Math.floor(value / 60)} 分`;
  }
  if (value < 86400) {
    return `${(value / 3600).toFixed(value < 36000 ? 1 : 0)} 小时`;
  }
  return `${(value / 86400).toFixed(value < 864000 ? 1 : 0)} 天`;
}

function metricCards(items: SheinLoginAccountStatus[]) {
  return [
    {
      label: "店铺数",
      value: items.length,
      note: "当前可管理账户",
      tone: "text-zinc-950",
    },
    {
      label: "Cookie 可用",
      value: items.filter((item) => item.has_cookie || (item.cookie_ttl ?? 0) > 0).length,
      note: "已有可用登录态",
      tone: "text-emerald-700",
    },
    {
      label: "等待验证码",
      value: items.filter((item) => item.waiting_for_verify_code).length,
      note: "需要人工继续处理",
      tone: "text-amber-700",
    },
    {
      label: "最近失败",
      value: items.filter((item) => item.last_failure).length,
      note: "存在失败记录",
      tone: "text-rose-700",
    },
  ];
}

function accountFilterItems(items: SheinLoginAccountStatus[]) {
  return [
    {
      key: "all" as const,
      label: "全部",
      count: items.length,
    },
    {
      key: "attention" as const,
      label: "待处理",
      count: items.filter(
        (item) =>
          item.waiting_for_verify_code ||
          Boolean(item.last_failure) ||
          (!item.has_cookie && (item.cookie_ttl ?? 0) <= 0),
      ).length,
    },
    {
      key: "verify" as const,
      label: "验证码",
      count: items.filter((item) => item.waiting_for_verify_code).length,
    },
    {
      key: "failed" as const,
      label: "失败",
      count: items.filter((item) => item.last_failure).length,
    },
    {
      key: "healthy" as const,
      label: "可用",
      count: items.filter((item) => item.has_cookie || (item.cookie_ttl ?? 0) > 0).length,
    },
  ];
}

function summaryTone(item: SheinLoginAccountStatus) {
  if (item.waiting_for_verify_code) {
    return "border-amber-200 bg-amber-50 text-amber-800";
  }
  if (item.last_failure) {
    return "border-rose-200 bg-rose-50 text-rose-800";
  }
  if (item.has_cookie || (item.cookie_ttl ?? 0) > 0) {
    return "border-emerald-200 bg-emerald-50 text-emerald-800";
  }
  return "border-zinc-200 bg-zinc-50 text-zinc-700";
}

function actionTone(item: SheinLoginAccountStatus, kind: "login" | "code" | "failure" | "cookie") {
  const actionKey = item.recommended_action?.key ?? "";
  if (
    (kind === "code" && (item.waiting_for_verify_code || actionKey === "submit_verify_code")) ||
    (kind === "login" && (actionKey === "retry_login" || actionKey === "check_cookie_persistence")) ||
    (kind === "failure" &&
      ["inspect_artifact", "use_master_account", "fix_account_permission"].includes(actionKey))
  ) {
    return "primary";
  }
  if (kind === "cookie" && actionKey === "check_credentials") {
    return "danger";
  }
  return "secondary";
}

function storeDisplay(item: SheinLoginAccountStatus) {
  const account = item.account;
  return account.store_name || account.shop_name || `店铺 ${account.store_id}`;
}

function filterAccounts(items: SheinLoginAccountStatus[], filter: AccountFilter) {
  switch (filter) {
    case "attention":
      return items.filter(
        (item) =>
          item.waiting_for_verify_code ||
          Boolean(item.last_failure) ||
          (!item.has_cookie && (item.cookie_ttl ?? 0) <= 0),
      );
    case "verify":
      return items.filter((item) => item.waiting_for_verify_code);
    case "failed":
      return items.filter((item) => item.last_failure);
    case "healthy":
      return items.filter((item) => item.has_cookie || (item.cookie_ttl ?? 0) > 0);
    default:
      return items;
  }
}

function storeStatusSummary(item: SheinLoginAccountStatus) {
  if (item.waiting_for_verify_code) {
    return "等待验证码";
  }
  if (item.last_failure) {
    return item.last_failure.error_code || item.last_failure.page_state || "最近失败";
  }
  if (item.login_in_progress) {
    return "登录流程执行中";
  }
  if (item.has_cookie || (item.cookie_ttl ?? 0) > 0) {
    return `Cookie 可用 · ${formatTTL(item.cookie_ttl)}`;
  }
  return "尚未持有可用 Cookie";
}

function storeStatusNote(item: SheinLoginAccountStatus) {
  if (item.waiting_for_verify_code) {
    return "登录流程停在验证码阶段，继续提交验证码即可。";
  }
  if (item.last_failure) {
    return (
      item.last_failure.action_message ||
      item.last_failure.error_message ||
      "查看失败详情，确认页面状态和建议动作。"
    );
  }
  if (item.login_in_progress) {
    return "后台正在尝试刷新登录态，可以稍后刷新状态。";
  }
  if (item.has_cookie || (item.cookie_ttl ?? 0) > 0) {
    return item.last_login_time
      ? `最近登录 ${formatDateTime(item.last_login_time)}`
      : "当前可以直接复用现有登录态。";
  }
  return "建议先发起登录，建立新的店铺 Cookie。";
}

function runbookItems(items: SheinLoginAccountStatus[]) {
  const verifyCount = items.filter((item) => item.waiting_for_verify_code).length;
  const failureCount = items.filter((item) => item.last_failure).length;
  const emptyCookieCount = items.filter(
    (item) => !item.has_cookie && (item.cookie_ttl ?? 0) <= 0 && !item.waiting_for_verify_code,
  ).length;

  return [
    {
      title: "先处理验证码",
      description:
        verifyCount > 0
          ? `${verifyCount} 个店铺停在验证码阶段，优先提交验证码。`
          : "当前没有店铺停在验证码阶段。",
      tone:
        verifyCount > 0
          ? "border-amber-200 bg-amber-50 text-amber-900"
          : "border-zinc-200 bg-zinc-50 text-zinc-700",
    },
    {
      title: "再检查失败记录",
      description:
        failureCount > 0
          ? `${failureCount} 个店铺存在最近失败，优先查看详情确认阻断点。`
          : "当前没有需要排查的失败记录。",
      tone:
        failureCount > 0
          ? "border-rose-200 bg-rose-50 text-rose-900"
          : "border-zinc-200 bg-zinc-50 text-zinc-700",
    },
    {
      title: "最后补新 Cookie",
      description:
        emptyCookieCount > 0
          ? `${emptyCookieCount} 个店铺还没有可用 Cookie，可以按需重新登录。`
          : "当前没有额外需要补 Cookie 的店铺。",
      tone:
        emptyCookieCount > 0
          ? "border-sky-200 bg-sky-50 text-sky-900"
          : "border-zinc-200 bg-zinc-50 text-zinc-700",
    },
  ];
}

function mutationStatusNote({
  clearCookie,
  clearFailure,
  login,
  verifyCode,
}: {
  clearCookie: ReturnType<typeof useClearSheinCookie>;
  clearFailure: ReturnType<typeof useClearSheinLastFailure>;
  login: ReturnType<typeof useLoginSheinAccount>;
  verifyCode: ReturnType<typeof useSubmitSheinVerifyCode>;
}) {
  if (verifyCode.isPending) {
    return "正在提交验证码并刷新店铺状态。";
  }
  if (login.isPending) {
    return "正在发起店铺登录。";
  }
  if (clearCookie.isPending) {
    return "正在清理 Cookie。";
  }
  if (clearFailure.isPending) {
    return "正在清理失败记录。";
  }

  const mutationError =
    verifyCode.error || login.error || clearCookie.error || clearFailure.error;
  if (mutationError) {
    return (mutationError as Error).message || "最近操作失败。";
  }

  return "状态每 15 秒自动刷新，也可以手动刷新。";
}

function VerifyCodeDialog({
  open,
  storeID,
  pending,
  onClose,
  onSubmit,
}: {
  open: boolean;
  storeID?: number | null;
  pending: boolean;
  onClose: () => void;
  onSubmit: (code: string) => void;
}) {
  const [code, setCode] = useState("");

  if (!open || !storeID) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4">
      <div className="w-full max-w-md rounded-3xl border border-white/70 bg-white p-6 shadow-[0_28px_80px_rgba(24,24,27,0.22)]">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-teal-700">
              Verify Code
            </p>
            <h2 className="mt-2 text-xl font-semibold text-zinc-950">提交验证码</h2>
            <p className="mt-2 text-sm leading-6 text-zinc-600">店铺 {storeID} 当前停在验证码阶段，提交后会刷新登录状态。</p>
          </div>
          <Button tone="ghost" className="h-9 px-3" onClick={onClose} disabled={pending}>
            关闭
          </Button>
        </div>

        <label className="mt-5 grid gap-2">
          <span className="text-xs font-semibold uppercase tracking-[0.14em] text-zinc-500">
            验证码
          </span>
          <input
            value={code}
            onChange={(event) => setCode(event.target.value)}
            className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm outline-none focus:border-zinc-400"
            placeholder="输入短信或邮箱验证码"
          />
        </label>

        <div className="mt-5 flex justify-end gap-3">
          <Button tone="secondary" onClick={onClose} disabled={pending}>
            取消
          </Button>
          <Button
            disabled={pending || !code.trim()}
            onClick={() => {
              onSubmit(code.trim());
              setCode("");
            }}
          >
            {pending ? "提交中..." : "提交验证码"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function FailureDialog({
  open,
  storeID,
  detail,
  isLoading,
  onClose,
}: {
  open: boolean;
  storeID?: number | null;
  detail?: SheinLoginFailureDetail;
  isLoading: boolean;
  onClose: () => void;
}) {
  if (!open || !storeID) {
    return null;
  }

  const boolRows = [
    ["登录页可见", detail?.on_login_page],
    ["请求失败弹层", detail?.request_failure_modal],
    ["登录表单可见", detail?.login_form_visible],
    ["卖家中心可见", detail?.seller_hub_visible],
    ["验证码区域可见", detail?.verification_visible],
    ["权限提示可见", detail?.permission_visible],
    ["协议提示可见", detail?.agreement_visible],
    ["账号密码错误可见", detail?.credential_error_visible],
  ] as const;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6">
      <div className="flex max-h-full w-full max-w-5xl flex-col overflow-hidden rounded-3xl border border-white/70 bg-white shadow-[0_28px_80px_rgba(24,24,27,0.22)]">
        <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-6 py-5">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-teal-700">
              Failure Detail
            </p>
            <h2 className="mt-2 text-xl font-semibold text-zinc-950">店铺 {storeID} 失败详情</h2>
            <p className="mt-1 text-sm text-zinc-500">
              {detail?.error_code || "最近失败记录"} {detail?.captured_at ? `· ${formatDateTime(detail.captured_at)}` : ""}
            </p>
          </div>
          <Button tone="ghost" className="h-9 px-3" onClick={onClose}>
            关闭
          </Button>
        </div>

        <div className="grid gap-4 overflow-auto px-6 py-5">
          {isLoading ? (
            <div className="flex min-h-40 items-center justify-center text-zinc-500">
              <LoaderCircle className="h-5 w-5 animate-spin" />
            </div>
          ) : !detail ? (
            <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-5 text-sm text-zinc-600">
              没有最近失败记录。
            </div>
          ) : (
            <>
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                {[
                  ["错误码", detail.error_code],
                  ["页面状态", detail.page_state],
                  ["建议动作", detail.action_key],
                  ["阶段", detail.stage],
                  ["URL", detail.url],
                  ["页面标题", detail.title],
                  ["Artifact", detail.artifact_path],
                  ["页面错误", detail.login_error],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
                      {label}
                    </div>
                    <div className="mt-2 break-all text-sm text-zinc-900">{value || "-"}</div>
                  </div>
                ))}
              </div>

              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                {boolRows.map(([label, value]) => (
                  <div key={label} className="rounded-2xl border border-zinc-200 bg-white p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
                      {label}
                    </div>
                    <div className="mt-2 text-sm font-medium text-zinc-900">{value ? "是" : "否"}</div>
                  </div>
                ))}
              </div>

              <Card className="border-zinc-200 bg-zinc-50 p-4">
                <h3 className="text-sm font-semibold text-zinc-900">建议说明</h3>
                <pre className="mt-3 whitespace-pre-wrap break-words text-xs leading-6 text-zinc-700">
                  {detail.action_message || "-"}
                </pre>
              </Card>

              <Card className="border-zinc-200 bg-zinc-50 p-4">
                <h3 className="text-sm font-semibold text-zinc-900">选择器状态</h3>
                <pre className="mt-3 whitespace-pre-wrap break-words text-xs leading-6 text-zinc-700">
                  {JSON.stringify(detail.selector_states ?? {}, null, 2)}
                </pre>
              </Card>

              <Card className="border-zinc-200 bg-zinc-50 p-4">
                <h3 className="text-sm font-semibold text-zinc-900">页面文本</h3>
                <pre className="mt-3 whitespace-pre-wrap break-words text-xs leading-6 text-zinc-700">
                  {detail.body_text || "-"}
                </pre>
              </Card>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

export function SheinLoginPage() {
  const accounts = useSheinLoginAccounts();
  const login = useLoginSheinAccount();
  const verifyCode = useSubmitSheinVerifyCode();
  const clearCookie = useClearSheinCookie();
  const clearFailure = useClearSheinLastFailure();

  const [verifyStoreID, setVerifyStoreID] = useState<number | null>(null);
  const [failureStoreID, setFailureStoreID] = useState<number | null>(null);
  const [activeFilter, setActiveFilter] = useState<AccountFilter>("attention");
  const failureDetail = useSheinLastFailure(failureStoreID);

  const accountItems = accounts.data ?? [];
  const summary = useMemo(() => metricCards(accountItems), [accountItems]);
  const filters = useMemo(() => accountFilterItems(accountItems), [accountItems]);
  const filteredAccounts = useMemo(
    () => filterAccounts(accountItems, activeFilter),
    [accountItems, activeFilter],
  );
  const runbook = useMemo(() => runbookItems(accountItems), [accountItems]);
  const statusNote = mutationStatusNote({ clearCookie, clearFailure, login, verifyCode });

  return (
    <ListingKitPageShell backgroundClassName="isolate bg-[linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)]">
      <section className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px] xl:items-start">
        <div className="grid gap-4">
          <section className="rounded-[2rem] border border-white/70 bg-white/78 p-6 shadow-[0_24px_90px_rgba(39,39,42,0.10)]">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">
                  SHEIN Login
                </p>
                <h1 className="mt-3 text-3xl font-semibold tracking-[-0.04em] text-zinc-950">
                  店铺登录管理
                </h1>
                <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-600">
                  先处理验证码和失败，再决定是否重登或清理 Cookie。页面只保留登录管理需要的信息，不再把调试细节直接铺在首屏。
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <span className="inline-flex h-10 items-center rounded-full border border-zinc-200 bg-white px-4 text-sm text-zinc-600">
                  <Clock3 className="mr-2 h-4 w-4 text-zinc-400" />
                  自动刷新 15s
                </span>
                <Button tone="secondary" onClick={() => accounts.refetch()} disabled={accounts.isFetching}>
                  <RefreshCw className={cn("mr-2 h-4 w-4", accounts.isFetching && "animate-spin")} />
                  刷新状态
                </Button>
              </div>
            </div>
          </section>

          <Card className="border-white/70 bg-white/88 p-0 shadow-[0_14px_36px_rgba(39,39,42,0.06)]">
            <div className="flex flex-wrap items-start justify-between gap-4 border-b border-zinc-200 px-6 py-5">
              <div>
                <h2 className="text-lg font-semibold text-zinc-950">店铺列表</h2>
                <p className="mt-1 text-sm text-zinc-500">
                  按异常优先级筛选，再对单店铺执行登录、验证码或清理动作。
                </p>
              </div>
              <div className="text-sm text-zinc-500">
                {accounts.isFetching ? "正在刷新..." : `显示 ${filteredAccounts.length} / ${accountItems.length}`}
              </div>
            </div>

            <div className="flex flex-wrap gap-2 border-b border-zinc-200 px-6 py-4">
              {filters.map((filter) => {
                const active = filter.key === activeFilter;
                return (
                  <button
                    key={filter.key}
                    type="button"
                    onClick={() => setActiveFilter(filter.key)}
                    className={cn(
                      "rounded-full border px-3 py-1.5 text-xs font-semibold transition",
                      active
                        ? "border-zinc-950 bg-zinc-950 text-white"
                        : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-300 hover:text-zinc-950",
                    )}
                  >
                    {filter.label} · {filter.count}
                  </button>
                );
              })}
            </div>

            {accounts.isLoading ? (
              <div className="flex min-h-60 items-center justify-center text-zinc-500">
                <LoaderCircle className="h-6 w-6 animate-spin" />
              </div>
            ) : accounts.isError ? (
              <div className="px-6 py-8">
                <div className="rounded-2xl border border-rose-200 bg-rose-50 p-5 text-sm text-rose-700">
                  {(accounts.error as Error)?.message || "SHEIN 登录状态加载失败。"}
                </div>
              </div>
            ) : !accountItems.length ? (
              <div className="px-6 py-8">
                <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-5 text-sm text-zinc-600">
                  当前没有可管理的 SHEIN 店铺。
                </div>
              </div>
            ) : !filteredAccounts.length ? (
              <div className="px-6 py-8">
                <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-5 text-sm text-zinc-600">
                  当前筛选下没有匹配店铺。
                </div>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-[980px] w-full">
              <thead>
                <tr className="border-b border-zinc-200 bg-zinc-50/90 text-left text-[11px] font-semibold uppercase tracking-[0.16em] text-zinc-500">
                  <th className="px-6 py-4">店铺</th>
                  <th className="px-6 py-4">当前状态</th>
                  <th className="px-6 py-4">建议动作</th>
                  <th className="px-6 py-4">操作</th>
                </tr>
              </thead>
              <tbody>
                {filteredAccounts.map((item) => (
                  <tr key={item.account.store_id} className="border-b border-zinc-200 align-top last:border-b-0">
                    <td className="px-6 py-5">
                      <div className="flex items-start gap-3">
                        <div className="mt-0.5 rounded-xl bg-zinc-100 p-2 text-zinc-700">
                          <Store className="h-4 w-4" />
                        </div>
                        <div>
                          <div className="font-semibold text-zinc-950">{storeDisplay(item)}</div>
                          <div className="mt-1 text-sm text-zinc-500">
                            账号 {item.account.username || "-"} · ID {item.account.store_id}
                            {item.account.proxy ? " · 已配置代理" : ""}
                          </div>
                          {item.account.login_url ? (
                            <a
                              href={item.account.login_url}
                              target="_blank"
                              rel="noreferrer"
                              className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-zinc-500 transition hover:text-zinc-900"
                            >
                              登录页
                              <ArrowUpRight className="h-3.5 w-3.5" />
                            </a>
                          ) : null}
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-5">
                      <div className="grid gap-2">
                        <div className="flex flex-wrap gap-2">
                          <span className={cn("rounded-full border px-3 py-1 text-xs font-semibold", summaryTone(item))}>
                            {storeStatusSummary(item)}
                          </span>
                          {item.login_in_progress ? (
                            <span className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-800">
                              登录中
                            </span>
                          ) : null}
                          {item.last_login_time && !item.waiting_for_verify_code ? (
                            <span className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs font-semibold text-zinc-700">
                              最近登录 {formatDateTime(item.last_login_time)}
                            </span>
                          ) : null}
                        </div>
                        <p className="text-sm text-zinc-500">
                          {storeStatusNote(item)}
                        </p>
                        {item.last_failure ? (
                          <p className="text-xs text-zinc-500">
                            {[formatDateTime(item.last_failure.captured_at), item.last_failure.stage]
                              .filter(Boolean)
                              .join(" · ")}
                          </p>
                        ) : null}
                      </div>
                    </td>
                    <td className="px-6 py-5">
                      {item.recommended_action?.key || item.recommended_action?.message ? (
                        <div className="grid gap-2">
                          <div className="flex items-start gap-2">
                            <ChevronRight className="mt-0.5 h-4 w-4 text-zinc-400" />
                            <div className="grid gap-1">
                              {item.recommended_action?.key ? (
                                <div className="font-semibold text-zinc-900">{item.recommended_action.key}</div>
                              ) : null}
                              {item.recommended_action?.message ? (
                                <p className="text-sm text-zinc-500">{item.recommended_action.message}</p>
                              ) : null}
                            </div>
                          </div>
                        </div>
                      ) : (
                        <span className="text-sm text-zinc-400">无建议动作</span>
                      )}
                    </td>
                    <td className="px-6 py-5">
                      <div className="flex flex-wrap gap-2">
                        <Button
                          tone={actionTone(item, "login")}
                          className="h-9 px-3"
                          onClick={() => login.mutate(item.account.store_id)}
                          disabled={login.isPending}
                        >
                          重新登录
                        </Button>
                        <Button
                          tone={actionTone(item, "code")}
                          className="h-9 px-3"
                          onClick={() => setVerifyStoreID(item.account.store_id)}
                        >
                          <KeyRound className="mr-2 h-4 w-4" />
                          验证码
                        </Button>
                        <Button
                          tone={actionTone(item, "failure")}
                          className="h-9 px-3"
                          onClick={() => setFailureStoreID(item.account.store_id)}
                        >
                          详情
                        </Button>
                        <Button
                          tone={actionTone(item, "cookie")}
                          className="h-9 px-3"
                          onClick={() => clearCookie.mutate(item.account.store_id)}
                          disabled={clearCookie.isPending}
                        >
                          清 Cookie
                        </Button>
                        <Button
                          tone="secondary"
                          className="h-9 px-3"
                          onClick={() => clearFailure.mutate(item.account.store_id)}
                          disabled={clearFailure.isPending}
                        >
                          清失败
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
            )}
          </Card>
        </div>

        <aside className="xl:sticky xl:top-4">
          <div className="grid gap-4">
            <Card className="border-white/70 bg-white/88 p-5 shadow-[0_14px_36px_rgba(39,39,42,0.06)]">
              <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
                <ShieldAlert className="h-4 w-4" />
                当前概况
              </div>
              <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                {summary.map((item) => (
                  <div key={item.label} className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-zinc-500">
                      {item.label}
                    </div>
                    <div className={cn("mt-2 text-2xl font-semibold leading-none", item.tone)}>{item.value}</div>
                    <div className="mt-2 text-sm text-zinc-500">{item.note}</div>
                  </div>
                ))}
              </div>
            </Card>

            <Card className="border-white/70 bg-white/88 p-5 shadow-[0_14px_36px_rgba(39,39,42,0.06)]">
              <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
                <CheckCircle2 className="h-4 w-4" />
                处理顺序
              </div>
              <div className="mt-4 grid gap-3">
                {runbook.map((item) => (
                  <div key={item.title} className={cn("rounded-2xl border px-4 py-3", item.tone)}>
                    <div className="font-semibold">{item.title}</div>
                    <div className="mt-1 text-sm leading-6 opacity-90">{item.description}</div>
                  </div>
                ))}
              </div>
            </Card>

            <Card className="border-white/70 bg-white/88 p-5 shadow-[0_14px_36px_rgba(39,39,42,0.06)]">
              <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
                <RefreshCw className={cn("h-4 w-4", accounts.isFetching && "animate-spin")} />
                同步状态
              </div>
              <p className="mt-4 text-sm leading-6 text-zinc-600">{statusNote}</p>
              <div className="mt-4 flex flex-wrap gap-2">
                <Button tone="secondary" className="h-9 px-3" onClick={() => accounts.refetch()} disabled={accounts.isFetching}>
                  立即刷新
                </Button>
                {failureStoreID ? (
                  <Button tone="ghost" className="h-9 px-3" onClick={() => setFailureStoreID(null)}>
                    关闭详情
                  </Button>
                ) : null}
              </div>
            </Card>
          </div>
        </aside>
      </section>

      <VerifyCodeDialog
        open={Boolean(verifyStoreID)}
        storeID={verifyStoreID}
        pending={verifyCode.isPending}
        onClose={() => setVerifyStoreID(null)}
        onSubmit={(code) => {
          if (!verifyStoreID) {
            return;
          }
          verifyCode.mutate(
            { storeID: verifyStoreID, code },
            {
              onSuccess: () => setVerifyStoreID(null),
            },
          );
        }}
      />

      <FailureDialog
        open={Boolean(failureStoreID)}
        storeID={failureStoreID}
        detail={failureDetail.data}
        isLoading={failureDetail.isLoading}
        onClose={() => setFailureStoreID(null)}
      />
    </ListingKitPageShell>
  );
}
