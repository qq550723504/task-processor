import { describe, expect, it } from "vitest";

import { DEFAULT_ALLOWED_HOSTS } from "./route";

describe("image proxy default allowlist", () => {
  it("includes the COS hosts used by Studio image URLs", () => {
    expect(DEFAULT_ALLOWED_HOSTS).toEqual(
      expect.arrayContaining([
        "cos-1303159911.cos.na-ashburn.myqcloud.com",
        "shuomi-1303159911.cos.ap-hongkong.myqcloud.com",
      ]),
    );
  });
});
