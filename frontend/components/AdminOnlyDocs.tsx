"use client";

import type { ReactNode } from "react";
import { useAuthGuard } from "../lib/useAuth";

type AdminOnlyDocsProps = {
  children: ReactNode;
  title?: string;
};

export function AdminOnlyDocs({ children, title = "Доступ ограничен" }: AdminOnlyDocsProps) {
  const { me, loading } = useAuthGuard();
  const isAdmin = (me?.role || "").toLowerCase() === "admin";

  if (loading) {
    return (
      <div className="rounded-2xl border border-slate-200 bg-white/80 p-6 text-sm text-slate-600 shadow-sm dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300">
        Загрузка документации...
      </div>
    );
  }

  if (!isAdmin) {
    return (
      <div className="rounded-2xl border border-amber-200 bg-amber-50 p-6 text-sm text-amber-900 shadow-sm dark:border-amber-700/40 dark:bg-amber-900/20 dark:text-amber-100">
        <h2 className="text-base font-semibold">{title}</h2>
        <p className="mt-2">
          Раздел доступен только администраторам. Если доступ нужен, обратитесь к
          владельцу системы.
        </p>
      </div>
    );
  }

  if (!me) {
    return null;
  }

  return <>{children}</>;
}
