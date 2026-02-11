"use client";

import type { ReactNode } from "react";

type BadgeTone = "indigo" | "emerald" | "amber" | "slate" | "red" | "green" | "yellow" | "blue" | "orange" | "sky";

const TONE_CLASSES: Record<BadgeTone, string> = {
  indigo: "bg-indigo-600/10 text-indigo-600 dark:text-indigo-300",
  emerald: "bg-emerald-600/10 text-emerald-600 dark:text-emerald-300",
  amber: "bg-amber-500/10 text-amber-600 dark:text-amber-300",
  slate: "bg-slate-500/10 text-slate-600 dark:text-slate-300",
  red: "bg-red-600/10 text-red-600 dark:text-red-300",
  green: "bg-green-600/10 text-green-600 dark:text-green-300",
  yellow: "bg-yellow-500/10 text-yellow-700 dark:text-yellow-300",
  blue: "bg-blue-600/10 text-blue-600 dark:text-blue-300",
  orange: "bg-orange-600/10 text-orange-600 dark:text-orange-300",
  sky: "bg-sky-500/10 text-sky-600 dark:text-sky-300"
};

export function Badge({
  label,
  tone = "slate",
  className = "",
  icon
}: {
  label: string;
  tone?: BadgeTone;
  className?: string;
  icon?: ReactNode;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold ${TONE_CLASSES[tone]} ${className}`}
    >
      {icon}
      {label}
    </span>
  );
}
