import {
  buildListingKitProxyUrl,
  getListingKitUpstreamBase,
} from "@/app/api/listing-kits/proxy-url";
import {
  logRequestInfo,
  logRequestWarn,
  newRequestLogId,
} from "@/lib/server/request-log";

type AsyncJobStatus = "running" | "succeeded" | "failed";

type AsyncJob = {
  id: string;
  path: string;
  method: "POST";
  status: AsyncJobStatus;
  startedAt: number;
  finishedAt?: number;
  result?: unknown;
  error?: string;
  upstreamStatus?: number;
  requestId: string;
};

const MAX_JOBS = 100;
const JOB_TTL_MS = 60 * 60 * 1000;
const jobs = new Map<string, AsyncJob>();

export function startListingKitAsyncJob(input: {
  path: string;
  body: unknown;
}) {
  cleanupJobs();
  const path = normalizeAsyncPath(input.path);
  assertAsyncPathAllowed(path);

  const job: AsyncJob = {
    id: newRequestLogId(),
    path,
    method: "POST",
    status: "running",
    startedAt: Date.now(),
    requestId: newRequestLogId(),
  };
  jobs.set(job.id, job);

  void runListingKitAsyncJob(job, input.body);
  return snapshotJob(job);
}

export function getListingKitAsyncJob(id: string) {
  cleanupJobs();
  const job = jobs.get(id);
  return job ? snapshotJob(job) : null;
}

async function runListingKitAsyncJob(job: AsyncJob, body: unknown) {
  const url = buildListingKitProxyUrl(
    getListingKitUpstreamBase(),
    job.path.split("/").filter(Boolean),
    "",
  );
  const startedAt = Date.now();
  logRequestInfo("listingkit async job started", {
    requestId: job.requestId,
    jobId: job.id,
    method: job.method,
    path: job.path,
  });

  try {
    const upstream = await fetch(url, {
      method: job.method,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body ?? {}),
      cache: "no-store",
    });
    job.upstreamStatus = upstream.status;
    const text = await upstream.text();
    const payload = parseJsonPayload(text);
    job.finishedAt = Date.now();
    if (!upstream.ok) {
      job.status = "failed";
      job.error = extractPayloadMessage(payload) ?? `ListingKit API request failed: ${upstream.status}`;
      job.result = payload;
      logRequestWarn("listingkit async job failed", {
        requestId: job.requestId,
        jobId: job.id,
        method: job.method,
        path: job.path,
        status: upstream.status,
        durationMs: Date.now() - startedAt,
        error: job.error,
      });
      return;
    }

    job.status = "succeeded";
    job.result = payload;
    logRequestInfo("listingkit async job succeeded", {
      requestId: job.requestId,
      jobId: job.id,
      method: job.method,
      path: job.path,
      status: upstream.status,
      durationMs: Date.now() - startedAt,
    });
  } catch (error) {
    job.status = "failed";
    job.finishedAt = Date.now();
    job.error =
      error instanceof Error ? error.message : "ListingKit async job failed";
    logRequestWarn("listingkit async job crashed", {
      requestId: job.requestId,
      jobId: job.id,
      method: job.method,
      path: job.path,
      status: 500,
      durationMs: Date.now() - startedAt,
      error: job.error,
    });
  }
}

function snapshotJob(job: AsyncJob) {
  return {
    job_id: job.id,
    path: job.path,
    status: job.status,
    started_at: new Date(job.startedAt).toISOString(),
    finished_at: job.finishedAt ? new Date(job.finishedAt).toISOString() : undefined,
    result: job.result,
    error: job.error,
    upstream_status: job.upstreamStatus,
    request_id: job.requestId,
  };
}

function cleanupJobs() {
  const now = Date.now();
  for (const [id, job] of jobs) {
    if (jobs.size <= MAX_JOBS && now - job.startedAt <= JOB_TTL_MS) {
      continue;
    }
    jobs.delete(id);
  }
}

function normalizeAsyncPath(path: string) {
  const trimmed = path.trim();
  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function assertAsyncPathAllowed(path: string) {
  if (path === "/studio/designs" || path === "/studio/product-images") {
    return;
  }
  throw new Error(`async ListingKit path is not allowed: ${path}`);
}

function parseJsonPayload(text: string) {
  if (!text) {
    return undefined;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return { raw: text };
  }
}

function extractPayloadMessage(payload: unknown) {
  if (!payload || typeof payload !== "object") {
    return undefined;
  }
  const record = payload as Record<string, unknown>;
  return typeof record.message === "string" ? record.message : undefined;
}
