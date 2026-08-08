import { afterEach, describe, expect, it, vi } from "vitest";

import { downloadStudioImage } from "@/lib/shein-studio/download-image";

describe("downloadStudioImage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("downloads a remote Studio image through the same-origin proxy", async () => {
    const blob = new Blob(["image-bytes"], { type: "image/png" });
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(blob, { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const createObjectURL = vi
      .fn<typeof URL.createObjectURL>()
      .mockReturnValue("blob:studio-design-1");
    const revokeObjectURL = vi.fn<typeof URL.revokeObjectURL>();
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL,
      revokeObjectURL,
    });

    const click = vi.fn();
    const anchor = {
      href: "",
      download: "",
      click,
    } as unknown as HTMLAnchorElement;
    const createElement = vi
      .spyOn(document, "createElement")
      .mockReturnValue(anchor);

    await downloadStudioImage(
      "https://cos-1303159911.cos.na-ashburn.myqcloud.com/20260705/design-1.png",
      "studio-design-1-original.png",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/image-proxy?url=https%3A%2F%2Fcos-1303159911.cos.na-ashburn.myqcloud.com%2F20260705%2Fdesign-1.png",
    );
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(createElement).toHaveBeenCalledWith("a");
    expect(anchor.href).toBe("blob:studio-design-1");
    expect(anchor.download).toBe("studio-design-1-original.png");
    expect(click).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:studio-design-1");
  });

  it("rejects when the proxied image download returns a non-2xx response", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response("bad gateway", { status: 502, statusText: "Bad Gateway" }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      downloadStudioImage("https://cdn.sdspod.com/images/design-1.png", "design-1.png"),
    ).rejects.toThrow("Failed to download image: 502 Bad Gateway");
  });
});
