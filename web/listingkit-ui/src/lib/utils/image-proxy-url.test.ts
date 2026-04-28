import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

describe("toImageProxyUrl", () => {
  it("keeps OSS image URLs direct so browser can load public S3 assets", () => {
    const url = "https://oss.shuomiai.com/listingkit-assets/20260428/test.png";

    expect(toImageProxyUrl(url)).toBe(url);
  });

  it("proxies other remote images", () => {
    const url = "https://cdn.sdspod.com/images/test.jpg";

    expect(toImageProxyUrl(url)).toBe(
      `/api/image-proxy?url=${encodeURIComponent(url)}`,
    );
  });
});
