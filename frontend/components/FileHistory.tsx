"use client";

import { useEffect, useState } from "react";

import { getFileHistory, type FileEditHistoryItem } from "../lib/fileApi";

type FileHistoryProps = {
  domainId: string;
  fileId?: string;
  refreshKey?: number;
};

export function FileHistory({ domainId, fileId, refreshKey = 0 }: FileHistoryProps) {
  const [items, setItems] = useState<FileEditHistoryItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!domainId || !fileId) {
      setItems([]);
      setError(null);
      return;
    }
    let alive = true;
    setLoading(true);
    setError(null);
    getFileHistory(fileId, domainId)
      .then((res) => {
        if (!alive) return;
        setItems(Array.isArray(res) ? res : []);
      })
      .catch((err: any) => {
        if (!alive) return;
        setError(err?.message || "Не удалось загрузить историю файла");
      })
      .finally(() => {
        if (!alive) return;
        setLoading(false);
      });

    return () => {
      alive = false;
    };
  }, [domainId, fileId, refreshKey]);

  return (
    <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">История файла</h3>
        <span className="text-xs text-slate-500 dark:text-slate-400">Diff/Revert появятся в v2</span>
      </div>
      {loading && <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">Загрузка...</div>}
      {error && <div className="mt-2 text-sm text-red-500">{error}</div>}
      {!loading && !error && !fileId && (
        <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">Выберите файл для просмотра истории.</div>
      )}
      {!loading && !error && fileId && items.length === 0 && (
        <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">История изменений пуста.</div>
      )}
      {!loading && !error && items.length > 0 && (
        <div className="mt-2 space-y-2">
          {items.map((item) => (
            <div key={item.id} className="rounded-lg border border-slate-200 p-2 text-xs dark:border-slate-700">
              <div className="font-semibold text-slate-700 dark:text-slate-200">{item.editType || "manual"}</div>
              <div className="text-slate-500 dark:text-slate-400">
                {item.editedBy} • {new Date(item.createdAt).toLocaleString()}
              </div>
              {item.description && (
                <div className="mt-1 text-slate-600 dark:text-slate-300">{item.description}</div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
