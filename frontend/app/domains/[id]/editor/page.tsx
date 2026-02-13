"use client";

import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { FiArrowLeft, FiEdit3, FiEye, FiFolder } from "react-icons/fi";

import { EditorToolbar } from "../../../../components/EditorToolbar";
import { FileHistory } from "../../../../components/FileHistory";
import { FileTree } from "../../../../components/FileTree";
import { MonacoEditor } from "../../../../components/MonacoEditor";
import { authFetch } from "../../../../lib/http";
import { getFile, listFiles, saveFile, type FileListItem } from "../../../../lib/fileApi";
import { showToast } from "../../../../lib/toastStore";
import { useAuthGuard } from "../../../../lib/useAuth";
import type {
  EditorDirtyState,
  EditorFileMeta,
  EditorSelectionState
} from "../../../../types/editor";

type DomainSummaryResponse = {
  domain: {
    id: string;
    project_id: string;
    url: string;
    status: string;
  };
  project_name: string;
  my_role: "admin" | "owner" | "editor" | "viewer";
};

const editableMime = (mimeType: string) => {
  const normalized = (mimeType || "").toLowerCase();
  return (
    normalized.startsWith("text/") ||
    normalized === "application/json" ||
    normalized === "application/javascript" ||
    normalized === "application/xml" ||
    normalized === "image/svg+xml"
  );
};

const detectLanguage = (pathValue: string) => {
  const path = pathValue.toLowerCase();
  if (path.endsWith(".html") || path.endsWith(".htm")) return "html";
  if (path.endsWith(".css")) return "css";
  if (path.endsWith(".js") || path.endsWith(".mjs") || path.endsWith(".cjs")) return "javascript";
  if (path.endsWith(".ts")) return "typescript";
  if (path.endsWith(".tsx")) return "typescript";
  if (path.endsWith(".json")) return "json";
  if (path.endsWith(".xml") || path.endsWith(".svg")) return "xml";
  if (path.endsWith(".md") || path.endsWith(".markdown")) return "markdown";
  return "plaintext";
};

const looksBinary = (value: string) => value.includes("\u0000");

export default function DomainEditorPage() {
  useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const domainId = params?.id as string;

  const requestedPath = searchParams.get("path") || "";
  const requestedLineRaw = searchParams.get("line") || "";
  const requestedLine = Number.parseInt(requestedLineRaw, 10);

  const [summary, setSummary] = useState<DomainSummaryResponse | null>(null);
  const [files, setFiles] = useState<EditorFileMeta[]>([]);
  const [selection, setSelection] = useState<EditorSelectionState | null>(null);
  const [dirtyState, setDirtyState] = useState<EditorDirtyState>({
    isDirty: false,
    originalContent: "",
    currentContent: ""
  });
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(true);
  const [fileLoading, setFileLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [focusLine, setFocusLine] = useState<number | undefined>(
    Number.isFinite(requestedLine) && requestedLine > 0 ? requestedLine : undefined
  );
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0);

  const currentFile = useMemo(
    () => files.find((item) => item.path === selection?.selectedPath),
    [files, selection?.selectedPath]
  );

  const readOnly = (summary?.my_role || "viewer") === "viewer";
  const binaryPreview = looksBinary(dirtyState.currentContent);
  const canSave =
    !readOnly &&
    Boolean(currentFile?.editable) &&
    !binaryPreview &&
    dirtyState.isDirty &&
    !saving;

  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirtyState.isDirty) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [dirtyState.isDirty]);

  const updatePathQuery = (pathValue: string) => {
    const query = new URLSearchParams(searchParams.toString());
    query.set("path", pathValue);
    query.delete("line");
    const qs = query.toString();
    router.replace((`/domains/${domainId}/editor${qs ? `?${qs}` : ""}` as any), { scroll: false });
  };

  const loadFile = async (file: EditorFileMeta, options?: { line?: number }) => {
    if (dirtyState.isDirty) {
      const confirmed = window.confirm("Есть несохраненные изменения. Переключить файл без сохранения?");
      if (!confirmed) {
        return;
      }
    }
    setFileLoading(true);
    setError(null);
    try {
      const payload = await getFile(domainId, file.path);
      const content = payload?.content ?? "";
      const mimeType = payload?.mimeType || file.mimeType;
      const language = detectLanguage(file.path);
      setSelection({
        selectedPath: file.path,
        selectedFileId: file.id,
        language,
        mimeType
      });
      setDirtyState({
        isDirty: false,
        originalContent: content,
        currentContent: content
      });
      setDescription("");
      setFocusLine(options?.line);
      updatePathQuery(file.path);
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить файл");
    } finally {
      setFileLoading(false);
    }
  };

  useEffect(() => {
    if (!domainId) return;
    let alive = true;

    const run = async () => {
      setLoading(true);
      setError(null);
      try {
        const [summaryResp, fileList] = await Promise.all([
          authFetch<DomainSummaryResponse>(`/api/domains/${domainId}/summary?gen_limit=1&link_limit=1`),
          listFiles(domainId)
        ]);
        if (!alive) return;

        setSummary(summaryResp);
        const prepared: EditorFileMeta[] = (Array.isArray(fileList) ? fileList : []).map((item: FileListItem) => ({
          id: item.id,
          path: item.path,
          size: item.size,
          mimeType: item.mimeType,
          updatedAt: item.updatedAt,
          editable: editableMime(item.mimeType)
        }));
        setFiles(prepared);

        if (prepared.length === 0) {
          setSelection(null);
          setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
          return;
        }

        const queryTarget = requestedPath ? prepared.find((item) => item.path === requestedPath) : null;
        const firstEditable = prepared.find((item) => item.editable);
        const fallback = firstEditable || prepared[0];
        const target = queryTarget || fallback;
        await loadFile(target, {
          line: Number.isFinite(requestedLine) && requestedLine > 0 ? requestedLine : undefined
        });
      } catch (err: any) {
        if (!alive) return;
        setError(err?.message || "Не удалось загрузить редактор");
      } finally {
        if (alive) {
          setLoading(false);
        }
      }
    };

    run();
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [domainId]);

  const onSelectFile = async (file: EditorFileMeta) => {
    await loadFile(file);
  };

  const onSave = async () => {
    if (!selection || !currentFile) {
      return;
    }
    if (readOnly) {
      showToast({
        type: "error",
        title: "Недостаточно прав",
        message: "Viewer может только просматривать файлы."
      });
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await saveFile(domainId, selection.selectedPath, dirtyState.currentContent, description.trim() || undefined);
      setDirtyState((prev) => ({
        ...prev,
        isDirty: false,
        originalContent: prev.currentContent,
        lastSavedAt: new Date().toISOString()
      }));
      setDescription("");
      setHistoryRefreshKey((value) => value + 1);
      showToast({
        type: "success",
        title: "Файл сохранен",
        message: selection.selectedPath
      });
    } catch (err: any) {
      const message = err?.message || "Не удалось сохранить файл";
      if (String(message).toLowerCase().includes("editor role required") || String(message).startsWith("403")) {
        showToast({
          type: "error",
          title: "Недостаточно прав для сохранения",
          message: selection.selectedPath
        });
      } else {
        showToast({
          type: "error",
          title: "Ошибка сохранения",
          message
        });
      }
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  const onRevert = () => {
    setDirtyState((prev) => ({
      ...prev,
      isDirty: false,
      currentContent: prev.originalContent
    }));
    setDescription("");
  };

  const onDownload = () => {
    if (!selection?.selectedPath) return;
    const blob = new Blob([dirtyState.currentContent], { type: selection.mimeType || "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = selection.selectedPath.split("/").pop() || "file.txt";
    a.click();
    URL.revokeObjectURL(url);
  };

  if (loading) {
    return <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка редактора...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-slate-200 bg-white/80 p-4 shadow dark:border-slate-800 dark:bg-slate-900/60">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h1 className="text-xl font-semibold flex items-center gap-2">
              <FiFolder /> Редактор сайта
            </h1>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              {summary?.domain.url || "—"} • Проект: {summary?.project_name || "—"}
            </p>
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
              Роль: {summary?.my_role || "viewer"} {readOnly ? "(только чтение)" : "(редактирование включено)"}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Link
              href={`/domains/${domainId}`}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiArrowLeft /> К домену
            </Link>
            {summary?.domain.project_id && (
              <Link
                href={`/projects/${summary.domain.project_id}`}
                className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
              >
                <FiArrowLeft /> К проекту
              </Link>
            )}
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-800 dark:bg-red-950/30 dark:text-red-300">
          {error}
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-[300px_1fr]">
        <aside className="rounded-xl border border-slate-200 bg-white/80 p-3 shadow dark:border-slate-800 dark:bg-slate-900/60">
          <h2 className="mb-2 text-sm font-semibold">Файлы сайта</h2>
          <FileTree files={files} selectedPath={selection?.selectedPath} loading={fileLoading} onSelect={onSelectFile} />
        </aside>

        <section className="space-y-3">
          <EditorToolbar
            currentPath={selection?.selectedPath}
            dirty={dirtyState.isDirty}
            saving={saving}
            canSave={canSave}
            readOnly={readOnly}
            description={description}
            onDescriptionChange={setDescription}
            onSave={onSave}
            onRevert={onRevert}
            onDownload={onDownload}
          />

          {!selection && (
            <div className="rounded-xl border border-slate-200 bg-white/80 p-6 text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400">
              Выберите файл для просмотра.
            </div>
          )}

          {selection && currentFile && (!currentFile.editable || binaryPreview) && (
            <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
              <div className="font-semibold">Файл недоступен для редактирования в v1</div>
              <div className="mt-1">Тип: {selection.mimeType || currentFile.mimeType || "unknown"}</div>
              <div className="mt-1">Размер: {currentFile.size} bytes</div>
              <div className="mt-1">Можно скачать содержимое и изменить локально.</div>
            </div>
          )}

          {selection && currentFile && currentFile.editable && !binaryPreview && (
            <MonacoEditor
              content={dirtyState.currentContent}
              language={selection.language}
              readOnly={readOnly || !currentFile.editable}
              scrollLine={focusLine}
              onChange={(value) => {
                setDirtyState((prev) => ({
                  ...prev,
                  currentContent: value,
                  isDirty: value !== prev.originalContent
                }));
              }}
            />
          )}

          <FileHistory
            domainId={domainId}
            fileId={selection?.selectedFileId}
            refreshKey={historyRefreshKey}
          />

          {readOnly && (
            <div className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-semibold text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
              <FiEye /> Режим просмотра: сохранение отключено для роли viewer
            </div>
          )}
          {!readOnly && (
            <div className="inline-flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs font-semibold text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300">
              <FiEdit3 /> Редактирование доступно
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
