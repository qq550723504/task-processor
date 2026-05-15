"use client";

import { ReactNode, useEffect } from "react";

export function ZitadelAuthGate({ children }: { children: ReactNode }) {
  useEffect(() => {
    let cancelled = false;

    async function verifySession() {
      const response = await fetch("/api/zitadel-auth/session", {
        method: "GET",
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      if (!response.ok && !cancelled) {
        const returnTo = `${window.location.pathname}${window.location.search}`;
        window.location.assign(
          `/api/zitadel-auth/login?returnTo=${encodeURIComponent(returnTo)}`,
        );
      }
    }

    void verifySession();

    return () => {
      cancelled = true;
    };
  }, []);

  return <>{children}</>;
}
