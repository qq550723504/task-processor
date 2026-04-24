"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";

export function TaskLauncher() {
  const router = useRouter();
  const [taskId, setTaskId] = useState("");
  const normalizedTaskId = taskId.trim();

  function openTask(route: "workspace" | "queue", nextTaskId: string) {
    router.push(`/listing-kits/${nextTaskId}/${route}`);
  }

  return (
    <Card className="max-w-2xl p-8">
      <div className="space-y-6">
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            ListingKit UI
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-zinc-950">
            Review workspace and queue console
          </h1>
          <p className="text-sm leading-6 text-zinc-600">
            Enter a ListingKit task id to open the review workspace or generation
            queue. The UI reads the existing listingkit APIs directly. Use
            <span className="mx-1 rounded bg-zinc-100 px-2 py-0.5 font-mono text-xs text-zinc-700">
              demo-task
            </span>
            to load the built-in mock flow when no backend task is available.
          </p>
        </div>

        <label className="block space-y-2">
          <span className="text-sm font-medium text-zinc-700">Task ID</span>
          <input
            className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none ring-0 transition focus:border-zinc-950"
            value={taskId}
            onChange={(event) => setTaskId(event.target.value)}
            placeholder="task_123456"
          />
        </label>

        <div className="flex flex-wrap gap-3">
          <Button tone="secondary" onClick={() => router.push("/listing-kits/new")}>
            Create New Task
          </Button>
          <Button tone="secondary" onClick={() => router.push("/listing-kits/sds")}>
            Open SDS Sync
          </Button>
          <Button tone="secondary" onClick={() => router.push("/listing-kits/shein")}>
            Open SHEIN Studio
          </Button>
          <Button
            disabled={!normalizedTaskId}
            onClick={() => openTask("workspace", normalizedTaskId)}
          >
            Open Workspace
          </Button>
          <Button
            tone="secondary"
            disabled={!normalizedTaskId}
            onClick={() => openTask("queue", normalizedTaskId)}
          >
            Open Queue
          </Button>
          <Button
            tone="ghost"
            disabled={!normalizedTaskId}
            onClick={() => router.push(`/listing-kits/${normalizedTaskId}/status`)}
          >
            Open Status
          </Button>
        </div>

        <div className="rounded-2xl border border-dashed border-zinc-200 bg-zinc-50/80 p-4">
          <div className="space-y-3">
            <div>
              <h2 className="text-sm font-semibold text-zinc-900">
                Local demo mode
              </h2>
              <p className="mt-1 text-sm leading-6 text-zinc-600">
                The Next.js proxy serves mock ListingKit responses when the task id
                is
                <span className="mx-1 font-mono text-xs text-zinc-800">
                  demo-task
                </span>
                or starts with
                <span className="mx-1 font-mono text-xs text-zinc-800">
                  mock-
                </span>
                .
              </p>
            </div>

            <div className="flex flex-wrap gap-3">
              <Button tone="ghost" onClick={() => openTask("workspace", "demo-task")}>
                Open Demo Workspace
              </Button>
              <Button tone="ghost" onClick={() => openTask("queue", "demo-task")}>
                Open Demo Queue
              </Button>
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
}
