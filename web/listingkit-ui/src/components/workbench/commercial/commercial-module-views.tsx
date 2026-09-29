"use client";
import Link from "next/link";
import { useState, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { CommercialOverview } from "@/lib/api/commercial";
import type {
  ResourceEventPage,
  ResourceEventFilters,
} from "@/lib/api/resource-events";
import { resourceDefinitions } from "./resource-purchase-panel";
import styles from "./commercial.module.css";
function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card role="region" aria-label={title} className={styles.panel}>
      <h2>{title}</h2>
      {children}
    </Card>
  );
}
function PageLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Button asChild variant="outline" size="sm">
      <Link href={href} prefetch={false}>
        {children}
      </Link>
    </Button>
  );
}
export function CommercialOverviewView({ data }: { data: CommercialOverview }) {
  const stores =
    data.store_services.state === "available"
      ? data.store_services.value
      : null;
  return (
    <div className={styles.stack}>
      <Panel title="当前方案">
        <p className={styles.eyebrow}>统一基础方案 · 按需付费</p>
        <h3>{data.base_plan.name}</h3>
        <p>基础方案无需订阅。店铺按期收费，AI 点数与数据资源预付使用。</p>
        <div className={styles.summaryMetrics}>
          <div className={`${styles.summaryMetric} ${styles.blue}`}>
            <p>店铺服务</p>
            <strong>{stores ? `${stores.active} 家生效中` : "暂不可用"}</strong>
            <small>
              {stores
                ? `${stores.expired} 家已到期 · ${stores.expiring_soon} 家 7 天内到期`
                : "未取得店铺服务统计"}
            </small>
          </div>
          {resourceDefinitions
            .filter((d) => d.type !== "store_renewal_period")
            .map((def) => {
              const balance =
                data.resources.state === "available"
                  ? data.resources.value.resources.find(
                      (r) => r.resource_type === def.type,
                    )
                  : null;
              return (
                <div
                  key={def.type}
                  className={`${styles.summaryMetric} ${styles.teal}`}
                >
                  <p>{def.name}</p>
                  <strong>
                    {balance?.state === "recorded"
                      ? `${BigInt(balance.available).toLocaleString("zh-CN")} ${def.unit}`
                      : balance?.state === "not_recorded"
                        ? "尚无余额记录"
                        : "暂不可用"}
                  </strong>
                  <small>企业可用余额 · 不按月清零</small>
                </div>
              );
            })}
        </div>
        <div className={styles.actions}>
          <PageLink href="/workbench/plans/options">购买资源</PageLink>
          <PageLink href="/workbench/plans/entitlements">查看我的权益</PageLink>
        </div>
      </Panel>
      <div className={styles.sectionHeading}>
        <h2>管理入口</h2>
      </div>
      <div className={styles.moduleEntries}>
        {[
          ["套餐方案", "基础方案与资源价格", "/workbench/plans/options"],
          [
            "我的权益",
            "店铺服务与企业资源余额",
            "/workbench/plans/entitlements",
          ],
          ["用量明细", "真实预留与消费流水", "/workbench/plans/usage"],
          ["充值中心", "企业钱包与按需购买", "/workbench/plans/top-up"],
          ["账单与订单", "充值及购买订单", "/workbench/plans/orders"],
        ].map(([title, detail, href]) => (
          <Card key={href} className={styles.moduleEntry}>
            <h3>{title}</h3>
            <p>{detail}</p>
            <PageLink href={href}>进入</PageLink>
          </Card>
        ))}
      </div>
      <Panel title="计费规则">
        <p>
          店铺 1 期为 30
          天，须先完成真实平台连接，再开通或续费服务。续费不会改变平台连接状态。
        </p>
        <p>
          图片和模型调用统一扣 AI
          点数；调用前预留，结果未知时保留预留。数据按采集成功入库的商品结果扣条数。
        </p>
        <p>
          成员 AI 月度消费上限按 UTC 自然月计算；企业已购点数和数据余额不清零。
        </p>
        <PageLink href="/workbench/account/organization/resources">
          管理成员资源与额度
        </PageLink>
      </Panel>
      <p className={styles.observation}>
        观察时间：{data.observed_at.replace("T", " ").replace("Z", " UTC")}
      </p>
    </div>
  );
}
const reasons: Record<string, string> = {
  purchase: "购买到账",
  allocated: "分配",
  reclaimed: "回收",
  reserved: "预留",
  consumed: "消费",
  released: "释放",
  grant_commercial_purchase: "购买到账",
  allocate_member_resource: "分配成员资源",
  reclaim_member_resource: "回收成员资源",
  reserve: "预留",
  commit: "消费",
  release: "释放",
  model_point_reserve: "模型调用预留",
  model_point_committed: "模型调用消费",
  model_point_released: "释放模型调用预留",
};
function quantity(value: string) {
  const n = BigInt(value);
  return `${n > BigInt(0) ? "+" : ""}${n.toLocaleString("zh-CN")}`;
}
function boundary(value: string, next = false) {
  return new Date(`${value}T00:00:00Z`).getTime() + (next ? 86400000 : 0);
}
export function UsageDetailsView({
  page,
  filters = {},
  onFilter,
  onNext,
}: {
  page: ResourceEventPage;
  filters?: ResourceEventFilters;
  onFilter: (v: ResourceEventFilters) => void;
  onNext: () => void;
}) {
  const [type, setType] = useState(filters.resourceType ?? "");
  const [from, setFrom] = useState(filters.from?.slice(0, 10) ?? "");
  const [until, setUntil] = useState(
    filters.until
      ? new Date(new Date(filters.until).getTime() - 86400000)
          .toISOString()
          .slice(0, 10)
      : "",
  );
  const [invalid, setInvalid] = useState(false);
  return (
    <div className={styles.stack}>
      <Panel title="流水明细">
        <p>
          每条记录来自企业资源账本。点数、期数和条数分别计量；资金金额请查看账单。
        </p>
        <form
          className={styles.filterBar}
          onSubmit={(e) => {
            e.preventDefault();
            if (from && until && boundary(from) >= boundary(until, true)) {
              setInvalid(true);
              return;
            }
            setInvalid(false);
            onFilter({
              resourceType:
                (type as ResourceEventFilters["resourceType"]) || undefined,
              from: from ? new Date(boundary(from)).toISOString() : undefined,
              until: until
                ? new Date(boundary(until, true)).toISOString()
                : undefined,
            });
          }}
        >
          <label>
            开始日期（UTC）
            <input
              type="date"
              aria-label="开始日期（UTC）"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
            />
          </label>
          <label>
            结束日期（UTC）
            <input
              type="date"
              aria-label="结束日期（UTC）"
              value={until}
              onChange={(e) => setUntil(e.target.value)}
            />
          </label>
          <label>
            类型
            <select
              aria-label="用量类型"
              value={type}
              onChange={(e) => setType(e.target.value)}
            >
              <option value="">全部类型</option>
              {resourceDefinitions.map((d) => (
                <option key={d.type} value={d.type}>
                  {d.name}
                </option>
              ))}
            </select>
          </label>
          <Button type="submit">筛选</Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              setType("");
              setFrom("");
              setUntil("");
              setInvalid(false);
              onFilter({});
            }}
          >
            重置
          </Button>
        </form>
        {invalid ? <p role="alert">结束日期不能早于开始日期。</p> : null}
        {page.items.length === 0 ? (
          <p>暂无匹配的资源流水。</p>
        ) : (
          <div className={styles.usageTableWrap}>
            <table className={styles.usageTable}>
              <caption className="sr-only">企业资源流水</caption>
              <thead>
                <tr>
                  <th>时间（UTC）</th>
                  <th>资源</th>
                  <th>操作</th>
                  <th>可用变动</th>
                  <th>分配变动</th>
                  <th>预留变动</th>
                  <th>消费变动</th>
                  <th>关联记录</th>
                </tr>
              </thead>
              <tbody>
                {page.items.map((row) => (
                  <tr key={row.event_id}>
                    <td>
                      {row.occurred_at.replace("T", " ").replace("Z", "")}
                    </td>
                    <td>
                      {
                        resourceDefinitions.find(
                          (d) => d.type === row.resource_type,
                        )?.name
                      }
                    </td>
                    <td>{reasons[row.reason] ?? row.reason}</td>
                    <td>{quantity(row.available_delta)}</td>
                    <td>{quantity(row.allocated_delta)}</td>
                    <td>{quantity(row.reserved_delta)}</td>
                    <td>{quantity(row.consumed_delta)}</td>
                    <td>{row.source_identity}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className={styles.pagination}>
          <span>当前页 {page.items.length} 条 · 每页最多 50 条</span>
          {page.next_cursor ? (
            <Button variant="outline" onClick={onNext}>
              下一页流水
            </Button>
          ) : null}
        </div>
      </Panel>
    </div>
  );
}
