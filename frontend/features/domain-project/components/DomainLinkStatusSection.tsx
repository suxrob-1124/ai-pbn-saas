import { useMemo, useState } from "react";
import Link from "next/link";
import type { UrlObject } from "url";
import { FiAlertTriangle, FiCheck, FiClock, FiDownload, FiInfo, FiPlay, FiRefreshCw, FiTrash2 } from "react-icons/fi";
import { Badge } from "../../../components/Badge";
import { authFetch } from "../../../lib/http";
import { getLinkTaskSteps } from "../services/statusMeta";
import { DOMAIN_PROJECT_CTA } from "../services/statusCta";
import { normalizeLinkTaskStatus } from "../../../lib/linkTaskStatus";
import { LinkTaskStatusBadge } from "./DomainStatusBadges";

type DomainLike = {
  link_status?: string;
  link_status_effective?: string;
};

type LinkTask = {
  id: string;
  anchor_text: string;
  target_url: string;
  scheduled_for: string;
  action?: string;
  status: string;
  found_location?: string;
  generated_content?: string;
  error_message?: string;
  log_lines?: string[];
  attempts: number;
  created_at: string;
};

type LinkDiffEntry = { filePath: string; line: number; before: string; after: string };

type DomainLinkStatusSectionProps = {
  domainId: string;
  domain: DomainLike | null;
  linkTasks: LinkTask[];
  linkTasksLoading: boolean;
  linkTasksError: string | null;
  linkNotice: string | null;
  linkAnchor: string;
  linkAcceptor: string;
  linkInProgress: boolean;
  canRemoveLink: boolean;
  linkActionLabel: string;
  onRefreshLinkTasks: () => Promise<void>;
  onRunLinkTask: () => Promise<void>;
  onRemoveLinkTask: () => Promise<void>;
};

export function DomainLinkStatusSection({
  domainId,
  linkTasks,
  linkTasksLoading,
  linkTasksError,
  linkNotice,
  linkAnchor,
  linkAcceptor,
  linkInProgress,
  canRemoveLink,
  linkActionLabel,
  onRefreshLinkTasks,
  onRunLinkTask,
  onRemoveLinkTask
}: DomainLinkStatusSectionProps) {
  const [linkTab, setLinkTab] = useState<"summary" | "logs">("summary");
  const [showAllLinkTasks, setShowAllLinkTasks] = useState(false);
  const [linkDiffs, setLinkDiffs] = useState<Record<string, LinkDiffEntry>>({});
  const visibleLinkTasks = useMemo(() => (showAllLinkTasks ? linkTasks : linkTasks.slice(0, 2)), [linkTasks, showAllLinkTasks]);

  const buildFileUrl = (path: string) => {
    const safe = path
      .split("/")
      .map((part) => encodeURIComponent(part))
      .join("/");
    return `/api/domains/${domainId}/files/${safe}`;
  };

  const buildEditorUrl = (filePath: string, line?: number): UrlObject => {
    const query: Record<string, string> = { path: filePath };
    if (line && line > 0) {
      query.line = String(line);
    }
    return {
      pathname: `/domains/${domainId}/editor`,
      query
    };
  };

  const parseFoundLocation = (value?: string) => {
    if (!value) return null;
    const [filePathRaw, lineRaw] = value.split(":");
    const filePath = (filePathRaw || "").trim();
    const line = parseInt(lineRaw || "0", 10) || 1;
    if (!filePath) return null;
    return { filePath, line };
  };

  const computeSnippet = (lines: string[], lineIndex: number, padding = 2) => {
    const start = Math.max(0, lineIndex - padding);
    const end = Math.min(lines.length, lineIndex + padding + 1);
    return lines.slice(start, end).join("\n");
  };

  const stripAnchorTag = (text: string, anchor: string, target: string) => {
    if (!anchor || !target) return text;
    const escapedAnchor = anchor.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const escapedTarget = target.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const re = new RegExp(`<a[^>]*href=["']${escapedTarget}["'][^>]*>${escapedAnchor}</a>`, "gi");
    return text.replace(re, anchor);
  };

  const renderDiffLines = (before: string, after: string, mode: "before" | "after") => {
    const beforeLines = before.split("\n");
    const afterLines = after.split("\n");
    const max = Math.max(beforeLines.length, afterLines.length);
    const rows = [];
    for (let i = 0; i < max; i += 1) {
      const b = beforeLines[i] ?? "";
      const a = afterLines[i] ?? "";
      const changed = b !== a;
      const text = mode === "before" ? b : a;
      const cls = changed
        ? mode === "before"
          ? "bg-red-950/40 text-red-200"
          : "bg-emerald-950/40 text-emerald-200"
        : "";
      rows.push(
        <div key={`${mode}-${i}`} className={`whitespace-pre-wrap font-mono text-xs px-1 ${cls}`}>
          {text || "\u00A0"}
        </div>
      );
    }
    return rows;
  };

  const loadLinkDiff = async (task: LinkTask) => {
    if (!task.found_location || linkDiffs[task.id]) return;
    const [filePathRaw, lineRaw] = task.found_location.split(":");
    const filePath = (filePathRaw || "").trim();
    const line = parseInt(lineRaw || "0", 10) || 1;
    if (!filePath) return;
    try {
      const fileResp = await authFetch<{ content: string }>(buildFileUrl(filePath));
      const content = fileResp?.content ?? "";
      const lines = content.split("\n");
      const idx = Math.max(0, line - 1);
      const afterSnippet = computeSnippet(lines, idx, 2);
      const beforeSnippet = stripAnchorTag(afterSnippet, task.anchor_text, task.target_url);
      setLinkDiffs((prev) => ({
        ...prev,
        [task.id]: { filePath, line, before: beforeSnippet, after: afterSnippet }
      }));
    } catch {
      // Ошибка уже видна в текущем статусе задачи.
    }
  };

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h3 className="font-semibold">Добавление ссылок</h3>
          <p className="text-xs text-slate-500 dark:text-slate-400">
            Отслеживайте процесс и результат вставки ссылок в HTML.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex rounded-full border border-slate-200 dark:border-slate-700 bg-white/70 dark:bg-slate-800/70 p-1">
            <button
              onClick={() => setLinkTab("summary")}
              className={`px-3 py-1 text-xs font-semibold rounded-full ${linkTab === "summary" ? "bg-indigo-600 text-white" : "text-slate-600 dark:text-slate-200"}`}
            >
              Сводка
            </button>
            <button
              onClick={() => setLinkTab("logs")}
              className={`px-3 py-1 text-xs font-semibold rounded-full ${linkTab === "logs" ? "bg-indigo-600 text-white" : "text-slate-600 dark:text-slate-200"}`}
            >
              Логи ссылок
            </button>
          </div>
          <button
            onClick={onRefreshLinkTasks}
            disabled={linkTasksLoading}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiRefreshCw /> Обновить
          </button>
          <button
            onClick={onRunLinkTask}
            disabled={linkTasksLoading || linkInProgress || !linkAnchor.trim() || !linkAcceptor.trim()}
            className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
          >
            <FiPlay /> {linkActionLabel}
          </button>
          {canRemoveLink ? (
            <button
              onClick={onRemoveLinkTask}
              disabled={linkTasksLoading}
              className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
            >
              <FiTrash2 /> {DOMAIN_PROJECT_CTA.linkRemove}
            </button>
          ) : (
            <>
              {linkInProgress ? (
                <span className="hidden sm:inline-flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2 py-1 text-[11px] font-semibold text-amber-600 dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                  <FiRefreshCw className="h-3 w-3" /> Выполняется
                </span>
              ) : (
                <span className="hidden sm:inline-flex items-center gap-1 rounded-full border border-slate-200 bg-slate-50 px-2 py-1 text-[11px] font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
                  <FiInfo className="h-3 w-3" /> Нет ссылки
                </span>
              )}
            </>
          )}
        </div>
      </div>
      {linkNotice && <div className="text-sm text-emerald-500">{linkNotice}</div>}
      {linkTasksError && <div className="text-sm text-red-500">{linkTasksError}</div>}
      {linkTab === "summary" && (
        <>
          <div className="grid gap-3 md:grid-cols-2">
            <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-3 space-y-2 bg-slate-50/60 dark:bg-slate-900/60">
              <div className="text-xs uppercase tracking-wide text-slate-400">Текущие настройки</div>
              <div className="text-sm text-slate-700 dark:text-slate-200">
                Анкор: <span className="font-semibold">{linkAnchor || "—"}</span>
              </div>
              <div className="text-sm text-slate-700 dark:text-slate-200">
                Акцептор: <span className="font-semibold">{linkAcceptor || "—"}</span>
              </div>
            </div>
            <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-3 space-y-2 bg-slate-50/60 dark:bg-slate-900/60">
              <div className="text-xs uppercase tracking-wide text-slate-400">Последняя задача</div>
              {linkTasks[0] ? (
                <>
                  <LinkTaskStatusBadge status={linkTasks[0].status} />
                  <div className="text-xs text-slate-500 dark:text-slate-400">
                    Создано: {new Date(linkTasks[0].created_at).toLocaleString()}
                  </div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">
                    Запланировано: {new Date(linkTasks[0].scheduled_for).toLocaleString()}
                  </div>
                  {linkTasks[0].found_location && (
                    <div className="text-xs text-slate-500 dark:text-slate-400">
                      Найдено: {linkTasks[0].found_location}
                    </div>
                  )}
                  {linkTasks[0].error_message && (
                    <div className="text-xs text-red-500">Ошибка: {linkTasks[0].error_message}</div>
                  )}
                </>
              ) : (
                <div className="text-sm text-slate-500 dark:text-slate-400">Задач ещё нет</div>
              )}
            </div>
          </div>
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="text-xs uppercase tracking-wide text-slate-400">История задач</div>
              <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                <span>Всего: {linkTasks.length}</span>
                {linkTasks.length > 2 && (
                  <button
                    onClick={() => setShowAllLinkTasks((v) => !v)}
                    className="text-indigo-600 hover:underline"
                  >
                    {showAllLinkTasks ? "Скрыть" : "Показать все"}
                  </button>
                )}
              </div>
            </div>
            {linkTasks.length === 0 ? (
              <div className="text-sm text-slate-500 dark:text-slate-400">Нет данных по задачам.</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                      <th className="py-2 pr-4">№</th>
                      <th className="py-2 pr-4">Статус</th>
                      <th className="py-2 pr-4">Запланировано</th>
                      <th className="py-2 pr-4">Попытки</th>
                      <th className="py-2 pr-4">Результат</th>
                      <th className="py-2 pr-4">Детали</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                    {visibleLinkTasks.map((task, idx) => (
                      <tr key={task.id}>
                        <td className="py-2 pr-4 text-xs text-slate-500 dark:text-slate-400">{idx + 1}</td>
                        <td className="py-2 pr-4">
                          <LinkTaskStatusBadge status={task.status} />
                        </td>
                        <td className="py-2 pr-4 text-slate-500 dark:text-slate-400">
                          {new Date(task.scheduled_for).toLocaleString()}
                        </td>
                        <td className="py-2 pr-4 text-slate-500 dark:text-slate-400">{task.attempts}</td>
                        <td className="py-2 pr-4 text-slate-500 dark:text-slate-400">
                          {task.error_message && <span className="text-red-500">Ошибка</span>}
                          {!task.error_message && task.found_location && <span>Вставлено</span>}
                          {!task.error_message && !task.found_location && task.generated_content && <span>Вставлено (ген. текст)</span>}
                          {!task.error_message && !task.found_location && !task.generated_content && <span>—</span>}
                        </td>
                        <td className="py-2 pr-4">
                          <Link href={{ pathname: `/links/${task.id}` }} className="text-indigo-600 hover:underline">
                            Открыть
                          </Link>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {linkTasks[0]?.generated_content && (
              <details className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/60 p-3">
                <summary className="cursor-pointer text-sm font-semibold">Показать сгенерированный текст</summary>
                <pre className="mt-2 whitespace-pre-wrap text-xs text-slate-600 dark:text-slate-300">
                  {linkTasks[0].generated_content}
                </pre>
              </details>
            )}
          </div>
        </>
      )}
      {linkTab === "logs" && (
        <div className="space-y-4">
          <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
            <span>Всего: {linkTasks.length}</span>
            {linkTasks.length > 2 && (
              <button
                onClick={() => setShowAllLinkTasks((v) => !v)}
                className="text-indigo-600 hover:underline"
              >
                {showAllLinkTasks ? "Скрыть" : "Показать все"}
              </button>
            )}
          </div>
          {linkTasks.length === 0 ? (
            <div className="text-sm text-slate-500 dark:text-slate-400">Нет задач для отображения.</div>
          ) : (
            visibleLinkTasks.map((task, idx) => {
              const isRemove = (task.action || "insert") === "remove";
              const foundMeta = parseFoundLocation(task.found_location);
              const steps = getLinkTaskSteps(task.action);
              const reached = new Set<string>();
              const normalizedTaskStatus = normalizeLinkTaskStatus(task.status) || task.status;
              if (["pending", "searching", "inserted", "generated", "removing", "removed", "failed"].includes(normalizedTaskStatus)) {
                reached.add("pending");
              }
              if (["searching", "inserted", "generated", "removing", "removed"].includes(normalizedTaskStatus)) reached.add("searching");
              if (!isRemove) {
                if (["inserted"].includes(normalizedTaskStatus)) reached.add("inserted");
                if (["generated"].includes(normalizedTaskStatus)) reached.add("generated");
              } else {
                if (["removing", "removed"].includes(normalizedTaskStatus)) reached.add("removing");
                if (["removed"].includes(normalizedTaskStatus)) reached.add("removed");
              }
              return (
                <div key={task.id} className="rounded-lg border border-slate-200 dark:border-slate-800 p-3 bg-slate-50/60 dark:bg-slate-900/60 space-y-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="text-sm font-semibold">Задача №{idx + 1}</div>
                    <LinkTaskStatusBadge status={task.status} />
                  </div>
                  <div className="grid gap-2 md:grid-cols-2 text-xs text-slate-500 dark:text-slate-400">
                    <div>Создано: {new Date(task.created_at).toLocaleString()}</div>
                    <div>Запланировано: {new Date(task.scheduled_for).toLocaleString()}</div>
                    <div>Анкор: {task.anchor_text}</div>
                    <div>Акцептор: {task.target_url}</div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {steps.map((step) => (
                      <Badge
                        key={step.id}
                        label={step.label}
                        tone={reached.has(step.id) ? "emerald" : "slate"}
                        icon={reached.has(step.id) ? <FiCheck /> : <FiClock />}
                        className="text-xs"
                      />
                    ))}
                    {task.status === "failed" && (
                      <Badge label="Ошибка" tone="red" icon={<FiAlertTriangle />} className="text-xs" />
                    )}
                  </div>
                  {task.error_message && (
                    <div className="text-xs text-red-500">Ошибка: {task.error_message}</div>
                  )}
                  {task.found_location && (
                    <div className="space-y-2">
                      <div className="text-xs text-slate-500 dark:text-slate-400">
                        Найдено: {task.found_location}
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <button
                          onClick={() => loadLinkDiff(task)}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        >
                          Показать diff
                        </button>
                        {linkDiffs[task.id]?.filePath && (
                          <a
                            href={buildFileUrl(linkDiffs[task.id].filePath)}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                          >
                            <FiDownload className="h-3 w-3" /> Открыть файл
                          </a>
                        )}
                        {foundMeta && (
                          <Link
                            href={buildEditorUrl(foundMeta.filePath, foundMeta.line)}
                            className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                          >
                            Открыть в редакторе
                          </Link>
                        )}
                      </div>
                      {linkDiffs[task.id] && (
                        <div className="grid gap-2 md:grid-cols-2 text-xs">
                          <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-950/60 p-2 text-slate-300">
                            <div className="text-[11px] uppercase text-slate-500 mb-1">До</div>
                            <div>{renderDiffLines(linkDiffs[task.id].before, linkDiffs[task.id].after, "before")}</div>
                          </div>
                          <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-950/60 p-2 text-slate-300">
                            <div className="text-[11px] uppercase text-slate-500 mb-1">После</div>
                            <div>{renderDiffLines(linkDiffs[task.id].before, linkDiffs[task.id].after, "after")}</div>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                  {task.generated_content && (
                    <details className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/60 p-3">
                      <summary className="cursor-pointer text-sm font-semibold">Сгенерированный текст</summary>
                      <pre className="mt-2 whitespace-pre-wrap text-xs text-slate-600 dark:text-slate-300">
                        {task.generated_content}
                      </pre>
                    </details>
                  )}
                  {task.log_lines && task.log_lines.length > 0 && (
                    <details className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/60 p-3">
                      <summary className="cursor-pointer text-sm font-semibold">Логи воркера</summary>
                      <pre className="mt-2 whitespace-pre-wrap text-xs text-slate-600 dark:text-slate-300">
                        {task.log_lines.join("\n")}
                      </pre>
                    </details>
                  )}
                </div>
              );
            })
          )}
        </div>
      )}
    </div>
  );
}
