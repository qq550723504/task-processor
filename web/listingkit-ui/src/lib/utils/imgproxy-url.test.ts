import { afterEach, describe, expect, it } from "vitest";

import { toImgproxyThumbnailUrl, toThumbnailPreviewUrl } from "@/lib/utils/imgproxy-url";

const IMGPROXY_ENV = "NEXT_PUBLIC_LISTINGKIT_IMGPROXY_BASE_URL";

describe("toImgproxyThumbnailUrl", () => {
  afterEach(() => {
    delete process.env[IMGPROXY_ENV];
  });

  it("returns the original url when imgproxy is not configured", () => {
    expect(
      toImgproxyThumbnailUrl("https://oss.shuomiai.com/listingkit-assets/20260529/demo.png", {
        width: 320,
        height: 320,
      }),
    ).toBe("https://oss.shuomiai.com/listingkit-assets/20260529/demo.png");
  });

  it("rewrites oss object urls to imgproxy thumbnail urls", () => {
    process.env[IMGPROXY_ENV] = "https://pod.shuomiai.com/img";

    expect(
      toImgproxyThumbnailUrl("https://oss.shuomiai.com/listingkit-assets/20260529/demo.png", {
        width: 320,
        height: 240,
      }),
    ).toBe(
      "https://pod.shuomiai.com/img/insecure/rs:fit:320:240/plain/s3://listingkit-assets/20260529/demo.png@webp",
    );
  });

  it("keeps non-oss urls unchanged", () => {
    process.env[IMGPROXY_ENV] = "https://pod.shuomiai.com/img";

    expect(
      toImgproxyThumbnailUrl("https://cdn.sdspod.com/images/example.jpg", {
        width: 320,
        height: 240,
      }),
    ).toBe("https://cdn.sdspod.com/images/example.jpg");
  });

  it("falls back to the existing image proxy for non-oss thumbnail previews", () => {
    process.env[IMGPROXY_ENV] = "https://pod.shuomiai.com/img";

    expect(
      toThumbnailPreviewUrl("https://cdn.sdspod.com/images/example.jpg", {
        width: 320,
        height: 240,
      }),
    ).toBe(
      `/api/image-proxy?url=${encodeURIComponent("https://cdn.sdspod.com/images/example.jpg")}`,
    );
  });

  it("keeps Tencent COS thumbnail previews direct", () => {
    process.env[IMGPROXY_ENV] = "https://pod.shuomiai.com/img";
    const url =
      "https://cos-1303159911.cos.na-ashburn.myqcloud.com/20260705/design.png";

    expect(
      toThumbnailPreviewUrl(url, {
        width: 320,
        height: 240,
      }),
    ).toBe(url);
  });

  it("keeps ListingKit uploads on the authenticated proxy when imgproxy is configured", () => {
    process.env[IMGPROXY_ENV] = "https://pod.shuomiai.com/img";

    expect(
      toThumbnailPreviewUrl(
        "/api/v1/listing-kits/uploads/files/0b15bb5e-9f9e-4952-9a06-fd31aab99901",
        { width: 320, height: 320 },
      ),
    ).toBe(
      "/api/listing-kits/uploads/files/0b15bb5e-9f9e-4952-9a06-fd31aab99901",
    );
  });
});
