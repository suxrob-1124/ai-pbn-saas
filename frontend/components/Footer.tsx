"use client";

import Link from "next/link";

export function Footer() {
  return (
    <footer className="mt-8 border-t border-slate-200 dark:border-slate-800 pt-6 text-sm text-slate-500 dark:text-slate-400">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span>SiteGen AI</span>
          <span className="text-slate-300 dark:text-slate-700">·</span>
          <span>Monitoring & Gen</span>
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <Link href="/docs" className="hover:text-slate-700 dark:hover:text-slate-200">
            Документация
          </Link>
          <span className="text-xs">© 2026</span>
        </div>
      </div>
    </footer>
  );
}
