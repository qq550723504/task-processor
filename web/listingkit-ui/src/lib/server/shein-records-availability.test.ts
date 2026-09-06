// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { isSheinRecordsAvailable } from "./shein-records-availability";

afterEach(() => vi.unstubAllEnvs());
it.each([
  [undefined, false],
  ["http://127.0.0.1:8080", true],
  ["https://example.com/", true],
  ["https://example.com/api", false],
  ["https://user:pass@example.com", false],
])("exports only configured availability for %s", (origin, expected) => {
  vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", origin);
  expect(isSheinRecordsAvailable()).toBe(expected);
});
