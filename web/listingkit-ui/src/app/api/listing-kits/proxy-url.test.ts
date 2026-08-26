import { buildListingKitProxyUrl } from "@/app/api/listing-kits/proxy-url";

describe("buildListingKitProxyUrl", () => {
  it("joins the upstream base with a nested path and query string", () => {
    const result = buildListingKitProxyUrl(
      "http://localhost:8080/api/v1/listing-kits",
      ["tasks", "task_123", "generation-queue"],
      "page=1&page_size=20",
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
    );

    expect(result).toBe(
      "http://localhost:8080/api/v1/image-agent/runs/run-1/events",
    );
  });

  it("does not treat similar or traversal-like prefixes as an upstream route", () => {
    expect(
      buildListingKitProxyUrl(
        "http://localhost:8080/api/v1/listing-kits",
        ["image-agent-evil", "runs"],
        "",
      ),
    ).toBe(
      "http://localhost:8080/api/v1/listing-kits/image-agent-evil/runs",
    );
    expect(() =>
      buildListingKitProxyUrl(
        "http://localhost:8080/api/v1/listing-kits",
        ["image-agent", "..", "admin"],
        "",
      ),
    ).toThrow("invalid proxy path segment");
  });
});
