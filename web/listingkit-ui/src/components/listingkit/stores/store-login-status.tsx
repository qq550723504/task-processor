"use client";

import { Badge } from "@/components/ui/badge";
import type { ListingStore } from "@/lib/api/admin-stores";
import type { SheinLoginAccountStatus } from "@/lib/types/shein-login";

export function sheinLoginStatusLabel(item?: SheinLoginAccountStatus | null) {
  if (!item) {
    return { label: "未登录", variant: "neutral" as const };
  }
  if (item.login_in_progress) {
    return { label: "登录中", variant: "warning" as const };
  }
  if (item.waiting_for_verify_code) {
    return { label: "待验证码", variant: "warning" as const };
  }
  if (item.has_cookie || (item.cookie_ttl ?? 0) > 0) {
    return { label: "已登录", variant: "success" as const };
  }
  if (item.last_failure) {
    return { label: "登录异常", variant: "danger" as const };
  }
  return { label: "未登录", variant: "neutral" as const };
}

export function buildSheinLoginStatusMap(items: SheinLoginAccountStatus[] | undefined) {
  const map = new Map<number, SheinLoginAccountStatus>();
  for (const item of items ?? []) {
    map.set(item.account.store_id, item);
  }
  return map;
}

export function StoreLoginStatusBadge({
  store,
  status,
  failed,
}: {
  store: Pick<ListingStore, "platform">;
  status?: SheinLoginAccountStatus | null;
  failed?: boolean;
}) {
  if (store.platform !== "SHEIN") {
    return <span className="text-zinc-400">-</span>;
  }
  if (failed) {
    return <Badge variant="danger">检测失败</Badge>;
  }
  const presentation = sheinLoginStatusLabel(status);
  return <Badge variant={presentation.variant}>{presentation.label}</Badge>;
}
