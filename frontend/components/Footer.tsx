"use client";

import Link from "next/link";

export function Footer() {
  return (
    <footer className="mt-8 border-t border-slate-200 dark:border-slate-800 pt-6 text-sm text-slate-500 dark:text-slate-400">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span>SiteGen AI</span>
          <span className="text-slate-300 dark:text-slate-700">·</span>
          <span>Мониторинг и генерации</span>
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <Link
            href={{ pathname: "/docs" }}
            className="font-semibold text-slate-700 underline underline-offset-4 hover:text-slate-900 dark:text-slate-200 dark:hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
          >
            Документация
          </Link>
          <span className="text-xs">© 2026</span>
        </div>
      </div>
    </footer>
  );
}
