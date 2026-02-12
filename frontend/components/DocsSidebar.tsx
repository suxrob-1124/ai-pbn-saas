"use client";

import Link from "next/link";
import type { Route } from "next";
import { usePathname } from "next/navigation";
import { useOptionalMe } from "../lib/useAuth";

const sections = [
  {
    title: "Основы",
    items: [
      { href: "/docs", label: "Обзор" },
      { href: "/docs/projects", label: "Проекты" },
      { href: "/docs/domains", label: "Домены" },
    ],
  },
  {
    title: "Планирование",
    items: [
      { href: "/docs/schedules", label: "Расписания" },
      { href: "/docs/queue", label: "Очередь" },
    ],
  },
  {
    title: "Мониторинг",
    items: [{ href: "/docs/indexing", label: "Индексация" }],
  },
  {
    title: "Ссылки и ошибки",
    items: [
      { href: "/docs/links", label: "Ссылки" },
      { href: "/docs/errors", label: "Ошибки" },
    ],
  },
  {
    title: "API",
    items: [
      { href: "/docs/api", label: "Swagger UI" },
      { href: "/docs/indexing-api", label: "API индексации" },
    ],
  },
];

export function DocsSidebar() {
  const pathname = usePathname();
  const { me } = useOptionalMe();
  const isAdmin = (me?.role || "").toLowerCase() === "admin";
  const visibleSections = isAdmin
    ? sections
    : sections.filter((section) => section.title !== "API");

  return (
    <aside className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900/70">
      <div className="mb-4">
        <p className="text-xs font-semibold uppercase tracking-[0.25em] text-slate-500">
          Документация
        </p>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Навигация по основным разделам.
        </p>
      </div>
      <nav className="space-y-5">
        {visibleSections.map((section) => (
          <div key={section.title}>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-slate-400">
              {section.title}
            </p>
            <div className="mt-2 flex flex-col gap-1">
              {section.items.map((item) => {
                const isActive =
                  item.href === "/docs"
                    ? pathname === "/docs"
                    : pathname.startsWith(item.href);
                return (
                  <Link
                    key={item.href}
                    href={item.href as Route}
                    className={
                      "rounded-lg px-3 py-2 text-sm font-medium transition " +
                      (isActive
                        ? "bg-slate-900 text-white shadow-sm dark:bg-slate-100 dark:text-slate-900"
                        : "text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800")
                    }
                  >
                    {item.label}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>
    </aside>
  );
}
