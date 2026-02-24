import Link from "next/link";
import type { ReactNode } from "react";
import { FiPause, FiPlay, FiTrash2, FiX } from "react-icons/fi";

type GenerationRow = {
  id: string;
  status: string;
  progress: number;
  updated_at?: string;
  artifacts?: Record<string, unknown>;
};

type DomainGenerationStatusSectionProps = {
  generations: GenerationRow[];
  visibleGenerations: number;
  loading: boolean;
  renderStatusBadge: (status: string) => ReactNode;
  computeProgress: (generation: GenerationRow) => number;
  onResumeGeneration: (generationId: string) => void;
  onPauseGeneration: (generationId: string) => void;
  onCancelGeneration: (generationId: string) => void;
  onDeleteGeneration: (generationId: string) => void;
  onShowMore: () => void;
};

export function DomainGenerationStatusSection({
  generations,
  visibleGenerations,
  loading,
  renderStatusBadge,
  computeProgress,
  onResumeGeneration,
  onPauseGeneration,
  onCancelGeneration,
  onDeleteGeneration,
  onShowMore
}: DomainGenerationStatusSectionProps) {
  const visible = generations.slice(0, visibleGenerations);
  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">Запуски</h3>
        <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {generations.length}</span>
      </div>
      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead>
            <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
              <th className="py-2 pr-4">№</th>
              <th className="py-2 pr-4">Статус</th>
              <th className="py-2 pr-4">Прогресс</th>
              <th className="py-2 pr-4">Обновлено</th>
              <th className="py-2 pr-4">Действия</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
            {visible.map((generation, idx) => (
              <tr key={generation.id}>
                <td className="py-3 pr-4 text-xs text-slate-500 dark:text-slate-400">{idx + 1}</td>
                <td className="py-3 pr-4">{renderStatusBadge(generation.status)}</td>
                <td className="py-3 pr-4">{computeProgress(generation)}%</td>
                <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                  {generation.updated_at ? new Date(generation.updated_at).toLocaleString() : "—"}
                </td>
                <td className="py-3 pr-4">
                  <div className="flex items-center gap-3">
                    <Link href={`/queue/${generation.id}`} className="text-indigo-600 hover:underline">
                      Открыть
                    </Link>
                    {generation.status === "paused" && (
                      <button
                        onClick={() => onResumeGeneration(generation.id)}
                        disabled={loading}
                        className="text-emerald-500 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 disabled:opacity-50"
                        title="Возобновить"
                      >
                        <FiPlay className="w-4 h-4" />
                      </button>
                    )}
                    {(generation.status === "pending" ||
                      generation.status === "processing" ||
                      generation.status === "pause_requested" ||
                      generation.status === "cancelling") && (
                      <>
                        {generation.status !== "cancelling" && (
                          <button
                            onClick={() => onPauseGeneration(generation.id)}
                            disabled={loading || generation.status === "pause_requested"}
                            className="text-amber-500 hover:text-amber-700 dark:text-amber-400 dark:hover:text-amber-300 disabled:opacity-50"
                            title={generation.status === "pause_requested" ? "Пауза запрошена" : "Пауза"}
                          >
                            <FiPause className="w-4 h-4" />
                          </button>
                        )}
                        <button
                          onClick={() => onCancelGeneration(generation.id)}
                          disabled={loading || generation.status === "cancelling"}
                          className="text-orange-500 hover:text-orange-700 dark:text-orange-400 dark:hover:text-orange-300 disabled:opacity-50"
                          title={generation.status === "cancelling" ? "Отмена..." : "Отменить"}
                        >
                          <FiX className="w-4 h-4" />
                        </button>
                      </>
                    )}
                    {generation.status === "cancelled" && (
                      <button
                        onClick={() => onCancelGeneration(generation.id)}
                        disabled={loading}
                        className="text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
                        title="Отменить"
                      >
                        <FiX className="w-4 h-4" />
                      </button>
                    )}
                    <button
                      onClick={() => onDeleteGeneration(generation.id)}
                      disabled={loading}
                      className="text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
                      title="Удалить"
                    >
                      <FiTrash2 className="w-4 h-4" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {generations.length === 0 && (
              <tr>
                <td colSpan={5} className="py-4 text-center text-slate-500 dark:text-slate-400">
                  Запусков пока нет.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {generations.length > visibleGenerations && (
        <div className="pt-2">
          <button onClick={onShowMore} className="text-sm text-indigo-600 hover:underline">
            Показать ещё
          </button>
        </div>
      )}
    </div>
  );
}
