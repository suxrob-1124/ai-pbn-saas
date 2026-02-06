"use client";

import { FiPlay } from "react-icons/fi";

type ScheduleTriggerProps = {
  onClick: () => void;
  disabled?: boolean;
};

export function ScheduleTrigger({ onClick, disabled }: ScheduleTriggerProps) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="inline-flex items-center gap-1 rounded-lg border border-emerald-200 bg-white px-3 py-1 text-xs font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:bg-slate-800 dark:text-emerald-300 disabled:opacity-50"
    >
      <FiPlay /> Запуск
    </button>
  );
}
