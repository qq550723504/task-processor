"use client";

import { useEffect, useRef, useState } from "react";

import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert";
import { Button } from "@/components/ui/button";

type VersionPayload = {
  appVersion: string;
  buildId: string;
};

type AppUpdateBannerProps = {
  onRefresh?: () => void;
  pollIntervalMs?: number;
};

const DEFAULT_POLL_INTERVAL_MS = 60_000;

export function resolveAppUpdatePollIntervalMs(rawValue?: string) {
  const parsed = Number(rawValue?.trim());
  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed;
  }
  return DEFAULT_POLL_INTERVAL_MS;
}

async function fetchVersionPayload(signal?: AbortSignal): Promise<VersionPayload> {
  const response = await fetch("/api/version", {
    cache: "no-store",
    signal,
  });
  if (!response.ok) {
    throw new Error(`version_request_failed:${response.status}`);
  }

  return (await response.json()) as VersionPayload;
}

export function AppUpdateBanner({
  onRefresh = () => window.location.reload(),
  pollIntervalMs = DEFAULT_POLL_INTERVAL_MS,
}: AppUpdateBannerProps) {
  const initialBuildIdRef = useRef<string | null>(null);
  const [updateAvailable, setUpdateAvailable] = useState(false);

  useEffect(() => {
    let cancelled = false;

    const checkVersion = async () => {
      try {
        const version = await fetchVersionPayload();
        if (cancelled) {
          return;
        }
        if (initialBuildIdRef.current === null) {
          initialBuildIdRef.current = version.buildId;
          return;
        }
        if (version.buildId !== initialBuildIdRef.current) {
          setUpdateAvailable(true);
        }
      } catch {
        // Ignore transient polling failures so the banner never interrupts current work.
      }
    };

    void checkVersion();
    const timer = window.setInterval(() => {
      void checkVersion();
    }, pollIntervalMs);

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [pollIntervalMs]);

  if (!updateAvailable) {
    return null;
  }

  return (
    <Alert className="mt-3 border-primary/20 bg-primary/5" variant="default">
      <AlertTitle>发现新版本</AlertTitle>
      <AlertDescription>发现新版本，刷新后即可使用。</AlertDescription>
      <AlertAction>
        <Button onClick={onRefresh} size="sm">
          立即刷新
        </Button>
      </AlertAction>
    </Alert>
  );
}
