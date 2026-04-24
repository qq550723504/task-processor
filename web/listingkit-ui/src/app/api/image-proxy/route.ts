import { NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

const MAX_IMAGE_BYTES = 25 * 1024 * 1024;

export async function GET(request: NextRequest) {
  const rawUrl = request.nextUrl.searchParams.get("url")?.trim();
  if (!rawUrl) {
    return NextResponse.json(
      { error: "missing_url", message: "Image URL is required." },
      { status: 400 },
    );
  }

  let url: URL;
  try {
    url = new URL(rawUrl);
  } catch {
    return NextResponse.json(
      { error: "invalid_url", message: "Image URL is invalid." },
      { status: 400 },
    );
  }

  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return NextResponse.json(
      { error: "unsupported_protocol", message: "Only http and https images are supported." },
      { status: 400 },
    );
  }

  const upstream = await fetch(url.toString(), {
    headers: {
      Accept: "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
      Referer: "https://www.sdsdiy.com/",
      "User-Agent":
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147 Safari/537.36",
    },
    cache: "no-store",
  });

  if (!upstream.ok) {
    return NextResponse.json(
      {
        error: "image_fetch_failed",
        message: `Image fetch failed: ${upstream.status}`,
        url: url.toString(),
      },
      { status: upstream.status },
    );
  }

  const contentLength = Number(upstream.headers.get("content-length") ?? 0);
  if (contentLength > MAX_IMAGE_BYTES) {
    return NextResponse.json(
      { error: "image_too_large", message: "Image is too large for preview." },
      { status: 413 },
    );
  }

  const contentType = upstream.headers.get("content-type") ?? "image/jpeg";
  const body = await upstream.arrayBuffer();
  if (body.byteLength > MAX_IMAGE_BYTES) {
    return NextResponse.json(
      { error: "image_too_large", message: "Image is too large for preview." },
      { status: 413 },
    );
  }

  return new NextResponse(body, {
    headers: {
      "Cache-Control": "no-store",
      "Content-Type": contentType,
    },
  });
}
