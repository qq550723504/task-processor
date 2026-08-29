import { buildListingKitProxyUrl } from "@/app/api/listing-kits/proxy-url";

describe("buildListingKitProxyUrl", () => {
  it("joins the upstream base with a nested path and query string", () => {
    const result = buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits",
      ["tasks", "task_123", "generation-queue"],
      "page=1&page_size=20",
      "GET",
    );

    expect(result).toBe(
      "http://localhost:8080/api/v1/listing-kits/tasks/task_123/generation-queue?page=1&page_size=20",
    );
  });

  it("normalizes trailing slashes on the upstream base", () => {
    const result = buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits/",
      ["tasks", "task_123", "preview"],
      "",
      "GET",
    );

    expect(result).toBe(
      "http://localhost:8080/api/v1/listing-kits/tasks/task_123/preview",
    );
  });

  it("maps only the explicit image-agent prefix to the sibling API", () => {
    const result = buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits",
      ["image-agent", "runs", "run-1", "events"],
      "",
      "GET",
    );

    expect(result).toBe(
      "http://localhost:8080/api/v1/image-agent/runs/run-1/events",
    );
  });

  it.each([
    ["GET", ["tasks", "task_123", "image-agent-assets"]],
    ["POST", ["tasks", "task_123", "image-agent-runs"]],
  ])("permits the exact task-scoped image-agent %s route", (method, path) => {
    expect(() => buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits", path, "", method,
    )).not.toThrow();
  });

  it.each([
    ["POST", ["tasks", "task_123", "image-agent-assets"]],
    ["GET", ["tasks", "task_123", "image-agent-runs"]],
    ["GET", ["tasks", "task_123", "image-agent-secrets"]],
  ])("rejects unlisted task-scoped image-agent %s route", (method, path) => {
    expect(() => buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits", path, "", method,
    )).toThrow("image-agent task proxy route is not allowed");
  });

  it("forwards every backend-valid action ID on the resume route", () => {
    const result = buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits",
      ["image-agent", "runs", "run-1", "commands", "approval+1", "resume"],
      "",
      "POST",
    );

    expect(result).toBe(
      "http://localhost:8080/api/v1/image-agent/runs/run-1/commands/approval%2B1/resume",
    );
  });

  it("does not treat similar or traversal-like prefixes as an upstream route", () => {
    expect(
      buildListingKitProxyUrl(
        "http://localhost:8080/api/v1/listing-kits",
        ["image-agent-evil", "runs"],
        "",
        "GET",
      ),
    ).toBe(
      "http://localhost:8080/api/v1/listing-kits/image-agent-evil/runs",
    );
    expect(() =>
      buildListingKitProxyUrl(
        "http://localhost:8080/api/v1/listing-kits",
        ["image-agent", "..", "admin"],
        "",
        "GET",
      ),
    ).toThrow("invalid proxy path segment");
  });

  it.each([
    ["POST", ["image-agent", "runs"]],
    ["GET", ["image-agent", "runs", "run-1"]],
    ["PUT", ["image-agent", "runs", "run-1", "plan"]],
    ["POST", ["image-agent", "runs", "run-1", "slots", "scene-2", "retry"]],
    ["POST", ["image-agent", "runs", "run-1", "slots", "scene-2", "attempts", "1", "recover"]],
    ["POST", ["image-agent", "runs", "run-1", "results", "approve"]],
    ["POST", ["image-agent", "runs", "run-1", "cancel"]],
    ["POST", ["image-agent", "runs", "run-1", "restart"]],
    ["GET", ["image-agent", "runs", "run-1", "events"]],
    ["POST", ["image-agent", "runs", "run-1", "commands", "action-1", "resume"]],
  ])("allows %s only for an exact image-agent route", (method, path) => {
    expect(() => buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits", path, "", method,
    )).not.toThrow();
  });

  it.each([
    ["GET", ["image-agent", "admin"]],
    ["GET", ["image-agent", "runs", "run-1", "secrets"]],
    ["POST", ["image-agent", "runs", "run-1", "events"]],
    ["GET", ["image-agent", "runs", "run-1", "restart"]],
    ["GET", ["image-agent", "runs", "run-1", "events", "extra"]],
    ["POST", ["image-agent", "runs", "run-1", "slots", "scene-2", "attempts", "0", "recover"]],
    ["GET", ["image-agent", "runs", "run%2F1"]],
    ["GET", ["image-agent", "runs", "run%5C1"]],
    ["GET", ["image-agent", "runs", "run-1", "unknown"]],
  ])("rejects %s for unsafe or unknown image-agent route %j", (method, path) => {
    expect(() => buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits", path, "", method,
    )).toThrow("image-agent proxy route is not allowed");
  });
});
