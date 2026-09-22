import { PlatformSubscriptionPage } from "@/components/listingkit/subscription/platform-subscription-page";

export default function ListingKitPlatformSubscriptionRoute() {
  return (
    <PlatformSubscriptionPage
      tenantDirectoryEnabled={
        process.env.LISTINGKIT_PLATFORM_ADMIN_DIRECTORY_ENABLED !== "false"
      }
    />
  );
}
