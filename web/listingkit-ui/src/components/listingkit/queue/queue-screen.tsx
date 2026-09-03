"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

export function QueueScreen({ taskId }: { taskId: string }) {
  const router = useRouter();
  useSearchParams();
  useEffect(() => {
    router.replace(`/listing-kits/${taskId}/workspace`);
  }, [router, taskId]);
  return null;
}
