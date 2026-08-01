import type {
  WorkspacePlatformAdapter,
  WorkspacePlatformProjection,
} from "@/components/listingkit/workspace/workspace-platform-adapter";
import type { AmazonPreviewPayload } from "@/lib/types/listingkit";

export function createAmazonWorkspaceAdapter<TSheinProjection = never>({
  amazon,
}: {
  amazon?: AmazonPreviewPayload | null;
} = {}): WorkspacePlatformAdapter<
  WorkspacePlatformProjection<TSheinProjection>
> {
  const subtitle = [amazon?.brand, amazon?.product_type]
    .filter((value): value is string => Boolean(value?.trim()))
    .join(" · ");

  return {
    platform: "amazon",
    project: () => ({
      kind: "amazon",
      platform: "amazon",
      title: amazon?.title?.trim() || undefined,
      subtitle: subtitle || undefined,
    }),
  };
}
