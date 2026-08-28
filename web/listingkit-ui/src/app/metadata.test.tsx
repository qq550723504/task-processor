import { headers } from "next/headers";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn() }));

import { generateMetadata } from "./layout";

describe("application metadata", () => {
  it("keeps the application shell branded as ListingKit", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ host: "listingkit.example" }));

    const metadata = await generateMetadata();

    expect(metadata.title).toBe("ListingKit | 一份资料，多平台增长");
    expect(metadata.openGraph).toMatchObject({
      title: "ListingKit | 一份资料，多平台增长",
      images: ["https://listingkit.example/og-v2.png"],
    });
  });

  it("applies Sumi metadata only to the public home page", async () => {
    const homePage = await import("./page");

    expect(homePage.metadata).toMatchObject({
      title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
      openGraph: {
        title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
        images: ["/sumi/fd824975-1e65-4585-9ebf-212d68cb1507.png"],
      },
    });
  });
});
