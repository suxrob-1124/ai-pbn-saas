"use client";

import { useEffect, useState } from "react";
import { dismissToast, subscribeToasts, ToastItem } from "../lib/toastStore";

const styles: Record<string, string> = {
  success: "border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-100",
  error: "border-red-200 bg-red-50 text-red-900 dark:border-red-700 dark:bg-red-900/40 dark:text-red-100",
  info: "border-slate-200 bg-white text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100",
  warning: "border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-700 dark:bg-amber-900/40 dark:text-amber-100"
};

export function ToastHost() {
  const [items, setItems] = useState<ToastItem[]>([]);

  useEffect(() => {
    const unsubscribe = subscribeToasts(setItems);
    return () => {
      unsubscribe();
    };
  }, []);

  if (items.length === 0) {
    return null;
  }

  return (
    <div className="fixed right-6 top-6 z-50 flex w-full max-w-sm flex-col gap-3">
      {items.map((toast) => (
        <div
          key={toast.id}
          className={`rounded-xl border p-3 shadow-lg ${styles[toast.type] || styles.info}`}
        >
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold">{toast.title}</div>
              {toast.message && <div className="text-xs opacity-80 mt-1">{toast.message}</div>}
            </div>
            <button
              onClick={() => dismissToast(toast.id)}
              className="text-xs opacity-60 hover:opacity-100"
              aria-label="Закрыть"
            >
              ✕
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
