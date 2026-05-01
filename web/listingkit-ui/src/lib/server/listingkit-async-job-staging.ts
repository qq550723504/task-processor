import { startListingKitAsyncJob } from "@/lib/server/listingkit-async-jobs";

type ListingKitAsyncJobStage = {
  id: string;
  path: string;
  chunkCount: number;
  chunks: string[];
  createdAt: number;
  updatedAt: number;
};

const MAX_STAGES = 100;
const STAGE_TTL_MS = 30 * 60 * 1000;
const stages = new Map<string, ListingKitAsyncJobStage>();

export function createListingKitAsyncJobStage(input: {
  path: string;
  chunkCount: number;
}) {
  cleanupStages();
  if (!input.path.trim()) {
    throw new Error("path is required");
  }
  if (!Number.isInteger(input.chunkCount) || input.chunkCount <= 0) {
    throw new Error("chunk_count must be a positive integer");
  }

  const id = crypto.randomUUID();
  stages.set(id, {
    id,
    path: input.path,
    chunkCount: input.chunkCount,
    chunks: new Array<string>(input.chunkCount),
    createdAt: Date.now(),
    updatedAt: Date.now(),
  });
  return { stage_id: id };
}

export function appendListingKitAsyncJobStageChunk(input: {
  stageId: string;
  chunkIndex: number;
  chunk: string;
}) {
  cleanupStages();
  const stage = stages.get(input.stageId);
  if (!stage) {
    throw new Error("async job stage was not found");
  }
  if (
    !Number.isInteger(input.chunkIndex) ||
    input.chunkIndex < 0 ||
    input.chunkIndex >= stage.chunkCount
  ) {
    throw new Error("chunk_index is out of range");
  }

  stage.chunks[input.chunkIndex] = input.chunk;
  stage.updatedAt = Date.now();
  return { ok: true };
}

export function startListingKitAsyncJobFromStage(stageId: string) {
  cleanupStages();
  const stage = stages.get(stageId);
  if (!stage) {
    throw new Error("async job stage was not found");
  }

  const missingChunkIndex = stage.chunks.findIndex(
    (chunk) => typeof chunk !== "string",
  );
  if (missingChunkIndex >= 0) {
    throw new Error(`async job stage chunk ${missingChunkIndex} is missing`);
  }

  const payloadText = stage.chunks.join("");
  stages.delete(stageId);

  let body: unknown;
  try {
    body = payloadText ? (JSON.parse(payloadText) as unknown) : {};
  } catch (error) {
    throw new Error(
      error instanceof Error
        ? `async job stage payload is invalid JSON: ${error.message}`
        : "async job stage payload is invalid JSON",
    );
  }

  return startListingKitAsyncJob({
    path: stage.path,
    body,
  });
}

function cleanupStages() {
  const now = Date.now();
  for (const [id, stage] of stages) {
    if (stages.size <= MAX_STAGES && now - stage.updatedAt <= STAGE_TTL_MS) {
      continue;
    }
    stages.delete(id);
  }
}
