"use client";

import { useState } from "react";
import { FiRotateCcw } from "react-icons/fi";
import { apiBase, refreshTokens } from "@/lib/http";

type Props = {
  domainId: string;
  sessionId: string;
  onRolledBack: (restoredCount: number, deletedCount: number) => void;
};

export function AgentSessionRollback({ domainId, sessionId, onRolledBack }: Props) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleRollback = async () => {
    setLoading(true);
    setError("");
    try {
      const doFetch = () =>
        fetch(`${apiBase()}/api/domains/${domainId}/agent/${sessionId}/rollback`, {
          method: "POST",
          credentials: "include",
        });

      let res = await doFetch();
      if (res.status === 401) {
        await refreshTokens();
        res = await doFetch();
      }

      if (!res.ok) {
        let msg = `${res.status}`;
        try {
          const d = await res.json();
          if (d?.error) msg = d.error;
        } catch { /* ignore */ }
        setError(msg);
        return;
      }

      const data = await res.json();
      if (data.status !== "rolled_back") {
        setError("Нет данных для отката — агент не изменял файлы в этой сессии.");
        return;
      }
      setOpen(false);
      onRolledBack(data.restored ?? 0, data.deleted ?? 0);
    } catch (e) {
      setError((e as Error)?.message || "Ошибка сети");
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="inline-flex items-center gap-1 rounded-md border border-amber-300 bg-amber-50 px-2 py-1 text-xs font-medium text-amber-700 hover:bg-amber-100 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-300 dark:hover:bg-amber-950/40"
        title="Откатить изменения агента"
      >
        <FiRotateCcw className="h-3 w-3" /> Откат
      </button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-sm rounded-xl border border-slate-200 bg-white p-5 shadow-lg dark:border-slate-700 dark:bg-slate-900">
            <h3 className="mb-2 text-sm font-semibold text-slate-900 dark:text-white">
              Откатить изменения агента?
            </h3>
            <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">
              Все файлы, изменённые агентом в этой сессии, будут восстановлены из снэпшота до начала сессии.
              Это действие нельзя отменить.
            </p>
            {error && (
              <div className="mb-3 rounded-md bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-950/30 dark:text-red-400">
                {error}
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setOpen(false)}
                disabled={loading}
                className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                Отмена
              </button>
              <button
                type="button"
                onClick={handleRollback}
                disabled={loading}
                className="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-400 disabled:opacity-50"
              >
                {loading ? "Откат…" : "Откатить"}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
