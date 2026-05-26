import type { SDSProductVariantSelection } from "@/lib/types/sds";
import { buildGroupedSDSSelectionID } from "@/lib/types/sds-baseline";

const STORAGE_KEY = "listingkit:sds:grouped-candidates";
const CHANGE_EVENT = "listingkit:sds:grouped-candidates:change";
const MAX_ITEMS = 24;
const emptyGroupedCandidates: SDSProductVariantSelection[] = [];

let cachedRaw = "";
let cachedItems: SDSProductVariantSelection[] = emptyGroupedCandidates;

function canUseStorage() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

function normalizeSelections(value: unknown) {
  if (!Array.isArray(value)) {
    return emptyGroupedCandidates;
  }
  const filtered = value.filter((item): item is SDSProductVariantSelection => {
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
  return filtered.length > 0 ? filtered : emptyGroupedCandidates;
}

function saveItems(items: SDSProductVariantSelection[]) {
  if (!canUseStorage()) {
    return;
  }
  const raw = JSON.stringify(items);
  window.localStorage.setItem(STORAGE_KEY, raw);
  cachedRaw = raw;
  cachedItems = items;
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

export function loadSDSGroupedCandidates() {
  if (!canUseStorage()) {
    return emptyGroupedCandidates;
  }

  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    cachedRaw = "";
    cachedItems = emptyGroupedCandidates;
    return cachedItems;
  }
  if (raw === cachedRaw) {
    return cachedItems;
  }

  try {
    cachedRaw = raw;
    cachedItems = normalizeSelections(JSON.parse(raw));
    return cachedItems;
  } catch {
    cachedRaw = raw;
    cachedItems = emptyGroupedCandidates;
    return cachedItems;
  }
}

export function saveSDSGroupedCandidate(selection: SDSProductVariantSelection) {
  const selectionId = buildGroupedSDSSelectionID(selection);
  const next = [
    selection,
    ...loadSDSGroupedCandidates().filter(
      (item) => buildGroupedSDSSelectionID(item) !== selectionId,
    ),
  ].slice(0, MAX_ITEMS);
  saveItems(next);
}

export function removeSDSGroupedCandidate(selection?: SDSProductVariantSelection) {
  const selectionId = buildGroupedSDSSelectionID(selection);
  if (!selectionId) {
    return;
  }
  saveItems(
    loadSDSGroupedCandidates().filter(
      (item) => buildGroupedSDSSelectionID(item) !== selectionId,
    ),
  );
}

export function hasSDSGroupedCandidate(selection?: SDSProductVariantSelection) {
  const selectionId = buildGroupedSDSSelectionID(selection);
  if (!selectionId) {
    return false;
  }
  return loadSDSGroupedCandidates().some(
    (item) => buildGroupedSDSSelectionID(item) === selectionId,
  );
}

export function subscribeSDSGroupedCandidates(onStoreChange: () => void) {
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
