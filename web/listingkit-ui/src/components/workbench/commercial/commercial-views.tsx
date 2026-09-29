import Link from "next/link";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { CommercialOverview } from "@/lib/api/commercial";
import styles from "./commercial.module.css";
export function EntitlementsOverview({ data }: { data: CommercialOverview }) {
  const stores =
    data.store_services.state === "available"
      ? data.store_services.value
      : null;
  return (
    <div className={styles.stack}>
      <Card role="region" aria-label="基础方案" className={styles.panel}>
        <h2>{data.base_plan.name}</h2>
        <p>无需购买或激活订阅；按实际需要购买店铺期数、AI 点数和数据条数。</p>
        <p>企业已购资源不按月清零；成员 AI 消费上限按 UTC 自然月管理。</p>
      </Card>
      <Card role="region" aria-label="店铺服务权益" className={styles.panel}>
        <h2>店铺服务权益</h2>
        {stores ? (
          <dl className={styles.facts}>
            <div>
              <dt>店铺记录</dt>
              <dd>{stores.records} 家</dd>
            </div>
            <div>
              <dt>生效服务</dt>
              <dd>{stores.active} 家</dd>
            </div>
            <div>
              <dt>已到期</dt>
              <dd>{stores.expired} 家</dd>
            </div>
            <div>
              <dt>7 天内到期</dt>
              <dd>{stores.expiring_soon} 家</dd>
            </div>
          </dl>
        ) : (
          <p role="alert">店铺服务统计暂不可用，请刷新重试。</p>
        )}
        <p>
          购买期数不会自动连接店铺。请先授权平台，再用分配的期数开通或续费，每期
          30 天。
        </p>
        <Button asChild variant="outline">
          <Link href="/workbench/stores" prefetch={false}>
            管理店铺服务
          </Link>
        </Button>
      </Card>
      <Card role="region" aria-label="成员资源管理" className={styles.panel}>
        <h2>成员资源管理</h2>
        <p>
          管理员可授权具体店铺、分配或回收店铺期数及数据条数，并设置成员 AI
          月度消费上限。平台权限仍由真实授权决定。
        </p>
        <Button asChild variant="outline">
          <Link
            href="/workbench/account/organization/resources"
            prefetch={false}
          >
            管理资源与额度
          </Link>
        </Button>
      </Card>
      <p className={styles.observation}>
        观察时间：{data.observed_at.replace("T", " ").replace("Z", " UTC")}
      </p>
    </div>
  );
}
