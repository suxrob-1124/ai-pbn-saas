import type { ReactNode } from "react";

type PaginationControlsProps = {
  page: number;
  hasNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  prevDisabled?: boolean;
  nextDisabled?: boolean;
  prevLabel?: string;
  nextLabel?: string;
  pageLabel?: ReactNode;
  middleSlot?: ReactNode;
};

export function PaginationControls({
  page,
  hasNext,
  onPrev,
  onNext,
  prevDisabled,
  nextDisabled,
  prevLabel = "Назад",
  nextLabel = "Вперёд",
  pageLabel,
  middleSlot
}: PaginationControlsProps) {
  return (
    <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
      <span>{pageLabel ?? `Страница ${page}`}</span>
      {middleSlot ? <div className="flex items-center gap-2">{middleSlot}</div> : <span />}
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={onPrev}
          disabled={Boolean(prevDisabled) || page <= 1}
          className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          {prevLabel}
        </button>
        <button
          type="button"
          onClick={onNext}
          disabled={Boolean(nextDisabled) || !hasNext}
          className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          {nextLabel}
        </button>
      </div>
    </div>
  );
}

