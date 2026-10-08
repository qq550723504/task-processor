"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getCommercialOrder } from "@/lib/api/commercial-billing";
import {
  cancelTopUp,
  checkoutTopUp,
  createTopUp,
  getTopUpOptions,
  paymentProvider,
  topUpMoney,
  yuanToMinor,
  WalletTopUpError,
  type PaymentProvider,
  type TopUpCheckout,
  type TopUpOrder,
} from "@/lib/api/wallet-topup";
import styles from "./wallet-topup.module.css";

const pendingSchema = z
  .object({
    key: z.string().uuid(),
    provider: paymentProvider,
    amount: z.string().regex(/^[1-9][0-9]{0,18}$/),
    orderId: z.string().uuid().optional(),
  })
  .strict();
type Pending = z.infer<typeof pendingSchema>;
const storageKey = (user: string, org: string) =>
  `wallet-topup.pending:${JSON.stringify([user, org])}`;
function subscribe(callback: () => void) {
  window.addEventListener("wallet-topup-pending", callback);
  window.addEventListener("storage", callback);
  return () => {
    window.removeEventListener("wallet-topup-pending", callback);
    window.removeEventListener("storage", callback);
  };
}
function readPending(raw: string): Pending | null {
  try {
    const p = pendingSchema.safeParse(JSON.parse(raw));
    return p.success ? p.data : null;
  } catch {
    return null;
  }
}
function persist(user: string, org: string, value: Pending | null) {
  if (value)
    sessionStorage.setItem(storageKey(user, org), JSON.stringify(value));
  else sessionStorage.removeItem(storageKey(user, org));
  window.dispatchEvent(new Event("wallet-topup-pending"));
}
const labels: Record<string, string> = {
  AUTHENTICATION_REQUIRED: "登录已失效，请重新登录。",
  FORBIDDEN: "当前充值权限已撤销。",
  PERMISSION_DENIED: "当前身份无充值权限。",
  ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销。",
  ORGANIZATION_CONTEXT_CHANGED: "企业已切换，请回到原企业查询订单。",
  IDENTITY_CONTEXT_CHANGED: "登录身份已变化。",
  PAYMENT_METHOD_UNAVAILABLE: "此支付渠道暂不可用。",
  INVALID_REQUEST: "金额或请求无效，请检查充值范围。",
  IDEMPOTENCY_CONFLICT: "订单状态已变化，请刷新原订单。",
  RECONCILIATION_REQUIRED: "正在核对原订单，请勿重复付款。",
  DEADLINE_EXCEEDED: "请求超时，结果尚未确认。请查询原订单。",
  NOT_FOUND: "未找到当前企业的原订单。",
};
function errorMessage(e: unknown) {
  const code = e && typeof e === "object" && "code" in e ? String(e.code) : "";
  return labels[code] ?? "服务暂不可用，请查询原订单，勿重复提交。";
}
function canManage(permissions: string[]) {
  return permissions.includes("workbench.commercial.wallet_topup");
}

export function WalletTopUpEntry({
  userId,
  organizationId,
  permissions,
}: {
  userId: string;
  organizationId: string;
  permissions: string[];
}) {
  const router = useRouter();
  const dialog = useRef<HTMLDialogElement>(null);
  const active = useRef<AbortController | null>(null);
  const inFlight = useRef(false);
  const raw = useSyncExternalStore(
    subscribe,
    () => {
      try {
        return sessionStorage.getItem(storageKey(userId, organizationId)) ?? "";
      } catch {
        return "__unavailable__";
      }
    },
    () => "__loading__",
  );
  const pending = readPending(raw);
  const storageReady = raw !== "__loading__" && (!raw || !!pending);
  const options = useQuery({
    queryKey: ["workbench", organizationId, "wallet-topup-options", userId],
    queryFn: ({ signal }) => getTopUpOptions(userId, organizationId, signal),
    retry: false,
    gcTime: 0,
    staleTime: 0,
  });
  const [amount, setAmount] = useState("");
  const [provider, setProvider] = useState<PaymentProvider | "">("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  useEffect(() => () => active.current?.abort(), []);
  const data = options.data;
  const allowed = canManage(permissions);
  const available = !options.isError && data?.channels.some((c) => c.available);
  const minor = yuanToMinor(amount);
  const amountValid =
    minor &&
    data &&
    BigInt(minor) >= BigInt(data.min_minor) &&
    BigInt(minor) <= BigInt(data.max_minor);
  async function submit(original?: Pending) {
    if (
      inFlight.current ||
      !allowed ||
      !storageReady ||
      (!original &&
        (!minor ||
          !amountValid ||
          !provider ||
          !data?.channels.some((c) => c.provider === provider && c.available)))
    )
      return;
    const intent = original ?? {
      key: crypto.randomUUID(),
      provider: provider as PaymentProvider,
      amount: minor!,
    };
    try {
      persist(userId, organizationId, intent);
    } catch {
      setMessage("无法保存原充值意图，尚未发送充值请求。");
      return;
    }
    inFlight.current = true;
    setBusy(true);
    setMessage("");
    const controller = new AbortController();
    active.current = controller;
    try {
      const order = await createTopUp(
        userId,
        organizationId,
        intent.provider,
        intent.amount,
        intent.key,
        controller.signal,
      );
      if (controller.signal.aborted) return;
      persist(userId, organizationId, { ...intent, orderId: order.order_id });
      dialog.current?.close();
      router.push(
        `/workbench/plans/orders/${encodeURIComponent(order.order_id)}`,
      );
    } catch (e) {
      if (!controller.signal.aborted) {
        if (
          e instanceof WalletTopUpError &&
          [
            "INVALID_REQUEST",
            "PAYMENT_METHOD_UNAVAILABLE",
            "FEATURE_UNAVAILABLE",
          ].includes(e.code)
        )
          persist(userId, organizationId, null);
        setMessage(errorMessage(e));
      }
    } finally {
      if (!controller.signal.aborted) {
        inFlight.current = false;
        setBusy(false);
      }
    }
  }
  return (
    <div className={styles.entry}>
      <Button
        disabled={
          !available ||
          !allowed ||
          !storageReady ||
          !!pending ||
          options.isFetching
        }
        onClick={() => {
          setMessage("");
          dialog.current?.showModal();
        }}
      >
        充值钱包
      </Button>
      {!allowed ? (
        <p>仅企业管理员可发起充值。</p>
      ) : !available ? (
        <p>
          {options.isPending
            ? "正在读取充值渠道…"
            : "充值暂未开放：支付渠道或金额配置尚未就绪。"}
        </p>
      ) : null}
      {!storageReady && raw !== "__loading__" ? (
        <p role="alert">原充值意图无法读取，请先在账单与订单中核对记录。</p>
      ) : null}
      {pending ? (
        <div role="status">
          <p>
            有一笔原充值意图待确认：¥{topUpMoney(pending.amount)} ·{" "}
            {pending.provider === "ALIPAY" ? "支付宝" : "微信支付"}
          </p>
          {pending.orderId ? (
            <Link
              href={`/workbench/plans/orders/${encodeURIComponent(pending.orderId)}`}
              prefetch={false}
            >
              查询原订单
            </Link>
          ) : (
            <Button
              variant="outline"
              disabled={busy || !allowed}
              onClick={() => void submit(pending)}
            >
              恢复原订单
            </Button>
          )}
        </div>
      ) : null}
      {message ? <p role="alert">{message}</p> : null}
      <dialog
        ref={dialog}
        className={styles.dialog}
        aria-labelledby="topup-title"
        aria-describedby="topup-description"
        onCancel={(event) => {
          if (busy) event.preventDefault();
        }}
      >
        <form
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <h2 id="topup-title">充值钱包</h2>
          <p id="topup-description" className={styles.description}>
            充值金额将进入当前企业钱包。有欠款时先偿债，渠道手续费由平台承担。充值不会自动开通套餐。
          </p>
          <label className={styles.field}>
            充值金额
            <Input
              inputMode="decimal"
              autoComplete="off"
              placeholder="请输入充值金额（元）"
              value={amount}
              disabled={busy || !!pending}
              onChange={(e) => setAmount(e.target.value)}
              maxLength={22}
            />
          </label>
          <p className={styles.range}>
            {data && data.min_minor !== "0"
              ? `单笔 ¥${topUpMoney(data.min_minor)} – ¥${topUpMoney(data.max_minor)}`
              : "金额配置暂不可用"}
          </p>
          <fieldset disabled={busy || !!pending}>
            <legend>快捷金额</legend>
            <div className={styles.amounts}>
              {data?.quick_amounts_minor.map((value) => (
                <Button
                  key={value}
                  type="button"
                  variant="outline"
                  aria-pressed={minor === value}
                  onClick={() => setAmount(topUpMoney(value))}
                >
                  ¥{topUpMoney(value)}
                </Button>
              ))}
            </div>
          </fieldset>
          <fieldset disabled={busy || !!pending}>
            <legend>支付方式</legend>
            <div className={styles.channels}>
              {data?.channels.map((channel) => (
                <label key={channel.provider}>
                  <input
                    type="radio"
                    name="payment-provider"
                    value={channel.provider}
                    checked={provider === channel.provider}
                    disabled={!channel.available}
                    onChange={() => setProvider(channel.provider)}
                  />
                  <strong>
                    {channel.provider === "ALIPAY" ? "支付宝" : "微信支付"}
                  </strong>
                  <span>
                    {!channel.available
                      ? "暂不可用"
                      : channel.provider === "ALIPAY"
                        ? "电脑网页付款"
                        : "手机微信扫码"}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>
          {message ? <p role="alert">{message}</p> : null}
          <div className={styles.actions}>
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => dialog.current?.close()}
            >
              取消
            </Button>
            <Button
              type="submit"
              disabled={
                busy || !!pending || !amountValid || !provider || !available
              }
            >
              {busy ? "正在创建订单…" : "确认充值"}
            </Button>
          </div>
        </form>
      </dialog>
    </div>
  );
}

const phaseLabels = {
  CREATED: "待付款",
  AWAITING_PAYMENT: "等待付款确认",
  RECONCILIATION_REQUIRED: "支付结果核对中",
  PAID_PENDING_CREDIT: "付款已收到，入账处理中",
  COMPLETED: "钱包入账处理完成",
  CLOSED_UNPAID: "未付款订单已关闭",
};
export function TopUpPaymentPanel({
  userId,
  organizationId,
  permissions,
  initialOrder,
}: {
  userId: string;
  organizationId: string;
  permissions: string[];
  initialOrder: TopUpOrder;
}) {
  const client = useQueryClient();
  const active = useRef<AbortController | null>(null);
  const lock = useRef(false);
  const [busy, setBusy] = useState(false);
  const [action, setAction] = useState<TopUpCheckout | null>(null);
  const [message, setMessage] = useState("");
  const [pollUntil] = useState(() => Date.now() + 10 * 60 * 1000);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  const order = useQuery({
    queryKey: [
      "workbench",
      organizationId,
      "commercial-order",
      userId,
      initialOrder.order_id,
    ],
    queryFn: async ({ signal }) => {
      try {
        const value = await getCommercialOrder(
          userId,
          organizationId,
          initialOrder.order_id,
          signal,
        );
        if (value.kind !== "WALLET_TOP_UP" || !value.top_up)
          throw new Error("invalid topup order");
        if (
          [
            "COMPLETED",
            "CLOSED_UNPAID",
            "PAID_PENDING_CREDIT",
            "RECONCILIATION_REQUIRED",
          ].includes(value.top_up.phase) ||
          value.top_up.close_requested
        )
          setAction(null);
        return value;
      } catch (e) {
        setAction(null);
        throw e;
      }
    },
    initialData: initialOrder,
    retry: false,
    gcTime: 0,
    staleTime: 0,
    refetchInterval: (query) =>
      query.state.error ||
      ["COMPLETED", "CLOSED_UNPAID"].includes(
        query.state.data?.top_up?.phase ?? "",
      ) ||
      Date.now() > pollUntil
        ? false
        : 3000,
    refetchIntervalInBackground: false,
  });
  const value = order.data;
  const attempt = value.top_up;
  const terminal =
    attempt?.phase === "COMPLETED" || attempt?.phase === "CLOSED_UNPAID";
  useEffect(() => () => active.current?.abort(), []);
  useEffect(() => {
    if (!action) return;
    const timer = setTimeout(
      () => setAction(null),
      Math.max(0, Date.parse(action.expires_at) - Date.now()),
    );
    return () => clearTimeout(timer);
  }, [action]);
  useEffect(() => {
    if (!terminal) return;
    try {
      const pending = readPending(
        sessionStorage.getItem(storageKey(userId, organizationId)) ?? "",
      );
      if (pending?.orderId === value.order_id)
        persist(userId, organizationId, null);
    } catch {
      /* Storage is optional when reading a completed order. */
    }
    if (attempt?.phase === "COMPLETED") {
      void client.invalidateQueries({
        queryKey: ["workbench", organizationId, "commercial-wallet", userId],
      });
      void client.invalidateQueries({
        queryKey: [
          "workbench",
          organizationId,
          "commercial-wallet-entries",
          userId,
        ],
      });
    }
  }, [
    terminal,
    attempt?.phase,
    userId,
    organizationId,
    value.order_id,
    client,
  ]);
  async function run(kind: "checkout" | "cancel") {
    if (lock.current || !canManage(permissions) || order.isError || !attempt) return;
    lock.current = true;
    setBusy(true);
    setMessage("");
    setAction(null);
    const controller = new AbortController();
    active.current = controller;
    try {
      if (kind === "checkout") {
        const result = await checkoutTopUp(
          userId,
          organizationId,
          value,
          controller.signal,
        );
        if (!controller.signal.aborted) setAction(result);
      } else {
        await cancelTopUp(userId, organizationId, value, controller.signal);
      }
    } catch (e) {
      if (!controller.signal.aborted) setMessage(errorMessage(e));
    } finally {
      if (!controller.signal.aborted) {
        lock.current = false;
        setBusy(false);
        void order.refetch();
      }
    }
  }
  const paymentVisible =
    !order.isError &&
    !terminal &&
    attempt &&
    ["CREATED", "AWAITING_PAYMENT"].includes(attempt.phase) &&
    !attempt.close_requested &&
    canManage(permissions);
  const showAction =
    paymentVisible &&
    action &&
    action.attempt_id === attempt.attempt_id &&
    Date.parse(action.expires_at) > now;
  return (
    <section className={styles.payment} aria-label="充值付款">
      <h2>
        {order.isError
          ? "无法读取当前支付状态"
          : attempt
            ? phaseLabels[attempt.phase]
            : "充值状态暂不可用"}
      </h2>
      <p>
        付款金额 ¥{topUpMoney(value.total_minor)} ·{" "}
        {attempt?.provider === "ALIPAY" ? "支付宝" : "微信支付"}
      </p>
      {attempt ? (
        <p>
          原付款期限：{new Date(attempt.expires_at).toLocaleString("zh-CN")}
          。重新读取不会延长期限。
        </p>
      ) : null}
      {attempt?.late_payment_corrected ? (
        <p>关闭后收到的可信付款已按原订单归入当前企业。</p>
      ) : null}
      {attempt?.phase === "COMPLETED" ? (
        <p>
          请到充值中心查看企业余额与流水。有欠款时先偿债，退款按原订单核对；套餐仍需单独确认购买。
        </p>
      ) : (
        <p>付款页面返回或手动刷新都不代表到账，请等待订单入账结果。</p>
      )}
      {showAction ? (
        action.kind === "QR_CODE" ? (
          <div className={styles.qr}>
            <QRCodeSVG
              value={action.payload}
              size={220}
              marginSize={4}
              title="请用手机微信扫描原订单付款码"
            />
            <p>请用手机微信扫码。手机浏览器暂不支持直接唤起支付。</p>
          </div>
        ) : (
          <Button asChild>
            <a
              href={action.payload}
              target="_blank"
              rel="noopener noreferrer"
              referrerPolicy="no-referrer"
            >
              前往支付宝付款
            </a>
          </Button>
        )
      ) : null}
      <div className={styles.actions}>
        {paymentVisible && !showAction ? (
          <Button
            disabled={
              busy || order.isFetching || Date.parse(attempt.expires_at) <= now
            }
            onClick={() => void run("checkout")}
          >
            {attempt.provider === "ALIPAY"
              ? "获取支付宝付款入口"
              : "显示微信付款码"}
          </Button>
        ) : null}
        {!terminal &&
        attempt &&
        !attempt.close_requested &&
        canManage(permissions) ? (
          <Button
            variant="outline"
            disabled={busy || order.isFetching || order.isError}
            onClick={() => void run("cancel")}
          >
            取消付款
          </Button>
        ) : null}
        <Button
          variant="outline"
          disabled={busy || order.isFetching}
          onClick={() => {
            setAction(null);
            void order.refetch();
          }}
        >
          查询原订单
        </Button>
        <Button variant="outline" asChild>
          <Link href="/workbench/plans/top-up" prefetch={false}>
            查看企业钱包
          </Link>
        </Button>
      </div>
      {message || order.isError ? (
        <p role="alert">{message || errorMessage(order.error)}</p>
      ) : null}
    </section>
  );
}
