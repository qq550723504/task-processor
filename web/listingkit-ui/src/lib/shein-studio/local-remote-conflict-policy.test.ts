import { describe, expect, it } from "vitest";

import {
  classifyLocalRemoteDraftState,
  pickLocalArrayValue,
  pickLocalStringValue,
  shouldUseLocalDraftOverRemote,
} from "@/lib/shein-studio/local-remote-conflict-policy";

describe("local/remote draft conflict policy", () => {
  it.each([
    {
      expected: "local_only",
      localUpdatedAt: "2026-06-22T00:00:00.000Z",
      remoteUpdatedAt: "",
    },
    {
      expected: "remote_only",
      localUpdatedAt: "",
      remoteUpdatedAt: "2026-06-22T00:00:00.000Z",
    },
    {
      expected: "local_newer",
      localUpdatedAt: "2026-06-22T00:00:01.000Z",
      remoteUpdatedAt: "2026-06-22T00:00:00.000Z",
    },
    {
      expected: "remote_newer",
      localUpdatedAt: "2026-06-22T00:00:00.000Z",
      remoteUpdatedAt: "2026-06-22T00:00:01.000Z",
    },
    {
      expected: "saving",
      localUpdatedAt: "2026-06-22T00:00:00.000Z",
      persistenceState: "saving",
      remoteUpdatedAt: "2026-06-22T00:00:01.000Z",
    },
    {
      expected: "save_failed",
      localUpdatedAt: "2026-06-22T00:00:00.000Z",
      persistenceState: "save_failed",
      remoteUpdatedAt: "2026-06-22T00:00:01.000Z",
    },
    {
      expected: "synchronized",
      localUpdatedAt: "2026-06-22T00:00:00.000Z",
      remoteUpdatedAt: "2026-06-22T00:00:00.000Z",
    },
    {
      expected: "conflict",
      localUpdatedAt: "not-a-date",
      remoteUpdatedAt: "2026-06-22T00:00:00.000Z",
    },
  ] as const)(
    "classifies $expected",
    ({ expected, localUpdatedAt, persistenceState, remoteUpdatedAt }) => {
      expect(
        classifyLocalRemoteDraftState({
          localUpdatedAt,
          persistenceState,
          remoteUpdatedAt,
        }),
      ).toBe(expected);
    },
  );

  it("uses local draft only when the policy can prove it is newer or local-only", () => {
    expect(
      shouldUseLocalDraftOverRemote({
        localUpdatedAt: "2026-06-22T00:00:01.000Z",
        remoteUpdatedAt: "2026-06-22T00:00:00.000Z",
      }),
    ).toBe(true);
    expect(
      shouldUseLocalDraftOverRemote({
        localUpdatedAt: "2026-06-22T00:00:00.000Z",
        remoteUpdatedAt: "2026-06-22T00:00:01.000Z",
      }),
    ).toBe(false);
    expect(
      shouldUseLocalDraftOverRemote({
        localUpdatedAt: "2026-06-22T00:00:00.000Z",
        persistenceState: "saving",
        remoteUpdatedAt: "2026-06-22T00:00:01.000Z",
      }),
    ).toBe(true);
    expect(
      shouldUseLocalDraftOverRemote({
        localUpdatedAt: "2026-06-22T00:00:00.000Z",
        persistenceState: "save_failed",
        remoteUpdatedAt: "2026-06-22T00:00:01.000Z",
      }),
    ).toBe(true);
  });

  it("keeps non-empty local field values and falls back to remote empties", () => {
    expect(pickLocalStringValue(" local prompt ", "remote prompt")).toBe(
      " local prompt ",
    );
    expect(pickLocalStringValue("   ", "remote prompt")).toBe("remote prompt");
    expect(pickLocalArrayValue(["local"], ["remote"])).toEqual(["local"]);
    expect(pickLocalArrayValue([], ["remote"])).toEqual(["remote"]);
  });
});
