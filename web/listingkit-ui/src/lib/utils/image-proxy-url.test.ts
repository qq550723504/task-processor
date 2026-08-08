import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

describe("toImageProxyUrl", () => {
  it("keeps OSS image URLs direct so browser can load public S3 assets", () => {
    const url = "https://oss.shuomiai.com/listingkit-assets/20260428/test.png";

    expect(toImageProxyUrl(url)).toBe(url);
  });

  it("keeps Tencent COS image URLs direct so generated designs do not depend on the local proxy", () => {
    const url =
      "https://cos-1303159911.cos.na-ashburn.myqcloud.com/20260705/test.png";

    expect(toImageProxyUrl(url)).toBe(url);
  });

  it("keeps the Hong Kong COS image URLs direct", () => {
    const url =
      "https://shuomi-1303159911.cos.ap-hongkong.myqcloud.com/20260719/test.png";

    expect(toImageProxyUrl(url)).toBe(url);
  });

  it("can force public remote image URLs through the proxy for downloads", () => {
    const url =
      "https://cos-1303159911.cos.na-ashburn.myqcloud.com/20260705/test.png";

    expect(toImageProxyUrl(url, { forceProxy: true })).toBe(
      `/api/image-proxy?url=${encodeURIComponent(url)}`,
    );
  });

  it("proxies other remote images", () => {
    const url = "https://cdn.sdspod.com/images/test.jpg";

    expect(toImageProxyUrl(url)).toBe(
      `/api/image-proxy?url=${encodeURIComponent(url)}`,
    );
  });

  it("rewrites listingkit uploaded image URLs to the local upload fetch route", () => {
    const url =
      "/api/v1/listing-kits/uploads/files/20260610/2ce3d54a-8ce9-459c-8118-cf8586d06e2d.png";

    expect(toImageProxyUrl(url)).toBe(
      "/api/listing-kits/uploads/files/20260610/2ce3d54a-8ce9-459c-8118-cf8586d06e2d.png",
    );
  });

  it("rewrites absolute listingkit uploaded image URLs to the local upload fetch route", () => {
    const url =
      "http://localhost:8085/api/v1/listing-kits/uploads/files/20260610/2ce3d54a-8ce9-459c-8118-cf8586d06e2d.png?version=1";

    expect(toImageProxyUrl(url)).toBe(
      "/api/listing-kits/uploads/files/20260610/2ce3d54a-8ce9-459c-8118-cf8586d06e2d.png?version=1",
    );
  });

  it("keeps already normalized listingkit upload URLs unchanged", () => {
    const url =
      "/api/listing-kits/uploads/files/8068c1dc-6383-49c6-a635-8e1d79ee6e4d";

    expect(toImageProxyUrl(url)).toBe(url);
  });

  it("keeps data URLs usable even when forceProxy is requested", () => {
    const url = "data:image/png;base64,abc123";

    expect(toImageProxyUrl(url, { forceProxy: true })).toBe(url);
  });

  it("keeps same-origin listingkit upload URLs usable when forceProxy is requested", () => {
    const url =
      "/api/listing-kits/uploads/files/20260610/2ce3d54a-8ce9-459c-8118-cf8586d06e2d.png";

    expect(toImageProxyUrl(url, { forceProxy: true })).toBe(url);
  });
});
