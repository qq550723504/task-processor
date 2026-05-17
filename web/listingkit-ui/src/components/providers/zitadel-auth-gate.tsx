"use client";

import { createContext, ReactNode, useContext, useEffect, useState } from "react";

export type ZitadelClientIdentity = {
  tenantId?: string | number;
  userId?: string | number;
  username?: string;
  userType?: string | number;
  roles?: string[];
};

type ZitadelSessionResponse = {
  ok?: boolean;
  identity?: ZitadelClientIdentity;
};

const ZitadelIdentityContext = createContext<ZitadelClientIdentity | null>(null);

export function useZitadelIdentity() {
  return useContext(ZitadelIdentityContext);
}

export function ZitadelAuthGate({ children }: { children: ReactNode }) {
  const [identity, setIdentity] = useState<ZitadelClientIdentity | null>(null);
  const [status, setStatus] = useState<"loading" | "ready">("loading");

  useEffect(() => {
    let cancelled = false;
    if (window.location.pathname.startsWith("/unauthorized")) {
      setStatus("ready");
      return;
    }

    async function verifySession() {
      const response = await fetch("/api/zitadel-auth/session", {
        method: "GET",
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      if (cancelled) {
        return;
      }
      if (response.ok) {
        const payload = (await response.json().catch(() => null)) as
          | ZitadelSessionResponse
          | null;
        setIdentity(payload?.identity ?? null);
        setStatus("ready");
        return;
      }
      if (response.status === 403) {
        window.location.assign("/unauthorized");
        return;
      }
      if (!cancelled) {
        const returnTo = `${window.location.pathname}${window.location.search}`;
        window.location.assign(returnTo || "/");
      }
    }

    void verifySession();

    return () => {
      cancelled = true;
    };
  }, []);

  if (status !== "ready") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-100 text-sm text-zinc-500">
        正在验证登录状态...
      </div>
    );
  }

  return (
    <ZitadelIdentityContext.Provider value={identity}>
      {children}
    </ZitadelIdentityContext.Provider>
  );
}
