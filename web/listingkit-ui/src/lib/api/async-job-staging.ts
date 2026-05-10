import { applyYudaoAuthHeaders } from "@/lib/api/yudao-auth";

const MAX_INLINE_ASYNC_JOB_BYTES = 1024;
const MAX_STAGED_CHUNK_BYTES = 700;

type AsyncJobStartRequest = {
  path: string;
  body: unknown;
};

type AsyncJobStageCreateResponse = {
  stage_id: string;
};

export async function stageAsyncJobRequestIfNeeded(
  input: AsyncJobStartRequest,
) {
  const payloadText = JSON.stringify(input.body ?? {});
  if (utf8ByteLength(payloadText) <= MAX_INLINE_ASYNC_JOB_BYTES) {
    return {
      staged: false as const,
      bodyText: payloadText,
    };
  }

  const chunks = chunkStringByUtf8Bytes(payloadText, MAX_STAGED_CHUNK_BYTES);
  const created = await fetch("/api/listing-kits/async-jobs/staged", {
    method: "POST",
    headers: applyYudaoAuthHeaders(new Headers({
      Accept: "application/json",
      "Content-Type": "application/json",
    })),
    body: JSON.stringify({
      path: input.path,
      chunk_count: chunks.length,
    }),
  });
  const createdPayload =
    (await created.json().catch(() => undefined)) as
      | (AsyncJobStageCreateResponse & { message?: string })
      | undefined;
  if (!created.ok || !createdPayload?.stage_id) {
    throw new Error(
      createdPayload?.message ??
        `ListingKit async job stage create failed: ${created.status}`,
    );
  }

  for (const [index, chunk] of chunks.entries()) {
    const response = await fetch("/api/listing-kits/async-jobs/staged", {
      method: "PUT",
      headers: applyYudaoAuthHeaders(new Headers({
        Accept: "application/json",
        "Content-Type": "application/json",
      })),
      body: JSON.stringify({
        stage_id: createdPayload.stage_id,
        chunk_index: index,
        chunk,
      }),
    });
    if (!response.ok) {
      const payload =
        (await response.json().catch(() => undefined)) as
          | { message?: string }
          | undefined;
      throw new Error(
        payload?.message ??
          `ListingKit async job stage upload failed: ${response.status}`,
      );
    }
  }

  return {
    staged: true as const,
    stageId: createdPayload.stage_id,
  };
}

function chunkStringByUtf8Bytes(text: string, maxBytes: number) {
  const chunks: string[] = [];
  const encoder = new TextEncoder();
  let current = "";
  let currentBytes = 0;

  for (const char of text) {
    const charBytes = encoder.encode(char).length;
    if (current && currentBytes + charBytes > maxBytes) {
      chunks.push(current);
      current = char;
      currentBytes = charBytes;
      continue;
    }
    current += char;
    currentBytes += charBytes;
  }

  if (current) {
    chunks.push(current);
  }

  return chunks.length > 0 ? chunks : [""];
}

function utf8ByteLength(text: string) {
  return new TextEncoder().encode(text).length;
}
