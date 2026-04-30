import { NextRequest, NextResponse } from "next/server";

import {
  appendListingKitAsyncJobStageChunk,
  createListingKitAsyncJobStage,
  startListingKitAsyncJobFromStage,
} from "@/lib/server/listingkit-async-job-staging";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as {
      path?: string;
      chunk_count?: number;
    };
    return NextResponse.json(
      createListingKitAsyncJobStage({
        path: body.path ?? "",
        chunkCount: body.chunk_count ?? 0,
      }),
      { status: 201 },
    );
  } catch (error) {
    return NextResponse.json(
      {
        error: "listingkit_async_job_stage_create_failed",
        message:
          error instanceof Error ? error.message : "Failed to create async job stage",
      },
      { status: 400 },
    );
  }
}

export async function PUT(request: NextRequest) {
  try {
    const body = (await request.json()) as {
      stage_id?: string;
      chunk_index?: number;
      chunk?: string;
    };
    return NextResponse.json(
      appendListingKitAsyncJobStageChunk({
        stageId: body.stage_id ?? "",
        chunkIndex: body.chunk_index ?? -1,
        chunk: body.chunk ?? "",
      }),
    );
  } catch (error) {
    return NextResponse.json(
      {
        error: "listingkit_async_job_stage_chunk_failed",
        message:
          error instanceof Error ? error.message : "Failed to store async job chunk",
      },
      { status: 400 },
    );
  }
}

export async function PATCH(request: NextRequest) {
  try {
    const body = (await request.json()) as {
      stage_id?: string;
    };
    return NextResponse.json(
      startListingKitAsyncJobFromStage(body.stage_id ?? ""),
      { status: 202 },
    );
  } catch (error) {
    return NextResponse.json(
      {
        error: "listingkit_async_job_stage_start_failed",
        message:
          error instanceof Error ? error.message : "Failed to start async job from stage",
      },
      { status: 400 },
    );
  }
}
