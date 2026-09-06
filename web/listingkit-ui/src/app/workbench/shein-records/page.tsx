import { notFound } from "next/navigation";

export default function SheinRecordsPage() {
  // #328 is paused until #331 approves the shared product projection.
  // Backend configuration alone must not enable this withdrawn entry.
  notFound();
}
