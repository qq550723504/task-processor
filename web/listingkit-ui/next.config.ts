import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "cdn.sdspod.com",
      },
      {
        protocol: "http",
        hostname: "cdn.sdspod.com",
      },
      {
        protocol: "http",
        hostname: "e.sdspod.com",
      },
      {
        protocol: "https",
        hostname: "e.sdspod.com",
      },
      {
        protocol: "https",
        hostname: "static-photo-center-prov.oss-cn-hangzhou.aliyuncs.com",
      },
    ],
  },
};

export default nextConfig;
