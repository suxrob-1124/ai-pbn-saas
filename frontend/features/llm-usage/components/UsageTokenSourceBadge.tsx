type UsageTokenSourceBadgeProps = {
  tokenSource: string;
};

export function UsageTokenSourceBadge({ tokenSource }: UsageTokenSourceBadgeProps) {
  const normalized = (tokenSource || "").trim() || "unknown";
  const isEstimated = normalized !== "provider";

  return (
    <div className="flex items-center gap-1">
      <span className="rounded bg-slate-100 px-2 py-1 text-xs dark:bg-slate-800">{normalized}</span>
      {isEstimated && (
        <span
          title="Токены/стоимость рассчитаны оценочно, а не получены напрямую от провайдера"
          className="rounded bg-amber-100 px-2 py-1 text-xs text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
        >
          estimated
        </span>
      )}
    </div>
  );
}

