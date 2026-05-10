"use client";

import { useEffect } from "react";

import {
  isYudaoAuthMessage,
  rememberYudaoAuth,
} from "@/lib/api/yudao-auth";

function allowedOrigins() {
  return (process.env.NEXT_PUBLIC_YUDAO_PARENT_ORIGINS ?? "")
    .split(",")
    .map((origin) => origin.trim())
    .filter(Boolean);
}

export function YudaoAuthBridge() {
  useEffect(() => {
    const origins = allowedOrigins();

    function handleMessage(event: MessageEvent) {
      if (origins.length > 0 && !origins.includes(event.origin)) {
        return;
      }
      if (!isYudaoAuthMessage(event.data)) {
        return;
      }
      rememberYudaoAuth(event.data.payload);
    }

    window.addEventListener("message", handleMessage);
    window.parent?.postMessage({ type: "listingkit:ready" }, "*");

    return () => {
      window.removeEventListener("message", handleMessage);
    };
  }, []);

  return null;
}
