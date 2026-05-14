"use client";

import { ReactNode, useEffect } from "react";

import {
  applyYudaoAuthHeaders,
  clearYudaoAuth,
  hasRequiredYudaoAuth,
  readYudaoAuth,
  YUDAO_AUTH_CHANGED_EVENT,
} from "@/lib/api/yudao-auth";

const shouldBypassAuthGate =
  process.env.NODE_ENV !== "production" &&
  process.env.NEXT_PUBLIC_LISTINGKIT_UI_BYPASS_AUTH_GATE === "1";

export function YudaoAuthGate({
  authWaitMs = 1200,
  children,
}: {
  authWaitMs?: number;
  children: ReactNode;
}) {
  if (shouldBypassAuthGate) {
    return <>{children}</>;
  }

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
        await verifyStoredAuth();
      } catch {
        clearYudaoAuth();
      }
    }

    function handleAuthChanged() {
      if (waitTimer) {
        clearTimeout(waitTimer);
        waitTimer = undefined;
      }
      void finishAuthCheck();
    }

    window.addEventListener(YUDAO_AUTH_CHANGED_EVENT, handleAuthChanged);
    void verifyStoredAuth()
      .then((verified) => {
        if (cancelled) {
          return;
        }
        if (!verified) {
          waitTimer = setTimeout(() => {
            void finishAuthCheck();
          }, authWaitMs);
        }
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

  return <>{children}</>;
}
