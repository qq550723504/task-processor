import { dispatchBrowserHistoryChange } from "@/lib/utils/live-search-params";

export function replaceBrowserHistory(href: string) {
  if (typeof window === "undefined") {
    return;
  }
  window.history.replaceState(window.history.state, "", href);
  dispatchBrowserHistoryChange();
}
