"use client";

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";

import { cn } from "@/lib/utils/cn";

type ToastVariant = "info" | "success" | "warning" | "error";

type ToastInput = {
  title: string;
  description?: string;
  durationMs?: number;
  variant?: ToastVariant;
};

type ToastRecord = ToastInput & {
  id: string;
  variant: ToastVariant;
};

type ToastAPI = {
  dismiss: (id: string) => void;
  show: (input: ToastInput) => string;
  error: (title: string, description?: string, durationMs?: number) => string;
  info: (title: string, description?: string, durationMs?: number) => string;
  success: (title: string, description?: string, durationMs?: number) => string;
  warning: (title: string, description?: string, durationMs?: number) => string;
};

const noop = () => "";
const toastContext = createContext<ToastAPI>({
  dismiss: () => undefined,
  show: noop,
  error: noop,
  info: noop,
  success: noop,
  warning: noop,
});

const DEFAULT_TOAST_DURATION_MS = 6000;

function toastCardClassName(variant: ToastVariant) {
  switch (variant) {
    case "success":
      return "border-emerald-200 bg-emerald-50 text-emerald-950";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-950";
    case "error":
      return "border-rose-200 bg-rose-50 text-rose-950";
    default:
      return "border-sky-200 bg-sky-50 text-sky-950";
  }
}

function toastAccentClassName(variant: ToastVariant) {
  switch (variant) {
    case "success":
      return "bg-emerald-500";
    case "warning":
      return "bg-amber-500";
    case "error":
      return "bg-rose-500";
    default:
      return "bg-sky-500";
  }
}

export function ToastProvider({ children }: PropsWithChildren) {
  const [toasts, setToasts] = useState<ToastRecord[]>([]);
  const timeoutsRef = useRef(new Map<string, ReturnType<typeof globalThis.setTimeout>>());

  const dismiss = useCallback((id: string) => {
    const timeoutId = timeoutsRef.current.get(id);
    if (timeoutId) {
      globalThis.clearTimeout(timeoutId);
      timeoutsRef.current.delete(id);
    }
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);

  const show = useCallback(
    ({
      durationMs = DEFAULT_TOAST_DURATION_MS,
      variant = "info",
      ...input
    }: ToastInput) => {
      const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      setToasts((current) => [...current, { id, variant, ...input }]);
      const timeoutId = globalThis.setTimeout(() => dismiss(id), durationMs);
      timeoutsRef.current.set(id, timeoutId);
      return id;
    },
    [dismiss],
  );

  const api = useMemo<ToastAPI>(
    () => ({
      dismiss,
      show,
      error: (title, description, durationMs) =>
        show({ title, description, durationMs, variant: "error" }),
      info: (title, description, durationMs) =>
        show({ title, description, durationMs, variant: "info" }),
      success: (title, description, durationMs) =>
        show({ title, description, durationMs, variant: "success" }),
      warning: (title, description, durationMs) =>
        show({ title, description, durationMs, variant: "warning" }),
    }),
    [dismiss, show],
  );

  return (
    <toastContext.Provider value={api}>
      {children}
      <div className="pointer-events-none fixed inset-x-0 top-4 z-[80] flex justify-center px-4">
        <div className="flex w-full max-w-xl flex-col gap-3">
          {toasts.map((toast) => (
            <div
              key={toast.id}
              className={cn(
                "pointer-events-auto rounded-2xl border shadow-lg backdrop-blur-sm",
                toastCardClassName(toast.variant),
              )}
              role="status"
            >
              <div className="flex items-start gap-3 px-4 py-3">
                <span
                  aria-hidden="true"
                  className={cn(
                    "mt-1 h-2.5 w-2.5 shrink-0 rounded-full",
                    toastAccentClassName(toast.variant),
                  )}
                />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold leading-5">{toast.title}</p>
                  {toast.description ? (
                    <p className="mt-1 text-sm leading-5 opacity-90">{toast.description}</p>
                  ) : null}
                </div>
                <button
                  aria-label="关闭提示"
                  className="rounded-full px-2 py-1 text-xs font-medium opacity-70 transition hover:opacity-100"
                  onClick={() => dismiss(toast.id)}
                  type="button"
                >
                  关闭
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </toastContext.Provider>
  );
}

export function useToast() {
  return useContext(toastContext);
}
