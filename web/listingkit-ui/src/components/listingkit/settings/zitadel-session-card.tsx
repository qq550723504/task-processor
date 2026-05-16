"use client";

import { RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";

type ZitadelIdentity = {
  tenantId?: string | number;
  userId?: string | number;
  userType?: string | number;
  roles?: string[];
};

type SessionState =
  | { status: "loading"; identity?: never; error?: never }
  | { status: "ready"; identity: ZitadelIdentity; error?: never }
  | { status: "error"; identity?: never; error: string };

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
  const [session, setSession] = useState<SessionState>({ status: "loading" });

  useEffect(() => {
    let cancelled = false;

    async function loadSession() {
      try {
        const response = await fetch("/api/zitadel-auth/session", {
          method: "GET",
          headers: { Accept: "application/json" },
          cache: "no-store",
        });
        const payload = (await response.json()) as {
          identity?: ZitadelIdentity;
          message?: string;
          error?: string;
        };
        if (cancelled) {
          return;
        }
        if (!response.ok || !payload.identity) {
          setSession({
            status: "error",
            error: payload.message || payload.error || "ZITADEL 会话不可用",
          });
          return;
        }
        setSession({ status: "ready", identity: payload.identity });
      } catch (error) {
        if (!cancelled) {
          setSession({
            status: "error",
            error: error instanceof Error ? error.message : "ZITADEL 会话读取失败",
          });
        }
      }
    }

    void loadSession();

    return () => {
      cancelled = true;
    };
  }, []);

  const roles = useMemo(
    () => (session.status === "ready" ? session.identity.roles ?? [] : []),
    [session],
  );
  const hasPlatformAccess = useMemo(
    () => roles.some((role) => PLATFORM_ADMIN_ROLES.includes(role)),
    [roles],
  );

  return (
    <section className="rounded-[1.5rem] border border-white/70 bg-white/85 p-5 shadow-[0_18px_70px_rgba(39,39,42,0.08)]">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-teal-700">
            ZITADEL
          </p>
          <h2 className="mt-2 text-xl font-semibold text-zinc-950">
            当前登录态
          </h2>
        </div>
        {session.status === "loading" ? (
          <Badge className="gap-2" variant="neutral">
            <RefreshCw className="size-3 animate-spin" />
            读取中
          </Badge>
        ) : (
          <Badge
            variant={
              session.status === "ready" && hasPlatformAccess
                ? "success"
                : "warning"
            }
          >
            {session.status === "ready" && hasPlatformAccess
              ? "具备平台管理权限"
              : "缺少平台管理权限"}
          </Badge>
        )}
      </div>

      {session.status === "error" ? (
        <Alert className="mt-4" variant="destructive">
          <AlertDescription>{session.error}</AlertDescription>
        </Alert>
      ) : null}

      {session.status === "ready" ? (
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2">
            <p className="text-xs font-medium text-zinc-500">租户 ID</p>
            <p className="mt-1 break-all font-mono text-sm text-zinc-950">
              {stringify(session.identity.tenantId)}
            </p>
          </div>
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2">
            <p className="text-xs font-medium text-zinc-500">用户 ID</p>
            <p className="mt-1 break-all font-mono text-sm text-zinc-950">
              {stringify(session.identity.userId)}
            </p>
          </div>
          <div className="rounded-lg border border-zinc-100 bg-zinc-50 px-3 py-2">
            <p className="text-xs font-medium text-zinc-500">身份类型</p>
            <p className="mt-1 break-all font-mono text-sm text-zinc-950">
              {stringify(session.identity.userType)}
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
                或 `admin` 之一，才能访问平台订阅和套餐管理。
              </p>
            ) : null}
          </div>
        </div>
      ) : null}
    </section>
  );
}
