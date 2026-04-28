import { NextRequest, NextResponse } from "next/server";

import {
  getListingKitAsyncJob,
  startListingKitAsyncJob,
} from "@/lib/server/listingkit-async-jobs";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as {
      path?: string;
      body?: unknown;
    };
    if (!body.path) {
      return NextResponse.json(
        { error: "invalid_request", message: "path is required" },
        { status: 400 },
      );
    }
    return NextResponse.json(
      startListingKitAsyncJob({
        path: body.path,
        body: body.body,
      }),
      { status: 202 },
    );
  } catch (error) {
    return NextResponse.json(
      {
        error: "listingkit_async_job_start_failed",
        message: error instanceof Error ? error.message : "Failed to start async job",
      },
      { status: 400 },
    );
  }
}

export async function GET(request: NextRequest) {
  const id = request.nextUrl.searchParams.get("id") ?? "";
  if (!id) {
    return NextResponse.json(
      { error: "invalid_request", message: "id is required" },
      { status: 400 },
    );
  }
  const job = getListingKitAsyncJob(id);
  if (!job) {
    return NextResponse.json(
      { error: "not_found", message: "Async job was not found" },
      { status: 404 },
    );
  }
  return NextResponse.json(job);
}
