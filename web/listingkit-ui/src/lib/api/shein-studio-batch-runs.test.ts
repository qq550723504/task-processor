import { afterEach, describe, expect, it, vi } from "vitest";

import {
  cancelSheinStudioBatchRun,
  getSheinStudioBatchRun,
  listSheinStudioBatchRunItems,
  parseSheinStudioBatchRunStartResponse,
  startSheinStudioBatchRun,
} from "@/lib/api/shein-studio-batch-runs";

describe("parseSheinStudioBatchRunStartResponse", () => {
  it("maps backend batch run payloads into frontend run types", () => {
    expect(
      parseSheinStudioBatchRunStartResponse({
        run: {
          id: "run-1",
          mode: "generate",
          failure_policy: "continue_on_error",
          status: "running",
          current_batch_id: "batch-2",
          current_index: 2,
          total_batches: 3,
          completed_batches: 1,
          succeeded_batches: 1,
          failed_batches: 0,
          last_error: "",
          cancel_requested: false,
          started_at: "2026-05-31T12:00:00Z",
          created_at: "2026-05-31T11:59:00Z",
          updated_at: "2026-05-31T12:00:01Z",
        },
        items: [
          {
            id: "run-1:1",
            run_id: "run-1",
            batch_id: "batch-1",
            position: 1,
            status: "succeeded",
            session_id: "session-1",
            async_job_id: "job-1",
            created_at: "2026-05-31T11:59:00Z",
            updated_at: "2026-05-31T12:00:01Z",
          },
        ],
      }),
    ).toEqual({
      run: {
        id: "run-1",
        mode: "generate",
        failurePolicy: "continue_on_error",
        status: "running",
        currentBatchId: "batch-2",
        currentIndex: 2,
        totalBatches: 3,
        completedBatches: 1,
        succeededBatches: 1,
        failedBatches: 0,
        lastError: "",
        cancelRequested: false,
        startedAt: "2026-05-31T12:00:00Z",
        finishedAt: undefined,
        createdAt: "2026-05-31T11:59:00Z",
        updatedAt: "2026-05-31T12:00:01Z",
      },
      items: [
        {
          id: "run-1:1",
          runId: "run-1",
          batchId: "batch-1",
          position: 1,
          status: "succeeded",
          sessionId: "session-1",
          asyncJobId: "job-1",
          errorMessage: undefined,
          startedAt: undefined,
          finishedAt: undefined,
          createdAt: "2026-05-31T11:59:00Z",
          updatedAt: "2026-05-31T12:00:01Z",
        },
      ],
    });
  });

  it("rejects invalid batch run payloads", () => {
    expect(() =>
      parseSheinStudioBatchRunStartResponse({
        run: {
          id: 123,
        },
      }),
    ).toThrow("ListingKit API returned an unexpected studio batch run response");
  });
});

describe("shein studio batch run API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("starts a studio batch run with ordered batch ids", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          run: {
            id: "run-1",
            mode: "generate",
            failure_policy: "continue_on_error",
            status: "pending",
            current_index: 0,
            total_batches: 2,
            completed_batches: 0,
            succeeded_batches: 0,
            failed_batches: 0,
            cancel_requested: false,
            created_at: "2026-05-31T12:00:00Z",
            updated_at: "2026-05-31T12:00:00Z",
          },
          items: [
            {
              id: "run-1:1",
              run_id: "run-1",
              batch_id: "batch-1",
              position: 1,
              status: "pending",
              created_at: "2026-05-31T12:00:00Z",
              updated_at: "2026-05-31T12:00:00Z",
            },
          ],
        }),
        {
          status: 202,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(startSheinStudioBatchRun(["batch-1", "batch-2"])).resolves.toMatchObject(
      {
        run: {
          id: "run-1",
          status: "pending",
          totalBatches: 2,
        },
        items: [{ batchId: "batch-1", position: 1 }],
      },
    );

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/listing-kits/studio/batch-runs");
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ batch_ids: ["batch-1", "batch-2"] }),
    });
  });

  it("loads a studio batch run detail", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          run: {
            id: "run-9",
            mode: "generate",
            failure_policy: "continue_on_error",
            status: "running",
            current_batch_id: "batch-2",
            current_index: 2,
            total_batches: 3,
            completed_batches: 1,
            succeeded_batches: 1,
            failed_batches: 0,
            cancel_requested: false,
            created_at: "2026-05-31T12:00:00Z",
            updated_at: "2026-05-31T12:00:01Z",
          },
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getSheinStudioBatchRun("run-9")).resolves.toMatchObject({
      id: "run-9",
      currentBatchId: "batch-2",
      status: "running",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/listing-kits/studio/batch-runs/run-9");
  });

  it("lists studio batch run items", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          items: [
            {
              id: "run-5:1",
              run_id: "run-5",
              batch_id: "batch-1",
              position: 1,
              status: "succeeded",
              created_at: "2026-05-31T12:00:00Z",
              updated_at: "2026-05-31T12:00:01Z",
            },
            {
              id: "run-5:2",
              run_id: "run-5",
              batch_id: "batch-2",
              position: 2,
              status: "failed",
              error_message: "upstream failed",
              created_at: "2026-05-31T12:00:00Z",
              updated_at: "2026-05-31T12:00:02Z",
            },
          ],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listSheinStudioBatchRunItems("run-5")).resolves.toEqual([
      expect.objectContaining({
        id: "run-5:1",
        batchId: "batch-1",
        status: "succeeded",
      }),
      expect.objectContaining({
        id: "run-5:2",
        batchId: "batch-2",
        errorMessage: "upstream failed",
      }),
    ]);

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/listing-kits/studio/batch-runs/run-5/items");
  });

  it("cancels a studio batch run", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify({ ok: true }), {
        status: 202,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(cancelSheinStudioBatchRun("run-cancel")).resolves.toBeUndefined();

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/batch-runs/run-cancel/cancel",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});
