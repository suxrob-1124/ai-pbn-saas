"use client";

import { useEffect, useState } from "react";

import { MonacoDiffEditor } from "./MonacoDiffEditor";
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
  const [showDiff, setShowDiff] = useState(true);

  useEffect(() => {
    if (!domainId || (!fileId && !filePath)) {
      setItems([]);
      setRevisions([]);
      setError(null);
      setSelectedRev("");
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

  useEffect(() => {
    if (!filePath) return;
    if (revisions.length === 0) {
      setSelectedRev("");
      return;
    }
    setSelectedRev((prev) => {
      if (prev && revisions.some((item) => item.id === prev)) {
        return prev;
      }
      return revisions[0].id;
    });
  }, [filePath, revisions]);

  const selected = revisions.find((item) => item.id === selectedRev) || null;
  const selectedIndex = selected ? revisions.findIndex((item) => item.id === selected.id) : -1;
  const previousRevision = selectedIndex >= 0 ? revisions[selectedIndex + 1] || null : null;

  const detectLanguage = (pathValue?: string) => {
    const path = (pathValue || "").toLowerCase();
    if (path.endsWith(".html") || path.endsWith(".htm")) return "html";
    if (path.endsWith(".css")) return "css";
    if (path.endsWith(".js") || path.endsWith(".mjs") || path.endsWith(".cjs")) return "javascript";
    if (path.endsWith(".ts") || path.endsWith(".tsx")) return "typescript";
    if (path.endsWith(".json")) return "json";
    if (path.endsWith(".xml") || path.endsWith(".svg")) return "xml";
    if (path.endsWith(".md") || path.endsWith(".markdown")) return "markdown";
    return "plaintext";
  };

  const isTextRevision = (rev: FileRevisionDTO | null) => {
    if (!rev) return false;
    if (typeof rev.content === "string" && rev.content.length > 0) return true;
    const mime = (rev.mime_type || "").toLowerCase();
    return mime.startsWith("text/") || mime === "application/json" || mime === "application/xml" || mime === "image/svg+xml";
  };

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
                onClick={() => {
                  setSelectedRev(item.id);
                  setShowDiff(true);
                }}
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
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setShowDiff(true)}
                    disabled={!previousRevision || !isTextRevision(selected)}
                    className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    View diff
                  </button>
                  <button
                    type="button"
                    onClick={() => setShowDiff(false)}
                    className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-[11px] font-semibold text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    View content
                  </button>
                </div>
                {showDiff && previousRevision && isTextRevision(selected) ? (
                  <MonacoDiffEditor
                    original={previousRevision.content || ""}
                    modified={selected.content || ""}
                    language={detectLanguage(filePath)}
                  />
                ) : (
                  <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-slate-100 p-2 text-xs dark:bg-slate-900/70">
                    {selected.content || "(бинарная ревизия, diff недоступен)"}
                  </pre>
                )}
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
