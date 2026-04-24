import type { SDSProductVariantSelection } from "@/lib/types/sds";

const STORAGE_KEY = "listingkit:sds:recent-variants";
const CHANGE_EVENT = "listingkit:sds:recent-variants:change";
const MAX_ITEMS = 6;
const emptyRecentVariants: SDSProductVariantSelection[] = [];

let cachedRaw = "";
let cachedItems: SDSProductVariantSelection[] = emptyRecentVariants;

function canUseStorage() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

export function loadRecentSDSVariants() {
  if (!canUseStorage()) {
    return emptyRecentVariants;
  }

  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    cachedRaw = "";
    cachedItems = emptyRecentVariants;
    return cachedItems;
  }

  if (raw === cachedRaw) {
    return cachedItems;
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      cachedRaw = raw;
      cachedItems = emptyRecentVariants;
      return cachedItems;
    }

    const filtered = parsed.filter((item): item is SDSProductVariantSelection => {
      return (
        !!item &&
        typeof item === "object" &&
        typeof item.variantId === "number" &&
        typeof item.productId === "number" &&
        typeof item.parentProductId === "number" &&
        typeof item.prototypeGroupId === "number" &&
        typeof item.layerId === "string" &&
        typeof item.productName === "string"
      );
    });
    cachedRaw = raw;
    cachedItems = filtered.length > 0 ? filtered : emptyRecentVariants;
    return cachedItems;
  } catch {
    cachedRaw = raw;
    cachedItems = emptyRecentVariants;
    return cachedItems;
  }
}

export function saveRecentSDSVariant(selection: SDSProductVariantSelection) {
  if (!canUseStorage()) {
    return;
  }

  const next = [
    selection,
    ...loadRecentSDSVariants().filter((item) => item.variantId !== selection.variantId),
  ].slice(0, MAX_ITEMS);

  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

export function subscribeRecentSDSVariants(onStoreChange: () => void) {
  if (!canUseStorage()) {
    return () => {};
  }

  const handleStorage = (event: StorageEvent) => {
    if (!event.key || event.key === STORAGE_KEY) {
      onStoreChange();
    }
  };

  const handleLocalChange = () => {
    onStoreChange();
  };

  window.addEventListener("storage", handleStorage);
  window.addEventListener(CHANGE_EVENT, handleLocalChange);

  return () => {
    window.removeEventListener("storage", handleStorage);
    window.removeEventListener(CHANGE_EVENT, handleLocalChange);
  };
}
