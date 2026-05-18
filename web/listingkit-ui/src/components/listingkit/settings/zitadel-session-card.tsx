"use client";

import { RefreshCw } from "lucide-react";
import { useMemo } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { ListingKitSettingsSection } from "@/components/listingkit/settings/listingkit-settings-section";
import { useZitadelSession } from "@/lib/query/use-zitadel-session";

const PLATFORM_ADMIN_ROLES = ["platform_admin", "listingkit_admin", "admin"];

function stringify(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  return "未返回";
}

export function ZitadelSessionCard() {
  const session = useZitadelSession();

  const roles = useMemo(
    () => (session.data?.roles ?? []),
    [session],
  );
  const hasPlatformAccess = useMemo(
    () => roles.some((role) => PLATFORM_ADMIN_ROLES.includes(role)),
    [roles],
  );

  return (
    <ListingKitSettingsSection
      id="session"
      eyebrow="ZITADEL"
      title="账户信息"
      description="查看当前登录用户在 ZITADEL 中的登录名、租户、用户标识和角色信息。后续设置与权限判断都基于这组身份。"
      actions={
        session.isPending ? (
          <Badge className="gap-2" variant="neutral">
            <RefreshCw className="size-3 animate-spin" />
            读取中
          </Badge>
        ) : (
          <Badge
            variant={
              session.data && hasPlatformAccess
                ? "success"
                : "warning"
            }
          >
            {session.data && hasPlatformAccess
              ? "具备平台管理权限"
              : "缺少平台管理权限"}
          </Badge>
        )
      }
    >
      {session.isError ? (
        <Alert className="mt-4" variant="destructive">
          <AlertDescription>{session.error.message}</AlertDescription>
        </Alert>
      ) : null}

      {session.data ? (
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2">
            <p className="text-xs font-medium text-zinc-500">登录名</p>
            <p className="mt-1 break-all font-mono text-sm text-zinc-950">
              {stringify(session.data.username)}
            </p>
          </div>
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2">
            <p className="text-xs font-medium text-zinc-500">用户 ID</p>
            <p className="mt-1 break-all font-mono text-sm text-zinc-950">
              {stringify(session.data.userId)}
            </p>
          </div>
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2">
            <p className="text-xs font-medium text-zinc-500">租户 ID</p>
            <p className="mt-1 break-all font-mono text-sm text-zinc-950">
              {stringify(session.data.tenantId)}
            </p>
          </div>
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2 md:col-span-3">
            <p className="text-xs font-medium text-zinc-500">身份类型</p>
            <p className="mt-1 break-all font-mono text-sm text-zinc-950">
              {stringify(session.data.userType)}
            </p>
          </div>
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2 md:col-span-3">
            <p className="text-xs font-medium text-zinc-500">角色</p>
            <div className="mt-2 flex flex-wrap gap-2">
              {roles.length > 0 ? (
                roles.map((role) => (
                  <span
                    key={role}
                    className="rounded-full bg-white px-2.5 py-1 font-mono text-xs text-zinc-800 ring-1 ring-zinc-200"
                  >
                    {role}
                  </span>
                ))
              ) : (
                <span className="text-sm text-zinc-500">未返回角色</span>
              )}
            </div>
            {!hasPlatformAccess ? (
              <p className="mt-3 text-sm text-zinc-600">
                需要在 ZITADEL 给当前用户配置 `platform_admin`、`listingkit_admin`
                或 `admin` 之一，才能访问租户订阅管理和套餐管理。
              </p>
            ) : null}
          </div>
        </div>
      ) : null}
    </ListingKitSettingsSection>
  );
}
