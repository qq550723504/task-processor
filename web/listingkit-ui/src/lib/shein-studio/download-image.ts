import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

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
    anchor.download = filename;
    anchor.click();
  } finally {
    URL.revokeObjectURL(objectUrl);
  }
}
