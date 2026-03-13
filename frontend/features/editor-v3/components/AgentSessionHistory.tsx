"use client";

import { useCallback, useEffect, useState } from "react";
import { FiClock, FiChevronDown, FiChevronUp, FiRefreshCw } from "react-icons/fi";
import { authFetch } from "@/lib/http";
import { AgentSessionRollback } from "./AgentSessionRollback";

type SessionDTO = {
  id: string;
  domain_id: string;
  created_by: string;
  created_at: string;
  finished_at?: string;
  status: string;
  summary?: string;
  files_changed?: string[];
  message_count: number;
  snapshot_tag?: string;
};

const STATUS_LABELS: Record<string, string> = {
  running: "Выполняется",
  done: "Завершено",
  error: "Ошибка",
  stopped: "Остановлено",
  rolled_back: "Откачено",
};

const STATUS_COLORS: Record<string, string> = {
  running: "text-indigo-600 bg-indigo-50 dark:bg-indigo-950/30 dark:text-indigo-300",
  done: "text-emerald-600 bg-emerald-50 dark:bg-emerald-950/30 dark:text-emerald-300",
  error: "text-red-600 bg-red-50 dark:bg-red-950/30 dark:text-red-300",
  stopped: "text-slate-500 bg-slate-100 dark:bg-slate-800 dark:text-slate-400",
  rolled_back: "text-amber-600 bg-amber-50 dark:bg-amber-950/30 dark:text-amber-300",
};

type Props = {
  domainId: string;
  onLoadSession?: (sessionId: string) => void;
  onRolledBack?: (restoredCount: number, deletedCount: number) => void;
};

export function AgentSessionHistory({ domainId, onLoadSession, onRolledBack }: Props) {
  const [sessions, setSessions] = useState<SessionDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await authFetch<SessionDTO[]>(`/api/domains/${domainId}/agent/sessions`);
      setSessions(data || []);
    } catch (e) {
      setError((e as Error)?.message || "Не удалось загрузить историю");
    } finally {
      setLoading(false);
    }
  }, [domainId]);

  useEffect(() => {
    load();
  }, [load]);

  const formatDate = (iso: string) => {
    const d = new Date(iso);
    return d.toLocaleString("ru-RU", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
  };

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-xs font-semibold text-slate-500 dark:text-slate-400">
          <FiClock className="h-3 w-3" /> История сессий
        </div>
        <button
          type="button"
          onClick={load}
          disabled={loading}
          className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 disabled:opacity-50 dark:hover:bg-slate-800 dark:hover:text-slate-200"
          title="Обновить"
        >
          <FiRefreshCw className={`h-3 w-3 ${loading ? "animate-spin" : ""}`} />
        </button>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 px-2 py-1.5 text-xs text-red-600 dark:bg-red-950/20 dark:text-red-400">
          {error}
        </div>
      )}

      {!loading && sessions.length === 0 && !error && (
        <div className="text-xs text-slate-400 dark:text-slate-500">Сессий нет</div>
      )}

      <div className="space-y-2 max-h-80 overflow-y-auto pr-1">
        {sessions.map((sess) => {
          const statusLabel = STATUS_LABELS[sess.status] || sess.status;
          const statusColor = STATUS_COLORS[sess.status] || STATUS_COLORS.stopped;
          const hasSnapshot = Boolean(sess.snapshot_tag);
          const isExpanded = expandedId === sess.id;
          const hasDetails = Boolean(sess.status === "running" || sess.summary || (sess.files_changed && sess.files_changed.length > 0));

          return (
            <div
              key={sess.id}
              className="rounded-lg border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800/50 overflow-hidden"
            >
              {/* Header row — always visible, clickable to expand */}
              <button
                type="button"
                onClick={() => setExpandedId(isExpanded ? null : sess.id)}
                className="w-full flex items-center justify-between gap-2 px-2.5 py-2 text-left hover:bg-slate-50 dark:hover:bg-slate-700/40 transition-colors"
              >
                <div className="flex items-center gap-1.5 flex-wrap min-w-0">
                  <span className={`rounded-full px-1.5 py-0.5 text-[10px] font-medium shrink-0 ${statusColor}`}>
                    {statusLabel}
                  </span>
                  <span className="text-[10px] text-slate-400 dark:text-slate-500">
                    {formatDate(sess.created_at)}
                  </span>
                  <span className="text-[10px] text-slate-400 dark:text-slate-500">
                    {sess.message_count} сообщ.
                  </span>
                  {sess.files_changed && sess.files_changed.length > 0 && (
                    <span className="text-[10px] text-slate-400 dark:text-slate-500">
                      · {sess.files_changed.length} файл(ов)
                    </span>
                  )}
                </div>
                {hasDetails && (
                  isExpanded
                    ? <FiChevronUp className="h-3 w-3 text-slate-400 shrink-0" />
                    : <FiChevronDown className="h-3 w-3 text-slate-400 shrink-0" />
                )}
              </button>

              {/* Expanded details */}
              {isExpanded && (
                <div className="px-2.5 pb-2.5 border-t border-slate-100 dark:border-slate-700 pt-2 space-y-2">
                  {sess.summary && (
                    <p className="text-xs text-slate-600 dark:text-slate-300">
                      {sess.summary}
                    </p>
                  )}
                  {sess.files_changed && sess.files_changed.length > 0 && (
                    <div className="flex flex-col gap-0.5">
                      <span className="text-[10px] font-medium text-slate-400 dark:text-slate-500 uppercase tracking-wide">
                        Изменённые файлы
                      </span>
                      {sess.files_changed.map((f) => (
                        <span key={f} className="text-[10px] font-mono text-slate-500 dark:text-slate-400 truncate">
                          {f}
                        </span>
                      ))}
                    </div>
                  )}
                  <div className="text-[10px] text-slate-400 dark:text-slate-500">
                    Автор: {sess.created_by}
                    {sess.finished_at && ` · завершено ${formatDate(sess.finished_at)}`}
                  </div>
                  {hasSnapshot && sess.status !== "rolled_back" && sess.status !== "running" && (
                    <AgentSessionRollback
                      domainId={domainId}
                      sessionId={sess.id}
                      onRolledBack={(restoredCount, deletedCount) => {
                        load();
                        onRolledBack?.(restoredCount, deletedCount);
                      }}
                    />
                  )}
                  {sess.status === "running" && (
                    <button
                      type="button"
                      onClick={() => onLoadSession?.(sess.id)}
                      className="text-[10px] font-medium text-indigo-600 hover:underline dark:text-indigo-400"
                    >
                      Подключиться к сессии
                    </button>
                  )}
                  {(sess.status === "done" || sess.status === "stopped" || sess.status === "error") && (
                    <button
                      type="button"
                      onClick={() => onLoadSession?.(sess.id)}
                      className="text-[10px] text-indigo-500 hover:underline"
                    >
                      Загрузить переписку
                    </button>
                  )}
                </div>
              )}
              {/* Rollback outside expand for non-expandable sessions */}
              {!isExpanded && !hasDetails && hasSnapshot && sess.status !== "rolled_back" && sess.status !== "running" && (
                <div className="px-2.5 pb-2">
                  <AgentSessionRollback
                    domainId={domainId}
                    sessionId={sess.id}
                    onRolledBack={(restoredCount, deletedCount) => {
                      load();
                      onRolledBack?.(restoredCount, deletedCount);
                    }}
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
