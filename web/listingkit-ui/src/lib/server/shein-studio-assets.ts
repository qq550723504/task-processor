import { getListingKitUpstreamBase } from "@/app/api/listing-kits/proxy-url";

function parseDataURL(dataUrl: string) {
  const match = dataUrl.match(/^data:([^;]+);base64,(.+)$/);
  if (!match) {
    throw new Error("Invalid generated image payload.");
  }

  return {
    contentType: match[1],
    buffer: Buffer.from(match[2], "base64"),
  };
}

function extensionForContentType(contentType: string) {
  switch (contentType) {
    case "image/jpeg":
      return "jpg";
    case "image/webp":
      return "webp";
    default:
      return "png";
  }
}

export async function persistGeneratedDesignAsset({
  id,
  dataUrl,
}: {
  id: string;
  dataUrl: string;
}) {
  const { contentType, buffer } = parseDataURL(dataUrl);
  const extension = extensionForContentType(contentType);
  const fileName = `${id}.${extension}`;
  const formData = new FormData();
  formData.append(
    "files",
    new Blob([buffer], { type: contentType }),
    fileName,
  );

  const response = await fetch(
    `${getListingKitUpstreamBase().replace(/\/+$/, "")}/uploads/images`,
    {
      method: "POST",
      body: formData,
      cache: "no-store",
    },
  );
  const payload = (await response.json().catch(() => ({}))) as {
    image_urls?: string[];
    message?: string;
  };

  if (!response.ok) {
    throw new Error(
      payload.message || `Persist generated design failed: ${response.status}`,
    );
  }

  const imageUrl = payload.image_urls?.[0]?.trim();
  if (!imageUrl) {
    throw new Error("Persist generated design returned no image URL.");
  }

  return {
    contentType,
    fileName,
    imageUrl,
  };
}
