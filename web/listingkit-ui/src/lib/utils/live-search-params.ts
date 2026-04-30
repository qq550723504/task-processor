"use client";

import { useEffect, useMemo, useState } from "react";

const BROWSER_HISTORY_EVENT = "codex:browser-history-change";

function readSearch() {
  if (typeof window === "undefined") {
    return "";
  }
  return window.location.search;
}

export function useLiveSearchParams() {
  const [search, setSearch] = useState(readSearch);

  useEffect(() => {
    function handleChange() {
      setSearch(readSearch());
    }

    window.addEventListener("popstate", handleChange);
    window.addEventListener(BROWSER_HISTORY_EVENT, handleChange);
    return () => {
      window.removeEventListener("popstate", handleChange);
      window.removeEventListener(BROWSER_HISTORY_EVENT, handleChange);
    };
  }, []);

  return useMemo(() => new URLSearchParams(search), [search]);
}

export function dispatchBrowserHistoryChange() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new Event(BROWSER_HISTORY_EVENT));
}
