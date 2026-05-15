import {
  buildListingKitProxyUrl,
  getListingKitUpstreamBase,
} from "@/app/api/listing-kits/proxy-url";
import {
  readLocalJsonFileSync,
  writeLocalJsonFileSync,
} from "@/lib/server/local-json-file";
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
const DEFAULT_RUNNING_JOB_TIMEOUT_MS = 60 * 60 * 1000;
const ASYNC_JOBS_FILE_NAME = "listingkit-async-jobs.json";
// Jobs are executed by the process that starts them. Status is mirrored to local
// JSON storage so refreshes and sibling instances with a shared volume can poll.
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
  persistJobs();

  void runListingKitAsyncJob(job, input.body);
  return snapshotJob(job);
}

export function getListingKitAsyncJob(id: string) {
  loadJobsFromStorage();
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
    if (!isCurrentRunningJob(job)) {
      logLateAsyncJobResponseIgnored(job, startedAt);
      return;
    }
    job.upstreamStatus = upstream.status;
    persistJobs();
    const text = await upstream.text();
    if (!isCurrentRunningJob(job)) {
      logLateAsyncJobResponseIgnored(job, startedAt, upstream.status);
      return;
    }
    const payload = parseJsonPayload(text);
    job.finishedAt = Date.now();
    if (!upstream.ok) {
      job.status = "failed";
      job.error = extractPayloadMessage(payload) ?? `ListingKit API request failed: ${upstream.status}`;
      job.result = payload;
      persistJobs();
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
    persistJobs();
    logRequestInfo("listingkit async job succeeded", {
      requestId: job.requestId,
      jobId: job.id,
      method: job.method,
      path: job.path,
      status: upstream.status,
      durationMs: Date.now() - startedAt,
    });
  } catch (error) {
    if (!isCurrentRunningJob(job)) {
      logLateAsyncJobResponseIgnored(job, startedAt);
      return;
    }
    job.status = "failed";
    job.finishedAt = Date.now();
    job.error =
      error instanceof Error ? error.message : "ListingKit async job failed";
    persistJobs();
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
  loadJobsFromStorage();
  const now = Date.now();
  let changed = false;
  for (const [id, job] of jobs) {
    if (job.status === "running" && now - job.startedAt > getRunningJobTimeoutMs()) {
      job.status = "failed";
      job.finishedAt = now;
      job.error = "ListingKit async job timed out before completion";
      changed = true;
      continue;
    }
    if (jobs.size <= MAX_JOBS && now - job.startedAt <= JOB_TTL_MS) {
      continue;
    }
    jobs.delete(id);
    changed = true;
  }
  if (changed) {
    persistJobs();
  }
}

function loadJobsFromStorage() {
  const parsed = readLocalJsonFileSync<{ jobs?: AsyncJob[] }>(
    ASYNC_JOBS_FILE_NAME,
    {},
  );
  if (!Array.isArray(parsed.jobs)) {
    return;
  }
  for (const job of parsed.jobs) {
    if (isPersistedJob(job)) {
      jobs.set(job.id, job);
    }
  }
}

function persistJobs() {
  writeLocalJsonFileSync(ASYNC_JOBS_FILE_NAME, { jobs: [...jobs.values()] });
}

function getRunningJobTimeoutMs() {
  const configured = Number(process.env.LISTINGKIT_UI_ASYNC_JOB_TIMEOUT_MS);
  return Number.isFinite(configured) && configured > 0
    ? configured
    : DEFAULT_RUNNING_JOB_TIMEOUT_MS;
}

function isPersistedJob(job: unknown): job is AsyncJob {
  if (!job || typeof job !== "object") {
    return false;
  }
  const record = job as Record<string, unknown>;
  return (
    typeof record.id === "string" &&
    typeof record.path === "string" &&
    record.method === "POST" &&
    (record.status === "running" ||
      record.status === "succeeded" ||
      record.status === "failed") &&
    typeof record.startedAt === "number" &&
    typeof record.requestId === "string"
  );
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

function isCurrentRunningJob(job: AsyncJob) {
  return jobs.get(job.id) === job && job.status === "running";
}

function logLateAsyncJobResponseIgnored(
  job: AsyncJob,
  startedAt: number,
  status?: number,
) {
  logRequestWarn("listingkit async job late response ignored", {
    requestId: job.requestId,
    jobId: job.id,
    method: job.method,
    path: job.path,
    status,
    durationMs: Date.now() - startedAt,
  });
}
