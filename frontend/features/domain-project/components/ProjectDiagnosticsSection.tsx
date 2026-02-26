import Link from "next/link";
import { FiRefreshCw } from "react-icons/fi";

type Generation = {
  id: string;
  domain_id?: string;
  domain_url?: string | null;
  error?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
};

type DomainLite = {
  id: string;
  url: string;
};

type ProjectDiagnosticsSectionProps = {
  loading: boolean;
  error: string | null;
  items: Generation[];
  domainById: Record<string, DomainLite>;
  formatDateTime: (value?: string) => string;
  onRefresh: () => void;
};

export function ProjectDiagnosticsSection({
  loading,
  error,
  items,
  domainById,
  formatDateTime,
  onRefresh
}: ProjectDiagnosticsSectionProps) {
  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-semibold">Ошибки генерации</h3>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Последние сбои генерации по доменам проекта с быстрым переходом в карточку запуска.
          </p>
        </div>
        <button
          onClick={onRefresh}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiRefreshCw /> Обновить
        </button>
      </div>
      {loading && <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка ошибок...</div>}
      {!loading && error && <div className="text-sm text-red-500">{error}</div>}
      {!loading && !error && items.length === 0 && (
        <div className="text-sm text-slate-500 dark:text-slate-400">Ошибок пока нет.</div>
      )}
      {!loading && !error && items.length > 0 && (
        <div className="space-y-3">
          {items.map((item) => {
            const domain = item.domain_id ? domainById[item.domain_id] : undefined;
            const label = domain?.url || item.domain_url || "Неизвестный домен";
            const when = item.updated_at || item.finished_at || item.started_at || item.created_at;
            const timeLabel = formatDateTime(when);
            const message = (item.error || "Ошибка не указана").trim();
            const shortMessage = message.length > 160 ? `${message.slice(0, 160)}…` : message;
            return (
              <div
                key={item.id}
                className="flex flex-col gap-3 rounded-lg border border-slate-200 bg-white/90 px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-900/70 md:flex-row md:items-center md:justify-between"
              >
                <div className="space-y-1">
                  <div className="font-semibold text-slate-900 dark:text-slate-100">{label}</div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">
                    {timeLabel} · {shortMessage}
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Link
                    href={`/queue/${item.id}`}
                    className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    Открыть
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
