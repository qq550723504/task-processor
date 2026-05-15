import path from "node:path";

export function resolveListingKitUILocalStorageDir() {
  return process.env.LISTINGKIT_UI_STORAGE_DIR?.trim() || path.join(process.cwd(), ".data");
}

export function resolveListingKitUILocalStoragePath(fileName: string) {
  return path.join(resolveListingKitUILocalStorageDir(), fileName);
}
