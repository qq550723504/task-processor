import { notFound } from "next/navigation";

export default function SheinRecordsPage() {
  // R328 uses the shared task center; this standalone entry stays withdrawn.
  // Backend configuration alone must not enable this withdrawn entry.
  notFound();
}
