"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState, type ReactNode } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { getCommercialOverview } from "@/lib/api/commercial";
import {
  getCommercialOrder,
  getCommercialOrderSummary,
  getCommercialOrders,
  getCommercialWallet,
  getCommercialWalletEntries,
  type CommercialOrderFilters,
} from "@/lib/api/commercial-billing";
import { getResourceOffers } from "@/lib/api/resource-purchase";
import {
  getResourceEvents,
  type ResourceEventFilters,
} from "@/lib/api/resource-events";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { EntitlementsOverview } from "./commercial-views";
import { EnterpriseResources } from "./enterprise-resources";
import { ResourcePurchasePanel } from "./resource-purchase-panel";
import {
  CommercialOverviewView,
  UsageDetailsView,
} from "./commercial-module-views";
import {
  OrderDetailView,
  OrdersView,
  WalletView,
} from "./commercial-billing-views";
import styles from "./commercial.module.css";
import { TopUpPaymentPanel } from "./wallet-topup";

export type PageKind =
  | "overview"
  | "options"
  | "entitlements"
  | "usage"
  | "top-up"
  | "orders"
  | "order-detail";

export function CommercialPage({
  page,
  orderId,
}: {
  page: PageKind;
  orderId?: string;
}) {
  const context = useWorkbenchContext();
  const organization = context.effectiveOrganization;
  if (context.isSwitching || context.isLoading)
    return (
      <PageFrame page={page} organization="正在确认企业">
        <ConsoleState
          kind="loading"
          title={context.isSwitching ? "正在切换企业" : "正在读取企业上下文"}
        >
          旧商业数据已清空，观察时间与周期尚未取得。
        </ConsoleState>
      </PageFrame>
    );
  if (
    context.error ||
    context.blockingError ||
    !context.user ||
    !organization ||
    context.selectionRequired
  )
    return (
      <PageFrame page={page} organization="未确认">
        <ConsoleState kind="unavailable" title="企业或登录上下文不可用">
          已停止读取商业数据。观察时间、周期与数值尚未取得。
        </ConsoleState>
      </PageFrame>
    );
  const scope = JSON.stringify([
    context.user.id,
    organization.id,
    context.permissions,
    context.permissions,
  ]);
  return (
    <ScopedCommercial
      key={`${page}:${scope}:${orderId ?? ""}`}
      page={page}
      scope={scope}
      userId={context.user.id}
      organizationId={organization.id}
      organizationName={organization.name}
      permissions={context.permissions}
      orderId={orderId}
    />
  );
}

function PageFrame({
  page,
  organization,
  actions,
  children,
}: {
  page: PageKind;
  organization: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const titles: Record<PageKind, string> = {
    overview: "套餐与权益",
    options: "套餐方案",
    entitlements: "我的权益",
    usage: "用量明细",
    "top-up": "充值中心",
    orders: "账单与订单",
    "order-detail": "订单详情",
  };
  const title = titles[page];
  const breadcrumbs =
    page === "overview"
      ? [{ label: title }]
      : page === "order-detail"
        ? [
            { label: "套餐与权益", href: "/workbench/plans" },
            { label: "账单与订单", href: "/workbench/plans/orders" },
            { label: title },
          ]
        : [{ label: "套餐与权益", href: "/workbench/plans" }, { label: title }];
  const descriptions: Record<PageKind, string> = {
    overview: "统一基础方案、店铺服务及企业预付资源。",
    options: "基础方案无需订阅，按当前价格购买店铺期数、AI 点数和数据条数。",
    entitlements: "查看店铺服务期限和企业已购资源。",
    usage: "查看真实资源预留、消费、分配和回收记录。",
    "top-up": "管理企业钱包、充值及按需购买。",
    orders: "查看企业账单汇总与订单记录；退款和发票暂不可用。",
    "order-detail": "查看当前企业的订单状态及明细。",
  };
  return (
    <ConsolePage
      className={styles.page}
      title={title}
      breadcrumbs={breadcrumbs}
      description={
        <>
          <p>{descriptions[page]}</p>
          <p>当前有效企业：{organization}</p>
        </>
      }
      actions={actions}
    >
      {children}
    </ConsolePage>
  );
}

function ScopedCommercial({
  page,
  scope,
  userId,
  organizationId,
  organizationName,
  permissions,
  orderId,
}: {
  page: PageKind;
  scope: string;
  userId: string;
  organizationId: string;
  organizationName: string;
  permissions: string[];
  orderId?: string;
}) {
  const [sequence, setSequence] = useState(0);
  const refresh = () => setSequence((value) => value + 1);
  if (page === "top-up" || page === "orders" || page === "order-detail") {
    return (
      <PageFrame
        page={page}
        organization={`${organizationName || "未提供名称"}（${organizationId}）`}
        actions={
          <Button variant="outline" onClick={refresh}>
            刷新数据
          </Button>
        }
      >
        <BillingRequest
          key={sequence}
          page={page}
          userId={userId}
          organizationId={organizationId}
          permissions={permissions}
          orderId={orderId}
        />
      </PageFrame>
    );
  }
  return (
    <PageFrame
      page={page}
      organization={`${organizationName || "未提供名称"}（${organizationId}）`}
      actions={
        <>
          {page === "overview" ? (
            <Button asChild variant="outline">
              <Link prefetch={false} href="/workbench/plans/entitlements">
                查看我的权益
              </Link>
            </Button>
          ) : (
            <Button asChild variant="outline">
              <Link
                prefetch={false}
                href={
                  page === "options"
                    ? "/workbench/plans/entitlements"
                    : "/workbench/plans/options"
                }
              >
                {page === "options" ? "查看我的权益" : "查看套餐方案"}
              </Link>
            </Button>
          )}
          <Button variant="outline" onClick={refresh}>
            刷新数据
          </Button>
        </>
      }
    >
      {page === "entitlements" ? (
        <EnterpriseResources
          userId={userId}
          organizationId={organizationId}
          scope={scope}
          sequence={sequence}
          showSummary
        />
      ) : null}
      <CommercialRequest
        key={sequence}
        page={page}
        scope={scope}
        userId={userId}
        organizationId={organizationId}
        organizationName={organizationName}
        permissions={permissions}
        sequence={sequence}
      />
    </PageFrame>
  );
}

function CommercialRequest({
  page,
  scope,
  userId,
  organizationId,
  organizationName,
  permissions,
  sequence,
}: {
  page: PageKind;
  scope: string;
  userId: string;
  organizationId: string;
  organizationName: string;
  permissions: string[];
  sequence: number;
}) {
  const response = useQuery({
    queryKey: [
      "workbench",
      organizationId,
      "commercial",
      page,
      scope,
      sequence,
    ],
    queryFn: ({ signal }) => getCommercialOverview(organizationId, signal),
    enabled: page !== "options" && page !== "usage",
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    refetchInterval: false,
  });
  if (page === "options")
    return (
      <ResourceCatalog
        userId={userId}
        organizationId={organizationId}
        organizationName={organizationName}
        permissions={permissions}
        sequence={sequence}
      />
    );
  if (page === "usage")
    return (
      <ResourceUsageRequest
        userId={userId}
        organizationId={organizationId}
        sequence={sequence}
      />
    );
  if (response.isPending || response.isFetching)
    return (
      <ConsoleState kind="loading" title="正在读取商业数据">
        正在校验当前企业权限，旧结果已隐藏。观察时间与周期尚未取得。
      </ConsoleState>
    );
  if (response.isError) return <ReadError error={response.error} />;
  if (!response.data || response.data.organization_id !== organizationId)
    return <ReadError error={{ code: "INVALID_UPSTREAM_RESPONSE" }} />;
  if (page === "overview")
    return <CommercialOverviewView data={response.data} />;
  return <EntitlementsOverview data={response.data} />;
}

function BillingRequest({
  page,
  userId,
  organizationId,
  orderId,
  permissions,
}: {
  page: "top-up" | "orders" | "order-detail";
  userId: string;
  organizationId: string;
  orderId?: string;
  permissions: string[];
}) {
  const [filters, setFilters] = useState<CommercialOrderFilters>({});
  const [cursor, setCursor] = useState("");
  const [walletCursor, setWalletCursor] = useState("");
  const wallet = useQuery({
    queryKey: ["workbench", organizationId, "commercial-wallet", userId],
    queryFn: ({ signal }) =>
      getCommercialWallet(userId, organizationId, signal),
    enabled: page === "top-up",
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const entries = useQuery({
    queryKey: [
      "workbench",
      organizationId,
      "commercial-wallet-entries",
      userId,
      walletCursor,
    ],
    queryFn: ({ signal }) =>
      getCommercialWalletEntries(
        userId,
        organizationId,
        signal,
        walletCursor || undefined,
      ),
    enabled: page === "top-up",
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const summary = useQuery({
    queryKey: ["workbench", organizationId, "commercial-order-summary", userId],
    queryFn: ({ signal }) =>
      getCommercialOrderSummary(userId, organizationId, signal),
    enabled: page === "orders",
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const orders = useQuery({
    queryKey: [
      "workbench",
      organizationId,
      "commercial-orders",
      userId,
      filters,
      cursor,
    ],
    queryFn: ({ signal }) =>
      getCommercialOrders(
        userId,
        organizationId,
        { ...filters, cursor: cursor || undefined },
        signal,
      ),
    enabled: page === "orders",
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const detail = useQuery({
    queryKey: [
      "workbench",
      organizationId,
      "commercial-order",
      userId,
      orderId,
    ],
    queryFn: ({ signal }) =>
      getCommercialOrder(userId, organizationId, orderId ?? "", signal),
    enabled: page === "order-detail" && Boolean(orderId),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  if (page === "top-up") {
    if (wallet.isPending || entries.isPending)
      return (
        <ConsoleState kind="loading" title="正在读取企业钱包">
          钱包余额与流水仅来自当前企业 owner。
        </ConsoleState>
      );
    if (wallet.isError) return <BillingReadError error={wallet.error} />;
    if (entries.isError) return <BillingReadError error={entries.error} />;
    if (!wallet.data || !entries.data)
      return (
        <ConsoleState kind="error" title="钱包响应无效">
          未能校验当前企业钱包响应。
        </ConsoleState>
      );
    return (
      <WalletView
        wallet={wallet.data}
        entries={entries.data}
        userId={userId}
        permissions={permissions}
        onNext={() => setWalletCursor(entries.data.next_cursor)}
        purchase={
          <ResourceCatalog
            userId={userId}
            organizationId={organizationId}
            organizationName="当前企业"
            permissions={permissions}
            sequence={0}
          />
        }
      />
    );
  }
  if (page === "orders") {
    if (summary.isPending || orders.isPending)
      return (
        <ConsoleState kind="loading" title="正在读取账单与订单">
          账单金额、状态和明细仅来自当前企业账单 owner。
        </ConsoleState>
      );
    if (summary.isError) return <BillingReadError error={summary.error} />;
    if (orders.isError) return <BillingReadError error={orders.error} />;
    if (!summary.data || !orders.data)
      return (
        <ConsoleState kind="error" title="账单响应无效">
          未能校验当前企业账单响应。
        </ConsoleState>
      );
    return (
      <OrdersView
        summary={summary.data}
        page={orders.data}
        onFilter={(value) => {
          setFilters({
            query: value.query || undefined,
            kind: (value.kind as CommercialOrderFilters["kind"]) || undefined,
            status:
              (value.status as CommercialOrderFilters["status"]) || undefined,
            from: value.from || undefined,
            until: value.until || undefined,
          });
          setCursor("");
        }}
        onNext={() => setCursor(orders.data.next_cursor)}
      />
    );
  }
  if (!orderId)
    return (
      <ConsoleState kind="error" title="订单编号无效">
        无法读取未提供编号的订单详情。
      </ConsoleState>
    );
  if (detail.isPending)
    return (
      <ConsoleState kind="loading" title="正在读取订单详情">
        仅展示当前企业 owner 返回的订单事实。
      </ConsoleState>
    );
  if (detail.isError) return <BillingReadError error={detail.error} />;
  return detail.data ? (
    <>
      {detail.data.kind === "WALLET_TOP_UP" ? (
        <TopUpPaymentPanel
          key={detail.data.order_id}
          userId={userId}
          organizationId={organizationId}
          permissions={permissions}
          initialOrder={detail.data}
        />
      ) : null}
      <OrderDetailView order={detail.data} />
    </>
  ) : (
    <ConsoleState kind="error" title="订单响应无效">
      未能校验当前企业订单响应。
    </ConsoleState>
  );
}

function BillingReadError({ error }: { error: unknown }) {
  const code =
    error &&
    typeof error === "object" &&
    "code" in error &&
    typeof error.code === "string"
      ? error.code
      : "UNKNOWN";
  const messages: Record<string, string> = {
    PERMISSION_DENIED: "无查看权限",
    AUTHENTICATION_REQUIRED: "登录已失效",
    IDENTITY_CONTEXT_CHANGED: "登录身份已变化",
    ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销",
    ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝",
    ORGANIZATION_SUSPENDED: "企业已暂停访问",
    ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化",
    ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业",
    DEPENDENCY_UNAVAILABLE: "账单服务暂不可用",
    FEATURE_UNAVAILABLE: "该商业能力暂未开放",
    DEADLINE_EXCEEDED: "账单读取超时",
    INVALID_UPSTREAM_RESPONSE: "账单服务响应无效",
  };
  return (
    <ConsoleState kind="error" title={messages[code] ?? "商业数据读取失败"}>
      <p>本次未取得当前企业的数据；不会回退到示例账单、余额或订单。</p>
      <p>重新确认企业上下文后可重试读取。</p>
    </ConsoleState>
  );
}

function ReadError({ error }: { error: unknown }) {
  const code =
    error &&
    typeof error === "object" &&
    "code" in error &&
    typeof error.code === "string"
      ? error.code
      : "UNKNOWN";
  const messages: Record<string, string> = {
    PERMISSION_DENIED: "无查看权限",
    AUTHENTICATION_REQUIRED: "登录已失效",
    ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销",
    ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝",
    ORGANIZATION_SUSPENDED: "企业已暂停访问",
    ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化",
    ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业",
    INVALID_REQUEST: "商业数据请求无效",
    DEPENDENCY_UNAVAILABLE: "商业数据依赖暂不可用",
    FEATURE_UNAVAILABLE: "该商业能力暂未开放",
    DEADLINE_EXCEEDED: "商业数据读取超时",
    INVALID_UPSTREAM_RESPONSE: "商业数据响应无效",
  };
  return (
    <ConsoleState kind="error" title={messages[code] ?? "商业数据读取失败"}>
      <p>本次未取得基础方案和店铺服务摘要。企业资源余额以上方读取结果为准。</p>
      <p>可重新选择企业或刷新数据，再由服务端确认访问权限。</p>
    </ConsoleState>
  );
}

function ResourceCatalog({
  userId,
  organizationId,
  organizationName,
  permissions,
  sequence,
}: {
  userId: string;
  organizationId: string;
  organizationName: string;
  permissions: string[];
  sequence: number;
}) {
  const result = useQuery({
    queryKey: [
      "workbench",
      organizationId,
      "resource-offers",
      userId,
      sequence,
    ],
    queryFn: ({ signal }) => getResourceOffers(userId, organizationId, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  if (result.isPending || result.isFetching)
    return (
      <ConsoleState kind="loading" title="正在读取资源价格">
        正在确认当前企业的价格配置。
      </ConsoleState>
    );
  if (result.isError) return <BillingReadError error={result.error} />;
  if (!result.data || result.data.organization_id !== organizationId)
    return <BillingReadError error={{ code: "INVALID_UPSTREAM_RESPONSE" }} />;
  return (
    <ResourcePurchasePanel
      userId={userId}
      organizationId={organizationId}
      organizationName={organizationName}
      permissions={permissions}
      offers={result.data}
    />
  );
}
function ResourceUsageRequest({
  userId,
  organizationId,
  sequence,
}: {
  userId: string;
  organizationId: string;
  sequence: number;
}) {
  const [filters, setFilters] = useState<ResourceEventFilters>({});
  const [cursor, setCursor] = useState("");
  const result = useQuery({
    queryKey: [
      "workbench",
      organizationId,
      "resource-events",
      userId,
      sequence,
      filters,
      cursor,
    ],
    queryFn: ({ signal }) =>
      getResourceEvents(
        userId,
        organizationId,
        { ...filters, cursor: cursor || undefined },
        signal,
      ),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  if (result.isPending || result.isFetching)
    return (
      <ConsoleState kind="loading" title="正在读取资源流水">
        旧流水已隐藏。
      </ConsoleState>
    );
  if (result.isError) return <BillingReadError error={result.error} />;
  if (!result.data || result.data.organization_id !== organizationId)
    return <BillingReadError error={{ code: "INVALID_UPSTREAM_RESPONSE" }} />;
  return (
    <UsageDetailsView
      page={result.data}
      filters={filters}
      onFilter={(v) => {
        setFilters(v);
        setCursor("");
      }}
      onNext={() => setCursor(result.data.next_cursor ?? "")}
    />
  );
}
