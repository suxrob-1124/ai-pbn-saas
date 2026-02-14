"use client";

import { useEffect, useState } from "react";

import {
  getFileHistory,
  getFileRevisionsByPath,
  revertFileToRevision,
  type FileEditHistoryItem,
  type FileRevisionDTO
} from "../lib/fileApi";
import { showToast } from "../lib/toastStore";

type FileHistoryProps = {
  domainId: string;
  fileId?: string;
  filePath?: string;
  refreshKey?: number;
  canWrite?: boolean;
  onReverted?: () => void;
};

export function FileHistory({
  domainId,
  fileId,
  filePath,
  refreshKey = 0,
  canWrite = false,
  onReverted
}: FileHistoryProps) {
  const [items, setItems] = useState<FileEditHistoryItem[]>([]);
  const [revisions, setRevisions] = useState<FileRevisionDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [reverting, setReverting] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [selectedRev, setSelectedRev] = useState<string>("");

  useEffect(() => {
    if (!domainId || (!fileId && !filePath)) {
      setItems([]);
      setRevisions([]);
      setError(null);
      return;
    }
    let alive = true;
    setLoading(true);
    setError(null);
    const run = async () => {
      try {
        if (filePath) {
          const rev = await getFileRevisionsByPath(domainId, filePath);
          if (!alive) return;
          setRevisions(Array.isArray(rev) ? rev : []);
          setItems([]);
        } else if (fileId) {
          const res = await getFileHistory(fileId, domainId);
          if (!alive) return;
          setItems(Array.isArray(res) ? res : []);
          setRevisions([]);
        }
      } catch (err: any) {
        if (!alive) return;
        setError(err?.message || "Не удалось загрузить историю файла");
      } finally {
        if (!alive) return;
        setLoading(false);
      }
    };
    void run();

    return () => {
      alive = false;
    };
  }, [domainId, fileId, filePath, refreshKey]);

  const selected = revisions.find((item) => item.id === selectedRev) || null;

  const onRevert = async (revisionId: string) => {
    if (!filePath || !canWrite || !revisionId) {
      return;
    }
    if (!confirm("Откатить файл к выбранной ревизии?")) {
      return;
    }
    setReverting(revisionId);
    try {
      await revertFileToRevision(domainId, filePath, revisionId, "revert from editor history");
      showToast({
        type: "success",
        title: "Файл откатан",
        message: filePath
      });
      onReverted?.();
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось откатить файл",
        message: err?.message || "unknown error"
      });
    } finally {
      setReverting("");
    }
  };

  return (
    <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">История файла</h3>
        <span className="text-xs text-slate-500 dark:text-slate-400">Diff/Revert v2</span>
      </div>
      {loading && <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">Загрузка...</div>}
      {error && <div className="mt-2 text-sm text-red-500">{error}</div>}
      {!loading && !error && !fileId && !filePath && (
        <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">Выберите файл для просмотра истории.</div>
      )}
      {!loading && !error && filePath && revisions.length === 0 && (
        <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">История ревизий пуста.</div>
      )}
      {!loading && !error && fileId && !filePath && items.length === 0 && (
        <div className="mt-2 text-sm text-slate-500 dark:text-slate-400">История изменений пуста.</div>
      )}
      {!loading && !error && revisions.length > 0 && (
        <div className="mt-2 grid gap-3 lg:grid-cols-[300px_1fr]">
          <div className="space-y-2">
            {revisions.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setSelectedRev(item.id)}
                className={`w-full rounded-lg border px-2 py-2 text-left text-xs ${
                  selectedRev === item.id
                    ? "border-indigo-500 bg-indigo-50 dark:bg-indigo-950/20"
                    : "border-slate-200 dark:border-slate-700"
                }`}
              >
                <div className="font-semibold">v{item.version} · {item.source}</div>
                <div className="text-slate-500 dark:text-slate-400">
                  {item.edited_by} · {new Date(item.created_at).toLocaleString()}
                </div>
                {item.description && (
                  <div className="mt-1 text-slate-600 dark:text-slate-300">{item.description}</div>
                )}
              </button>
            ))}
          </div>
          <div className="rounded-lg border border-slate-200 p-2 dark:border-slate-700">
            {!selected && (
              <div className="text-xs text-slate-500 dark:text-slate-400">Выберите ревизию для просмотра diff.</div>
            )}
            {selected && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs">
                  <div>Ревизия v{selected.version}</div>
                  {canWrite && (
                    <button
                      type="button"
                      onClick={() => onRevert(selected.id)}
                      disabled={reverting === selected.id}
                      className="rounded-lg border border-amber-200 bg-amber-50 px-2 py-1 font-semibold text-amber-700 hover:bg-amber-100 disabled:opacity-60 dark:border-amber-800 dark:bg-amber-900/30 dark:text-amber-300"
                    >
                      {reverting === selected.id ? "Откат..." : "Revert to this version"}
                    </button>
                  )}
                </div>
                <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-slate-100 p-2 text-xs dark:bg-slate-900/70">
                  {selected.content || "(бинарная ревизия, diff недоступен)"}
                </pre>
              </div>
            )}
          </div>
        </div>
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
