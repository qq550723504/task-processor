"use client";

import { ReactNode, useEffect, useState } from "react";

import {
  applyYudaoAuthHeaders,
  clearYudaoAuth,
  hasRequiredYudaoAuth,
  readYudaoAuth,
  YUDAO_AUTH_CHANGED_EVENT,
} from "@/lib/api/yudao-auth";

type AuthState = "authorized" | "checking" | "unauthorized";

export function YudaoAuthGate({
  authWaitMs = 1200,
  children,
}: {
  authWaitMs?: number;
  children: ReactNode;
}) {
  const [authState, setAuthState] = useState<AuthState>("checking");

  useEffect(() => {
    let cancelled = false;
    let waitTimer: ReturnType<typeof setTimeout> | undefined;

    async function verifyStoredAuth() {
      const auth = readYudaoAuth();
      if (!hasRequiredYudaoAuth(auth)) {
        return false;
      }

      const response = await fetch("/api/yudao-auth/verify", {
        method: "POST",
        headers: applyYudaoAuthHeaders(new Headers({ Accept: "application/json" })),
        cache: "no-store",
      });
      if (!response.ok) {
        clearYudaoAuth();
        return false;
      }
      return true;
    }

    async function finishAuthCheck() {
      try {
        if (await verifyStoredAuth()) {
          if (!cancelled) {
            setAuthState("authorized");
          }
          return;
        }
      } catch {
        clearYudaoAuth();
      }

      if (!cancelled) {
        setAuthState("unauthorized");
      }
    }

    function handleAuthChanged() {
      if (waitTimer) {
        clearTimeout(waitTimer);
        waitTimer = undefined;
      }
      setAuthState("checking");
      void finishAuthCheck();
    }

    window.addEventListener(YUDAO_AUTH_CHANGED_EVENT, handleAuthChanged);
    setAuthState("checking");
    void verifyStoredAuth()
      .then((verified) => {
        if (cancelled) {
          return;
        }
        if (verified) {
          setAuthState("authorized");
          return;
        }
        waitTimer = setTimeout(() => {
          void finishAuthCheck();
        }, authWaitMs);
      })
      .catch(() => {
        clearYudaoAuth();
        if (!cancelled) {
          waitTimer = setTimeout(() => {
            void finishAuthCheck();
          }, authWaitMs);
        }
      });

    return () => {
      cancelled = true;
      if (waitTimer) {
        clearTimeout(waitTimer);
      }
      window.removeEventListener(YUDAO_AUTH_CHANGED_EVENT, handleAuthChanged);
    };
  }, [authWaitMs]);

  if (authState === "authorized") {
    return <>{children}</>;
  }

  if (authState === "checking") {
    return (
      <main className="flex min-h-screen items-center justify-center bg-zinc-100 px-6 text-zinc-900">
        <div className="w-full max-w-sm rounded border border-zinc-200 bg-white p-6 shadow-sm">
          <div className="text-sm font-medium">正在校验登录状态</div>
          <div className="mt-2 text-sm text-zinc-500">请稍候。</div>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-zinc-100 px-6 text-zinc-900">
      <div className="w-full max-w-md rounded border border-zinc-200 bg-white p-6 shadow-sm">
        <div className="text-base font-semibold">未授权访问</div>
        <div className="mt-2 text-sm leading-6 text-zinc-600">
          请先登录 AI Listing 后台，再从左侧菜单进入 ListingKit 工作台。
        </div>
      </div>
    </main>
  );
}
