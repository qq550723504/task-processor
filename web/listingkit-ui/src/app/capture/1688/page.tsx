import type { Metadata } from "next";
import { CaptureReceiver } from "./capture-receiver";
export const metadata: Metadata = { title: "1688 Browser Capture", referrer: "no-referrer", robots: { index: false, follow: false } };
export default function BrowserCapturePage() { return <CaptureReceiver />; }
