import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

function filenameWithBlobExtension(filename: string, blobType: string): string {
  if (/\.[a-z0-9]{2,5}$/i.test(filename)) {
    return filename;
  }
  const subtype = blobType.split("/")[1]?.split(";")[0]?.trim().toLowerCase();
  return subtype ? `${filename}.${subtype === "jpeg" ? "jpg" : subtype}` : filename;
}

export async function downloadStudioImage(
  src: string,
  filename: string,
): Promise<void> {
  const response = await fetch(toImageProxyUrl(src, { forceProxy: true }));
  if (!response.ok) {
    throw new Error(
      `Failed to download image: ${response.status} ${response.statusText}`,
    );
  }

  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);

  try {
    const anchor = document.createElement("a");
    anchor.href = objectUrl;
    anchor.download = filenameWithBlobExtension(filename, blob.type);
    anchor.click();
  } finally {
    URL.revokeObjectURL(objectUrl);
  }
}
