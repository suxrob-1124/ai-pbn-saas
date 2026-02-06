"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { authFetch, patch, del } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import { FiClock, FiPlay, FiCheck, FiAlertTriangle, FiRefreshCw, FiTrash2, FiPause, FiX } from "react-icons/fi";
import { ArtifactsViewer, LogsViewer } from "../../../components/ArtifactsViewer";
import PipelineSteps from "../../../components/PipelineSteps";
import { computeDisplayProgress } from "../../../lib/pipelineProgress";
import { AuditReport } from "../../../components/AuditReport";

type Domain = {
  id: string;
  project_id: string;
  server_id?: string;
  url: string;
  main_keyword?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  status: string;
  last_generation_id?: string;
  updated_at?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
};

type Generation = {
  id: string;
  status: string;
  progress: number;
  created_at?: string;
  updated_at?: string;
  logs?: any;
  artifacts?: Record<string, any>;
};

type LinkTask = {
  id: string;
  domain_id: string;
  anchor_text: string;
  target_url: string;
  scheduled_for: string;
  status: string;
  found_location?: string;
  generated_content?: string;
  error_message?: string;
  log_lines?: string[];
  attempts: number;
  created_by: string;
  created_at: string;
  completed_at?: string;
};

export default function DomainPage() {
  useAuthGuard();
  const params = useParams();
  const id = params?.id as string;
  const [domain, setDomain] = useState<Domain | null>(null);
  const [gens, setGens] = useState<Generation[]>([]);
  const [projectName, setProjectName] = useState<string>("");
  const [kw, setKw] = useState("");
  const [country, setCountry] = useState("");
  const [language, setLanguage] = useState("");
  const [exclude, setExclude] = useState("");
  const [server, setServer] = useState("");
  const [loading, setLoading] = useState(false);
  const [visibleGens, setVisibleGens] = useState(2);
  const [error, setError] = useState<string | null>(null);
  const [pipelineStepInFlight, setPipelineStepInFlight] = useState<string | null>(null);
  const [linkAnchor, setLinkAnchor] = useState("");
  const [linkAcceptor, setLinkAcceptor] = useState("");
  const [linkTasks, setLinkTasks] = useState<LinkTask[]>([]);
  const [linkTasksLoading, setLinkTasksLoading] = useState(false);
  const [linkTasksError, setLinkTasksError] = useState<string | null>(null);
  const [linkNotice, setLinkNotice] = useState<string | null>(null);
  const [linkTab, setLinkTab] = useState<"summary" | "logs">("summary");
  const [linkDiffs, setLinkDiffs] = useState<Record<string, { filePath: string; line: number; before: string; after: string }>>({});

  const load = async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const d = await authFetch<Domain>(`/api/domains/${id}`);
      setDomain(d);
      setKw(d?.main_keyword || "");
      setCountry(d?.target_country || "");
      setLanguage(d?.target_language || "");
      setExclude(d?.exclude_domains || "");
      setServer(d?.server_id || "");
      setLinkAnchor(d?.link_anchor_text || "");
      setLinkAcceptor(d?.link_acceptor_url || "");
      if (d?.project_id) {
        try {
          const p = await authFetch<any>(`/api/projects/${d.project_id}`);
          setProjectName(p?.name || "");
        } catch {
          /* ignore */
        }
      }
      try {
        const list = await authFetch<Generation[]>(`/api/domains/${id}/generations`);
        setGens(Array.isArray(list) ? list : []);
      } catch {
        setGens([]);
      }
      try {
        const tasks = await authFetch<LinkTask[]>(`/api/domains/${id}/links`);
        const list = Array.isArray(tasks) ? tasks : [];
        list.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
        setLinkTasks(list);
      } catch (err: any) {
        setLinkTasks([]);
        setLinkTasksError(err?.message || "Не удалось загрузить задачи ссылок");
      }
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить домен");
    } finally {
      setLoading(false);
    }
  };

  const save = async () => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/domains/${id}`, {
        keyword: kw,
        country,
        language,
        exclude_domains: exclude,
        server_id: server,
        link_anchor_text: linkAnchor,
        link_acceptor_url: linkAcceptor
      });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось сохранить");
    } finally {
      setLoading(false);
    }
  };

  const runLinkTask = async () => {
    if (!id) return;
    setLinkTasksError(null);
    setLinkNotice(null);
    if (!linkAnchor.trim() || !linkAcceptor.trim()) {
      setLinkTasksError("Заполните анкор и акцептор");
      return;
    }
    setLinkTasksLoading(true);
    try {
      await authFetch(`/api/domains/${id}/link/run`, { method: "POST" });
      setLinkNotice("Запуск добавления ссылки инициирован");
      await load();
    } catch (err: any) {
      setLinkTasksError(err?.message || "Не удалось запустить добавление ссылки");
    } finally {
      setLinkTasksLoading(false);
    }
  };

  const refreshLinkTasks = async () => {
    if (!id) return;
    setLinkTasksLoading(true);
    setLinkTasksError(null);
    try {
      const tasks = await authFetch<LinkTask[]>(`/api/domains/${id}/links`);
      const list = Array.isArray(tasks) ? tasks : [];
      list.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      setLinkTasks(list);
    } catch (err: any) {
      setLinkTasksError(err?.message || "Не удалось загрузить задачи ссылок");
    } finally {
      setLinkTasksLoading(false);
    }
  };

  const triggerGeneration = async (forceStep?: string) => {
    if (!id) return;
    if (!kw.trim()) {
      setError("Сначала задайте ключевое слово");
      return;
    }
    const latestGen = gens[0];
    if (!forceStep && latestGen?.status === "success") {
      if (!confirm("Генерация уже завершена. Запустить заново?")) {
        return;
      }
    }
    setError(null);
    setLoading(true);
    setGens((prev) => {
      if (prev.length === 0) {
        return prev;
      }
      const updated = [...prev];
      updated[0] = { ...updated[0], status: "processing", progress: 0 };
      return updated;
    });
    try {
      const payload = forceStep ? { force_step: forceStep } : undefined;
      const headers = payload ? { "Content-Type": "application/json" } : undefined;
      await authFetch(`/api/domains/${id}/generate`, {
        method: "POST",
        headers,
        body: payload ? JSON.stringify(payload) : undefined
      });
      await load();
    } catch (err: any) {
      const msg = err?.message || "Не удалось запустить генерацию";
      setError(msg);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  const handleMainAction = async () => {
    try {
      await triggerGeneration();
    } catch {
      // Ошибка уже показана
    }
  };

  const handleForceStep = async (stepId: string) => {
    setPipelineStepInFlight(stepId);
    try {
      await triggerGeneration(stepId);
    } catch {
      // Ошибка обработана внутри triggerGeneration
    } finally {
      setPipelineStepInFlight(null);
    }
  };

  const deleteGeneration = async (genId: string) => {
    if (!confirm("Удалить этот запуск?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/generations/${genId}`);
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить запуск");
    } finally {
      setLoading(false);
    }
  };

  const pauseGeneration = async (genId: string) => {
    if (!confirm("Приостановить выполнение задачи?")) return;
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/generations/${genId}`, { action: "pause" });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось приостановить задачу");
    } finally {
      setLoading(false);
    }
  };

  const resumeGeneration = async (genId: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/generations/${genId}`, { action: "resume" });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось возобновить задачу");
    } finally {
      setLoading(false);
    }
  };

  const cancelGeneration = async (genId: string) => {
    if (!confirm("Отменить выполнение задачи?")) return;
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/generations/${genId}`, { action: "cancel" });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось отменить задачу");
    } finally {
      setLoading(false);
    }
  };

  const activeStatusesList = ["pending", "processing", "pause_requested", "cancelling"];
  const latestGen = gens[0];
  const latestDisplayProgress = latestGen ? computeDisplayProgress(latestGen.artifacts, latestGen.progress, latestGen.status) : 0;
  const isRegenerate = Boolean(latestGen && latestGen.status === "success");
  const mainButtonText = !latestGen ? "Запустить генерацию" : isRegenerate ? "Перегенерировать всё" : "Продолжить генерацию";
  const mainButtonIcon = isRegenerate ? <FiRefreshCw /> : <FiPlay />;
  const mainButtonDisabled = loading || Boolean(latestGen && activeStatusesList.includes(latestGen.status));

  useEffect(() => {
    load();
  }, [id]);

  const buildFileUrl = (path: string) => {
    const safe = path
      .split("/")
      .map((part) => encodeURIComponent(part))
      .join("/");
    return `/api/domains/${id}/files/${safe}`;
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
    if (!task.found_location) return;
    if (linkDiffs[task.id]) return;
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
    } catch (err: any) {
      setLinkTasksError(err?.message || "Не удалось загрузить файл для diff");
    }
  };

  useEffect(() => {
    // каждый раз когда перезагружаем список генераций — сбрасываем пагинацию
    setVisibleGens(2);
  }, [gens.length]);

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold">Домен</h1>
            {domain && (
              <>
                <div className="mt-1 text-lg font-semibold">{domain.url}</div>
                <div className="text-sm text-slate-500 dark:text-slate-400">
                  Проект: {projectName || domain.project_id} · Статус: <StatusBadge status={domain.status} />
                </div>
                <div className="text-sm text-slate-500 dark:text-slate-400 mt-1">Ключевое слово: {domain.main_keyword || "—"}</div>
                <div className="text-xs text-slate-500 dark:text-slate-400">
                  Сервер: {domain.server_id || "—"} · Страна: {domain.target_country || "—"} · Язык: {domain.target_language || "—"}
                </div>
                {domain.exclude_domains && <div className="text-xs text-slate-500 dark:text-slate-400">Исключить: {domain.exclude_domains}</div>}
                <div className="text-xs text-slate-500 dark:text-slate-400">
                  Обновлено: {domain.updated_at ? new Date(domain.updated_at).toLocaleString() : "—"}
                </div>
              </>
            )}
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleMainAction}
              disabled={mainButtonDisabled}
              className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
            >
              {mainButtonIcon} {mainButtonText}
            </button>
            {gens.length > 0 && gens[0] && (
              <>
                {gens[0].status === "paused" && (
                  <button
                    onClick={() => resumeGeneration(gens[0].id)}
                    disabled={loading}
                    className="inline-flex items-center gap-2 rounded-lg border border-emerald-200 bg-white px-3 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:bg-slate-800 dark:text-emerald-300 disabled:opacity-50"
                  >
                    <FiPlay /> Возобновить
                  </button>
                )}
                {(gens[0].status === "pending" || gens[0].status === "processing" || gens[0].status === "pause_requested" || gens[0].status === "cancelling") && (
                  <>
                    {gens[0].status !== "cancelling" && (
                      <button
                        onClick={() => pauseGeneration(gens[0].id)}
                        disabled={loading || gens[0].status === "pause_requested"}
                        className="inline-flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-700 dark:bg-slate-800 dark:text-amber-300 disabled:opacity-50"
                      >
                        <FiPause /> {gens[0].status === "pause_requested" ? "Пауза запрошена..." : "Пауза"}
                      </button>
                    )}
                    <button
                      onClick={() => cancelGeneration(gens[0].id)}
                      disabled={loading || gens[0].status === "cancelling"}
                      className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 disabled:opacity-50"
                    >
                      <FiX /> {gens[0].status === "cancelling" ? "Отмена..." : "Отменить"}
                    </button>
                  </>
                )}
                {gens[0].status === "cancelled" && (
                  <button
                    onClick={() => cancelGeneration(gens[0].id)}
                    disabled={loading}
                    className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 disabled:opacity-50"
                  >
                    <FiX /> Отменить
                  </button>
                )}
              </>
            )}
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            {domain?.project_id ? (
              <Link
                href={`/projects/${domain.project_id}`}
                className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
              >
                ← К проекту
              </Link>
            ) : (
              <button
                disabled
                className="inline-flex items-center gap-2 rounded-lg bg-slate-300 px-3 py-2 text-sm font-semibold text-slate-600 cursor-not-allowed"
              >
                ← К проекту
              </button>
            )}
          </div>
        </div>
        {error && <div className="mt-2 text-red-500 text-sm">{error}</div>}
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <h3 className="font-semibold mb-3">Этапы генерации</h3>
        <PipelineSteps
          domainId={id}
          generation={gens[0]}
          disabled={loading}
          activeStep={pipelineStepInFlight}
          onForceStep={handleForceStep}
        />
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <h3 className="font-semibold">Настройки домена</h3>
        <div className="grid gap-3 md:grid-cols-2">
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Ключевое слово</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={kw}
              onChange={(e) => setKw(e.target.value)}
              placeholder="casino ..."
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Анкор</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={linkAnchor}
              onChange={(e) => setLinkAnchor(e.target.value)}
              placeholder="Текст ссылки"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Акцептор</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={linkAcceptor}
              onChange={(e) => setLinkAcceptor(e.target.value)}
              placeholder="https://example.com"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Сервер</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={server}
              onChange={(e) => setServer(e.target.value)}
              placeholder="seotech-web-media1"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Страна</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={country}
              onChange={(e) => setCountry(e.target.value)}
              placeholder="se"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Язык</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
              placeholder="sv-SE"
            />
          </label>
          <label className="text-sm space-y-1 md:col-span-2">
            <span className="text-slate-600 dark:text-slate-300">Исключить домены (через запятую)</span>
            <textarea
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              rows={2}
              value={exclude}
              onChange={(e) => setExclude(e.target.value)}
              placeholder="https://example.com, https://www.foo.bar"
            />
          </label>
        </div>
        <button
          onClick={save}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
        >
          Сохранить
        </button>
      </div>

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
              onClick={refreshLinkTasks}
              disabled={linkTasksLoading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            <button
              onClick={runLinkTask}
              disabled={linkTasksLoading || !linkAnchor.trim() || !linkAcceptor.trim()}
              className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
            >
              <FiPlay /> Запустить добавление
            </button>
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
                <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {linkTasks.length}</span>
              </div>
              {linkTasks.length === 0 ? (
                <div className="text-sm text-slate-500 dark:text-slate-400">Нет данных по задачам.</div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                        <th className="py-2 pr-4">ID</th>
                        <th className="py-2 pr-4">Статус</th>
                        <th className="py-2 pr-4">Запланировано</th>
                        <th className="py-2 pr-4">Попытки</th>
                        <th className="py-2 pr-4">Результат</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                      {linkTasks.slice(0, 6).map((task) => (
                        <tr key={task.id}>
                          <td className="py-2 pr-4 font-mono text-xs">{task.id.slice(0, 8)}</td>
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
                            {!task.error_message && !task.found_location && task.generated_content && <span>Сгенерировано</span>}
                            {!task.error_message && !task.found_location && !task.generated_content && <span>—</span>}
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
            {linkTasks.length === 0 ? (
              <div className="text-sm text-slate-500 dark:text-slate-400">Нет задач для отображения.</div>
            ) : (
              linkTasks.slice(0, 6).map((task) => {
                const steps = [
                  { id: "pending", label: "В очереди" },
                  { id: "searching", label: "Поиск места" },
                  { id: "found", label: "Место найдено" },
                  { id: "inserted", label: "Вставка ссылки" },
                  { id: "generated", label: "Генерация текста" }
                ];
                const reached = new Set<string>();
                if (["pending", "searching", "found", "inserted", "generated", "failed"].includes(task.status)) {
                  reached.add("pending");
                }
                if (["searching", "found", "inserted", "generated"].includes(task.status)) reached.add("searching");
                if (["found", "inserted", "generated"].includes(task.status)) reached.add("found");
                if (["inserted"].includes(task.status)) reached.add("inserted");
                if (["generated"].includes(task.status)) reached.add("generated");
                return (
                  <div key={task.id} className="rounded-lg border border-slate-200 dark:border-slate-800 p-3 bg-slate-50/60 dark:bg-slate-900/60 space-y-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="text-sm font-semibold">
                        Задача {task.id.slice(0, 8)}
                      </div>
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
                        <span
                          key={step.id}
                          className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${
                            reached.has(step.id)
                              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
                              : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                          }`}
                        >
                          {reached.has(step.id) ? <FiCheck /> : <FiClock />} {step.label}
                        </span>
                      ))}
                      {task.status === "failed" && (
                        <span className="inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200">
                          <FiAlertTriangle /> Ошибка
                        </span>
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
                              Открыть файл
                            </a>
                          )}
                          {linkDiffs[task.id]?.filePath && (
                            <button
                              disabled
                              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white/60 px-2 py-1 text-xs font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-400 cursor-not-allowed"
                              title="Маршрут редактора появится позже"
                            >
                              Открыть в редакторе
                            </button>
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

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Запуски</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {gens.length}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                <th className="py-2 pr-4">ID</th>
                <th className="py-2 pr-4">Статус</th>
                <th className="py-2 pr-4">Прогресс</th>
                <th className="py-2 pr-4">Обновлено</th>
                <th className="py-2 pr-4">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
              {gens.slice(0, visibleGens).map((g) => {
                const rowProgress = computeDisplayProgress(g.artifacts, g.progress, g.status);
                return (
                <tr key={g.id}>
                  <td className="py-3 pr-4 font-mono text-xs">{g.id.slice(0, 8)}</td>
                  <td className="py-3 pr-4">
                    <StatusBadge status={g.status} />
                  </td>
                  <td className="py-3 pr-4">{rowProgress}%</td>
                  <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">{g.updated_at ? new Date(g.updated_at).toLocaleString() : "—"}</td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-3">
                    <Link href={`/queue/${g.id}`} target="_blank" className="text-indigo-600 hover:underline">
                      Открыть
                    </Link>
                      {g.status === "paused" && (
                        <button
                          onClick={() => resumeGeneration(g.id)}
                          disabled={loading}
                          className="text-emerald-500 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 disabled:opacity-50"
                          title="Возобновить"
                        >
                          <FiPlay className="w-4 h-4" />
                        </button>
                      )}
                      {(g.status === "pending" || g.status === "processing" || g.status === "pause_requested" || g.status === "cancelling") && (
                        <>
                          {g.status !== "cancelling" && (
                            <button
                              onClick={() => pauseGeneration(g.id)}
                              disabled={loading || g.status === "pause_requested"}
                              className="text-amber-500 hover:text-amber-700 dark:text-amber-400 dark:hover:text-amber-300 disabled:opacity-50"
                              title={g.status === "pause_requested" ? "Пауза запрошена" : "Пауза"}
                            >
                              <FiPause className="w-4 h-4" />
                            </button>
                          )}
                          <button
                            onClick={() => cancelGeneration(g.id)}
                            disabled={loading || g.status === "cancelling"}
                            className="text-orange-500 hover:text-orange-700 dark:text-orange-400 dark:hover:text-orange-300 disabled:opacity-50"
                            title={g.status === "cancelling" ? "Отмена..." : "Отменить"}
                          >
                            <FiX className="w-4 h-4" />
                          </button>
                        </>
                      )}
                      {g.status === "cancelled" && (
                        <button
                          onClick={() => cancelGeneration(g.id)}
                          disabled={loading}
                          className="text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
                          title="Отменить"
                        >
                          <FiX className="w-4 h-4" />
                        </button>
                      )}
                      <button
                        onClick={() => deleteGeneration(g.id)}
                        disabled={loading}
                        className="text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
                        title="Удалить"
                      >
                        <FiTrash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              )})}
              {gens.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-4 text-center text-slate-500 dark:text-slate-400">
                    Запусков пока нет.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        {gens.length > visibleGens && (
          <div className="pt-2">
            <button
              onClick={() => setVisibleGens((v) => Math.min(gens.length, v + 3))}
              className="text-sm text-indigo-600 hover:underline"
            >
              Показать ещё
            </button>
          </div>
        )}
      </div>

      {gens.length > 0 && (
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-semibold">Последний запуск</h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                ID: {gens[0].id} · Статус: <StatusBadge status={gens[0].status} /> · Прогресс: {latestDisplayProgress}%
              </p>
            </div>
            <Link href={`/queue/${gens[0].id}`} className="text-sm text-indigo-600 hover:underline">
              Открыть карточку запуска
            </Link>
          </div>
          <AuditReport report={gens[0].artifacts?.audit_report} />
          <LogsViewer logs={gens[0].logs} />
          <ArtifactsViewer artifacts={gens[0].artifacts} />
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; color: string; icon: React.ReactNode }> = {
    pending: { text: "В очереди", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiClock /> },
    processing: { text: "В работе", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPlay /> },
    pause_requested: { text: "Пауза запрошена", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-200", icon: <FiPause /> },
    paused: { text: "Приостановлено", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPause /> },
    cancelling: { text: "Отмена...", color: "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-200", icon: <FiX /> },
    cancelled: { text: "Отменено", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiX /> },
    success: { text: "Готово", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiCheck /> },
    error: { text: "Ошибка", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiAlertTriangle /> },
  };
  const cfg = map[status] || { text: status, color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiClock /> };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${cfg.color}`}>
      {cfg.icon} {cfg.text}
    </span>
  );
}

function LinkTaskStatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; color: string; icon: React.ReactNode }> = {
    pending: { text: "Ожидает", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiClock /> },
    searching: { text: "Поиск", color: "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200", icon: <FiRefreshCw /> },
    found: { text: "Найдено", color: "bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200", icon: <FiCheck /> },
    inserted: { text: "Вставлено", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiCheck /> },
    generated: { text: "Сгенерировано", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-200", icon: <FiCheck /> },
    failed: { text: "Ошибка", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiAlertTriangle /> }
  };
  const cfg = map[status] || { text: status, color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiClock /> };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${cfg.color}`}>
      {cfg.icon} {cfg.text}
    </span>
  );
}
