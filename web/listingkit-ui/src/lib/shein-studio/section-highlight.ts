"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export const SHEIN_STUDIO_SECTION_FOCUS_EVENT =
  "listingkit:shein-studio:section-focus";

export type SheinStudioSectionFocusAction =
  | "recent-batches"
  | "product-picker";

export type SheinStudioSectionFocusDetail = {
  action?: SheinStudioSectionFocusAction;
  sectionId?: string;
};

export function dispatchSheinStudioSectionFocus(
  detail: SheinStudioSectionFocusDetail,
) {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(
    new CustomEvent<SheinStudioSectionFocusDetail>(
      SHEIN_STUDIO_SECTION_FOCUS_EVENT,
      { detail },
    ),
  );
}

export function resolveSheinStudioSectionFocusAction(
  detail: SheinStudioSectionFocusDetail,
) {
  if (detail.action === "recent-batches") {
    return "shein-studio-recent-batches";
  }
  if (detail.action === "product-picker") {
    return "shein-studio-product-picker";
  }
  return detail.sectionId ?? "";
}

export function useHighlightedSectionScroller(timeoutMs = 1800) {
  const [highlightedSectionId, setHighlightedSectionId] = useState("");
  const highlightTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const scrollToSection = useCallback((id: string) => {
    const element = document.getElementById(id);
    if (typeof element?.scrollIntoView === "function") {
      element.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    }
  }, []);

  const scrollToSectionWithHighlight = useCallback(
    (id: string) => {
      scrollToSection(id);
      setHighlightedSectionId(id);
      if (highlightTimeoutRef.current) {
        clearTimeout(highlightTimeoutRef.current);
      }
      highlightTimeoutRef.current = setTimeout(() => {
        setHighlightedSectionId((current) => (current === id ? "" : current));
      }, timeoutMs);
    },
    [scrollToSection, timeoutMs],
  );

  useEffect(() => {
    return () => {
      if (highlightTimeoutRef.current) {
        clearTimeout(highlightTimeoutRef.current);
      }
    };
  }, []);

  return {
    highlightedSectionId,
    scrollToSectionWithHighlight,
  };
}
