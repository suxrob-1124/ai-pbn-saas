"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import type { UrlObject } from "url";
import { apiBase, authFetch, authFetchCached, patch } from "../../../lib/http";
import { showToast } from "../../../lib/toastStore";
import { useAuthGuard } from "../../../lib/useAuth";
import { FiClock, FiPlay, FiCheck, FiAlertTriangle, FiRefreshCw, FiTrash2, FiPause, FiX, FiInfo, FiEdit3, FiCode, FiDownload } from "react-icons/fi";
import { PromptOverridesPanel } from "../../../components/PromptOverridesPanel";
import PipelineSteps from "../../../components/PipelineSteps";
import { computeDisplayProgress } from "../../../lib/pipelineProgress";
import { getLinkTaskStatusMeta, normalizeLinkTaskStatus } from "../../../lib/linkTaskStatus";
import { Badge } from "../../../components/Badge";
import { DOMAIN_PROJECT_CTA, getGenerationStatusMeta, getLinkActionLabel } from "../../../features/domain-project/services/statusCta";
import { canEditPromptOverrides, canOpenDomainEditor, isMainGenerationActionDisabled } from "../../../features/domain-project/services/actionGuards";
import { deriveDomainLinkActionMeta, deriveMainGenerationMeta, getLinkTaskSteps } from "../../../features/domain-project/services/statusMeta";
import { useDomainAsyncActions } from "../../../features/domain-project/hooks/useDomainAsyncActions";
import { ActionFlowBanner } from "../../../features/domain-project/components/ActionFlowBanner";
import { DomainHeaderActionsSection } from "../../../features/domain-project/components/DomainHeaderActionsSection";
import { DomainGenerationStatusSection } from "../../../features/domain-project/components/DomainGenerationStatusSection";
import { DomainLogsSection } from "../../../features/domain-project/components/DomainLogsSection";

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
  last_attempt_generation_id?: string;
  last_success_generation_id?: string;
  published_at?: string;
  published_path?: string;
  file_count?: number;
  total_size_bytes?: number;
  deployment_mode?: string;
  updated_at?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_status?: string;
  link_status_effective?: string;
  link_status_source?: "domain" | "active_task";
};

type Generation = {
  id: string;
  domain_id?: string;
  status: string;
  progress: number;
  error?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
  logs?: any;
  artifacts?: Record<string, any>;
  artifacts_summary?: Record<string, any>;
};

type LinkTask = {
  id: string;
  domain_id: string;
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
  created_by: string;
  created_at: string;
  completed_at?: string;
};

type DomainSummary = {
  domain: Domain;
  project_name: string;
  generations: Generation[];
  latest_attempt?: Generation;
  latest_success?: Generation;
  link_tasks: LinkTask[];
  my_role?: "admin" | "owner" | "editor" | "viewer";
};

type GenerationDetail = {
  id: string;
  logs?: any;
  artifacts?: Record<string, any>;
  artifacts_summary?: Record<string, any>;
};

type DeploymentAttempt = {
  id: string;
  domain_id: string;
  generation_id: string;
  mode: string;
  target_path: string;
  owner_before?: string;
  owner_after?: string;
  status: string;
  error_message?: string;
  file_count: number;
  total_size_bytes: number;
  created_at: string;
  finished_at?: string;
};

export default function DomainPage() {
  useAuthGuard();
  const params = useParams();
  const id = params?.id as string;
  const [domain, setDomain] = useState<Domain | null>(null);
  const [gens, setGens] = useState<Generation[]>([]);
  const [latestAttempt, setLatestAttempt] = useState<Generation | null>(null);
  const [latestSuccess, setLatestSuccess] = useState<Generation | null>(null);
  const [generationDetails, setGenerationDetails] = useState<Record<string, GenerationDetail>>({});
  const [myRole, setMyRole] = useState<"admin" | "owner" | "editor" | "viewer">("viewer");
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
  const [showAllLinkTasks, setShowAllLinkTasks] = useState(false);
  const [showResultHTMLModal, setShowResultHTMLModal] = useState(false);
  const [resultHTMLTab, setResultHTMLTab] = useState<"preview" | "code">("preview");
  const [liveResultHTML, setLiveResultHTML] = useState("");
  const [liveResultCode, setLiveResultCode] = useState("");
  const [liveResultLoading, setLiveResultLoading] = useState(false);
  const [liveResultError, setLiveResultError] = useState<string | null>(null);
  const [deployments, setDeployments] = useState<DeploymentAttempt[]>([]);
  const [showDomainPromptOverrides, setShowDomainPromptOverrides] = useState(false);

  const load = async (force = false) => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const summary = await authFetchCached<DomainSummary>(`/api/domains/${id}/summary?gen_limit=10&link_limit=20`, undefined, {
        ttlMs: 15000,
        bypassCache: force
      });
      const d = summary?.domain || null;
      setDomain(d);
      setProjectName(summary?.project_name || "");
      setMyRole(summary?.my_role || "viewer");
      setKw(d?.main_keyword || "");
      setCountry(d?.target_country || "");
      setLanguage(d?.target_language || "");
      setExclude(d?.exclude_domains || "");
      setServer(d?.server_id || "");
      setLinkAnchor(d?.link_anchor_text || "");
      setLinkAcceptor(d?.link_acceptor_url || "");
      const list = Array.isArray(summary?.generations) ? summary.generations : [];
      setGens(list);
      setLatestAttempt(summary?.latest_attempt || list[0] || null);
      setLatestSuccess(summary?.latest_success || null);
      const tasks = Array.isArray(summary?.link_tasks) ? summary.link_tasks : [];
      tasks.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      setLinkTasks(tasks);
      setLinkTasksError(null);
      void loadDeployments();
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить домен");
    } finally {
      setLoading(false);
    }
  };

  const loadDeployments = async () => {
    if (!id) return;
    try {
      const list = await authFetch<DeploymentAttempt[]>(`/api/domains/${id}/deployments?limit=10`);
      setDeployments(Array.isArray(list) ? list : []);
    } catch {
      setDeployments([]);
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
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось сохранить");
    } finally {
      setLoading(false);
    }
  };

  const { currentAttempt, isRegenerate, mainButtonText } = deriveMainGenerationMeta(latestAttempt, gens);
  const latestAttemptDetail = currentAttempt?.id ? generationDetails[currentAttempt.id] : undefined;
  const latestSuccessDetail = latestSuccess?.id ? generationDetails[latestSuccess.id] : undefined;
  const latestDisplayProgress = currentAttempt ? computeDisplayProgress(latestAttemptDetail?.artifacts, currentAttempt.progress, currentAttempt.status) : 0;
  const mainButtonIcon = isRegenerate ? <FiRefreshCw /> : <FiPlay />;
  const mainButtonDisabled = isMainGenerationActionDisabled(loading, currentAttempt?.status);
  const visibleLinkTasks = showAllLinkTasks ? linkTasks : linkTasks.slice(0, 2);
  const { hasActiveLink, linkInProgress, canRemoveLink } = deriveDomainLinkActionMeta(domain, linkTasks);
  const linkActionLabel = getLinkActionLabel(hasActiveLink, linkInProgress);
  const {
    runLinkTask,
    removeLinkTask,
    refreshLinkTasks,
    handleMainAction,
    handleForceStep,
    deleteGeneration,
    pauseGeneration,
    resumeGeneration,
    cancelGeneration,
    generationFlow,
    linkFlow
  } = useDomainAsyncActions({
    id,
    kw,
    domain,
    gens,
    latestAttempt,
    linkTasks,
    linkAnchor,
    linkAcceptor,
    canRemoveLink,
    load,
    setLoading,
    setError,
    setGens,
    setLatestAttempt,
    setPipelineStepInFlight,
    setLinkTasksLoading,
    setLinkTasksError,
    setLinkNotice,
    setLinkTasks
  });
  const latestArtifacts = useMemo<Record<string, any> | null>(() => {
    if (latestSuccessDetail?.artifacts && typeof latestSuccessDetail.artifacts === "object") {
      return latestSuccessDetail.artifacts;
    }
    return null;
  }, [latestSuccessDetail?.artifacts]);
  const latestSuccessSummary = useMemo<Record<string, any>>(() => {
    if (latestSuccess?.artifacts_summary && typeof latestSuccess.artifacts_summary === "object") {
      return latestSuccess.artifacts_summary;
    }
    if (latestSuccessDetail?.artifacts_summary && typeof latestSuccessDetail.artifacts_summary === "object") {
      return latestSuccessDetail.artifacts_summary;
    }
    return {};
  }, [latestSuccess?.artifacts_summary, latestSuccessDetail?.artifacts_summary]);
  const legacyDecodeMeta = useMemo(() => getLegacyDecodeMeta(latestArtifacts), [latestArtifacts]);
  const summaryLegacyDecodeMeta = useMemo(() => getLegacyDecodeMeta(latestSuccessSummary), [latestSuccessSummary]);
  const effectiveLegacyDecodeMeta = legacyDecodeMeta || summaryLegacyDecodeMeta;
  const hasArtifacts = Boolean(
    latestArtifacts && Object.keys(latestArtifacts).length > 0
  ) || Boolean(latestSuccessSummary.has_final_html || latestSuccessSummary.has_zip_archive || latestSuccessSummary.has_generated_files);
  const finalHTML = useMemo(() => getArtifactText(latestArtifacts, "final_html") || getArtifactText(latestArtifacts, "html_raw"), [latestArtifacts]);
  const zipArchive = useMemo(() => getArtifactText(latestArtifacts, "zip_archive"), [latestArtifacts]);
  const canOpenEditor = canOpenDomainEditor(domain);
  const showResultBlock = Boolean(latestSuccess) || hasArtifacts || domain?.status === "published";
  const resultSourceLabel = effectiveLegacyDecodeMeta ? "Legacy Decoded" : "Generated";
  const canEditPrompts = canEditPromptOverrides(myRole);

  const ensureGenerationDetails = async (generationId: string) => {
    if (!generationId || generationDetails[generationId]) {
      return;
    }
    try {
      const detail = await authFetch<GenerationDetail>(`/api/generations/${generationId}`);
      setGenerationDetails((prev) => ({ ...prev, [generationId]: detail }));
    } catch {
      // Данные деталей не критичны для рендера, показываем summary без raw.
    }
  };

  useEffect(() => {
    load();
  }, [id]);

  useEffect(() => {
    const ids = [latestAttempt?.id, latestSuccess?.id].filter((value): value is string => Boolean(value));
    ids.forEach((generationId) => {
      void ensureGenerationDetails(generationId);
    });
  }, [latestAttempt?.id, latestSuccess?.id]);

  const buildFileUrl = (path: string) => {
    const safe = path
      .split("/")
      .map((part) => encodeURIComponent(part))
      .join("/");
    return `/api/domains/${id}/files/${safe}`;
  };

  const buildEditorUrl = (filePath: string, line?: number): UrlObject => {
    const query: Record<string, string> = { path: filePath };
    if (line && line > 0) {
      query.line = String(line);
    }
    return {
      pathname: `/domains/${id}/editor`,
      query
    };
  };

  const editorHref: UrlObject = { pathname: `/domains/${id}/editor` };

  const downloadZipArchive = () => {
    if (!zipArchive) return;
    try {
      const binary = atob(zipArchive);
      const bytes = new Uint8Array(binary.length);
      for (let idx = 0; idx < binary.length; idx += 1) {
        bytes[idx] = binary.charCodeAt(idx);
      }
      const blob = new Blob([bytes], { type: "application/zip" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      const fileName = domain?.url ? `${domain.url}.zip` : "site.zip";
      a.download = fileName;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast({
        type: "error",
        title: "Ошибка скачивания ZIP",
        message: "Не удалось декодировать zip_archive"
      });
    }
  };

  const openLiveResultPreview = async () => {
    setResultHTMLTab("preview");
    setShowResultHTMLModal(true);
    setLiveResultLoading(true);
    setLiveResultError(null);
    try {
      const indexResp = await authFetch<{ content: string }>(`/api/domains/${id}/files/index.html`);
      const indexHtml = indexResp?.content || "";
      const styleResp = await authFetch<{ content: string }>(`/api/domains/${id}/files/style.css`).catch(() => null);
      const scriptResp = await authFetch<{ content: string }>(`/api/domains/${id}/files/script.js`).catch(() => null);

      const styleContent = styleResp?.content ? rewriteCssUrls(styleResp.content, id) : "";
      const scriptContent = scriptResp?.content || "";
      const htmlWithAssets = injectRuntimeAssets(indexHtml, styleContent, scriptContent);
      const livePreview = rewriteHtmlAssetRefs(htmlWithAssets, id);

      setLiveResultCode(indexHtml);
      setLiveResultHTML(livePreview);
    } catch {
      if (finalHTML) {
        setLiveResultCode(finalHTML);
        setLiveResultHTML(finalHTML);
        setLiveResultError("Показан артефактный HTML: живые файлы пока недоступны.");
      } else {
        setLiveResultCode("");
        setLiveResultHTML("");
        setLiveResultError("Не удалось загрузить live preview из файлов домена.");
      }
    } finally {
      setLiveResultLoading(false);
    }
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
      <DomainHeaderActionsSection
        domain={domain}
        projectName={projectName}
        error={error}
        currentAttempt={currentAttempt}
        mainButtonText={mainButtonText}
        mainButtonIcon={mainButtonIcon}
        mainButtonDisabled={mainButtonDisabled}
        loading={loading}
        canOpenEditor={canOpenEditor}
        editorHref={editorHref}
        generationFlow={generationFlow}
        linkFlow={linkFlow}
        renderStatusBadge={(status) => <StatusBadge status={status} />}
        onMainAction={handleMainAction}
        onResumeGeneration={resumeGeneration}
        onPauseGeneration={pauseGeneration}
        onCancelGeneration={cancelGeneration}
        onRefresh={() => load(true)}
      />

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <h3 className="font-semibold mb-3">Этапы генерации</h3>
        <PipelineSteps
          domainId={id}
          generation={latestAttemptDetail?.artifacts ? { ...currentAttempt, artifacts: latestAttemptDetail.artifacts } : currentAttempt || undefined}
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
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h3 className="font-semibold">Промпты домена</h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Переопределения промптов и моделей для этапов генерации.
            </p>
          </div>
          <button
            type="button"
            onClick={() => setShowDomainPromptOverrides((prev) => !prev)}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            {showDomainPromptOverrides ? "Скрыть блок" : "Показать блок"}
          </button>
        </div>
        {showDomainPromptOverrides ? (
          <PromptOverridesPanel
            title="Переопределения промптов (домен)"
            endpoint={`/api/domains/${id}/prompts`}
            canEdit={canEditPrompts}
            layout="single-stage"
          />
        ) : (
          <div className="rounded-lg border border-slate-200 bg-slate-50/70 px-3 py-2 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-300">
            Блок скрыт. Нажмите «Показать блок», чтобы открыть настройки промптов домена.
          </div>
        )}
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
              disabled={linkTasksLoading || linkInProgress || !linkAnchor.trim() || !linkAcceptor.trim()}
              className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
            >
              <FiPlay /> {linkActionLabel}
            </button>
            {canRemoveLink ? (
              <button
                onClick={removeLinkTask}
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
                              Открыть файл
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

      {showResultBlock && (
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h3 className="font-semibold">Результат</h3>
              <div className="mt-1 flex items-center gap-2">
                <Badge
                  label={resultSourceLabel}
                  tone={effectiveLegacyDecodeMeta ? "sky" : "emerald"}
                  icon={<FiCheck />}
                  className="text-xs"
                />
                {effectiveLegacyDecodeMeta?.decoded_at && (
                  <span className="text-xs text-slate-500 dark:text-slate-400">
                    Декодировано: {new Date(effectiveLegacyDecodeMeta.decoded_at).toLocaleString()}
                  </span>
                )}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => {
                  void openLiveResultPreview();
                }}
                disabled={!domain?.id}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                <FiCode /> Просмотр HTML
              </button>
              <button
                type="button"
                onClick={downloadZipArchive}
                disabled={!zipArchive}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                <FiDownload /> Скачать ZIP
              </button>
              {hasArtifacts && (
                <a
                  href="#domain-artifacts"
                  className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  К артефактам
                </a>
              )}
              {latestAttempt && (
                <Link
                  href={`/queue/${latestAttempt.id}`}
                  className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  Последняя попытка
                </Link>
              )}
              {latestSuccess && latestSuccess.id !== latestAttempt?.id && (
                <Link
                  href={`/queue/${latestSuccess.id}`}
                  className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  Последний успех
                </Link>
              )}
              {domain?.url && (
                <a
                  href={`https://${domain.url}`}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  Открыть по домену
                </a>
              )}
              {canOpenEditor ? (
                <Link
                  href={editorHref}
                  className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-xs font-semibold text-white hover:bg-indigo-500"
                >
                  <FiEdit3 /> Открыть в редакторе
                </Link>
              ) : (
                <span
                  title="Редактор доступен после публикации и синхронизации файлов"
                  className="inline-flex items-center gap-2 rounded-lg bg-slate-300 px-3 py-2 text-xs font-semibold text-slate-600"
                >
                  <FiEdit3 /> Открыть в редакторе
                </span>
              )}
            </div>
          </div>

          {!hasArtifacts && domain?.status === "published" && (
            <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
              Артефакты еще не заполнены. Запустите decode backfill: `go run ./cmd/backfill_legacy_artifacts --mode apply ...`
            </div>
          )}
        </div>
      )}

      {deployments[0] && (
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-2">
          <h3 className="font-semibold">Последний деплой</h3>
          <div className="text-sm text-slate-600 dark:text-slate-300">
            Статус: <StatusBadge status={deployments[0].status} /> · Режим: {deployments[0].mode}
          </div>
          <div className="text-xs text-slate-500 dark:text-slate-400">
            Путь: {deployments[0].target_path || "—"} · Owner: {deployments[0].owner_after || deployments[0].owner_before || "—"}
          </div>
          <div className="text-xs text-slate-500 dark:text-slate-400">
            Файлов: {deployments[0].file_count} · Размер: {formatBytes(deployments[0].total_size_bytes)}
          </div>
          {deployments[0].error_message && (
            <div className="text-xs text-red-500">{deployments[0].error_message}</div>
          )}
        </div>
      )}

      <DomainGenerationStatusSection
        generations={gens}
        visibleGenerations={visibleGens}
        loading={loading}
        renderStatusBadge={(status) => <StatusBadge status={status} />}
        computeProgress={(generation) => computeDisplayProgress(generation.artifacts, generation.progress, generation.status)}
        onResumeGeneration={resumeGeneration}
        onPauseGeneration={pauseGeneration}
        onCancelGeneration={cancelGeneration}
        onDeleteGeneration={deleteGeneration}
        onShowMore={() => setVisibleGens((v) => Math.min(gens.length, v + 3))}
      />

      <DomainLogsSection
        currentAttempt={currentAttempt}
        latestSuccess={latestSuccess}
        latestAttemptDetail={latestAttemptDetail}
        latestSuccessDetail={latestSuccessDetail}
        latestDisplayProgress={latestDisplayProgress}
        renderStatusBadge={(status) => <StatusBadge status={status} />}
      />

      {showResultHTMLModal && (
        <div className="fixed inset-0 z-50 bg-black/60 px-3 py-6 md:px-8 overflow-auto">
          <div className="mx-auto max-w-6xl rounded-xl border border-slate-200 bg-white p-4 shadow-2xl dark:border-slate-800 dark:bg-slate-950">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h4 className="text-sm font-semibold">Финальный HTML</h4>
              <button
                type="button"
                onClick={() => setShowResultHTMLModal(false)}
                className="rounded-lg border border-slate-200 px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-100 dark:hover:bg-slate-800"
              >
                Закрыть
              </button>
            </div>
            <div className="mt-3 flex items-center gap-2">
              <button
                type="button"
                onClick={() => setResultHTMLTab("preview")}
                className={`rounded-lg border px-3 py-1 text-xs font-semibold ${
                  resultHTMLTab === "preview"
                    ? "bg-indigo-600 border-indigo-600 text-white"
                    : "border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-100"
                }`}
              >
                Preview
              </button>
              <button
                type="button"
                onClick={() => setResultHTMLTab("code")}
                className={`rounded-lg border px-3 py-1 text-xs font-semibold ${
                  resultHTMLTab === "code"
                    ? "bg-indigo-600 border-indigo-600 text-white"
                    : "border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-100"
                }`}
              >
                Code
              </button>
            </div>
            {liveResultLoading ? (
              <div className="mt-3 h-[70vh] rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-300">
                Загружаем живой preview...
              </div>
            ) : (
              <>
                {liveResultError && (
                  <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
                    {liveResultError}
                  </div>
                )}
                {resultHTMLTab === "preview" ? (
                  <iframe
                    title="Final HTML Preview"
                    sandbox="allow-same-origin allow-scripts"
                    srcDoc={liveResultHTML}
                    className="mt-3 h-[70vh] w-full rounded-lg border border-slate-200 dark:border-slate-700 bg-white"
                  />
                ) : (
                  <pre className="mt-3 h-[70vh] overflow-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs dark:border-slate-700 dark:bg-slate-900/60">
                    {liveResultCode || "(файл index.html отсутствует)"}
                  </pre>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function getArtifactText(artifacts: Record<string, any> | null, key: string): string {
  if (!artifacts) return "";
  const value = artifacts[key];
  if (typeof value === "string") return value;
  return "";
}

function getLegacyDecodeMeta(artifacts: Record<string, any> | null): { source?: string; decoded_at?: string } | null {
  if (!artifacts) return null;
  const raw = artifacts.legacy_decode_meta;
  if (!raw || typeof raw !== "object") return null;
  const source = typeof (raw as any).source === "string" ? (raw as any).source : undefined;
  const decodedAt = typeof (raw as any).decoded_at === "string" ? (raw as any).decoded_at : undefined;
  return { source, decoded_at: decodedAt };
}

function rewriteHtmlAssetRefs(html: string, domainId: string): string {
  const base = apiBase();
  return html.replace(/\b(src|href)\s*=\s*["']([^"']+)["']/gi, (full, attr, rawValue: string) => {
    const value = rawValue.trim();
    if (!value || value.startsWith("#")) return full;
    if (/^(data:|mailto:|tel:|javascript:|https?:)/i.test(value)) return full;
    const normalized = value.replace(/^\.\//, "").replace(/^\//, "");
    if (!normalized) return full;
    const [pathPart, hashPart = ""] = normalized.split("#");
    const [purePath, queryPart = ""] = pathPart.split("?");
    if (!purePath) return full;
    const encodedPath = purePath
      .split("/")
      .filter(Boolean)
      .map((part) => encodeURIComponent(part))
      .join("/");
    if (!encodedPath) return full;
    const query = queryPart ? `&${queryPart}` : "";
    const hash = hashPart ? `#${hashPart}` : "";
    const url = `${base}/api/domains/${domainId}/files/${encodedPath}?raw=1${query}${hash}`;
    return `${attr}="${url}"`;
  });
}

function rewriteCssUrls(css: string, domainId: string): string {
  const base = apiBase();
  return css.replace(/url\(([^)]+)\)/gi, (_full, rawValue: string) => {
    const value = rawValue.trim().replace(/^['"]|['"]$/g, "");
    if (!value || value.startsWith("#")) return `url(${rawValue})`;
    if (/^(data:|https?:|blob:)/i.test(value)) return `url(${rawValue})`;
    const normalized = value.replace(/^\.\//, "").replace(/^\//, "");
    const [pathPart, hashPart = ""] = normalized.split("#");
    const [purePath, queryPart = ""] = pathPart.split("?");
    if (!purePath) return `url(${rawValue})`;
    const encodedPath = purePath
      .split("/")
      .filter(Boolean)
      .map((part) => encodeURIComponent(part))
      .join("/");
    if (!encodedPath) return `url(${rawValue})`;
    const query = queryPart ? `&${queryPart}` : "";
    const hash = hashPart ? `#${hashPart}` : "";
    return `url("${base}/api/domains/${domainId}/files/${encodedPath}?raw=1${query}${hash}")`;
  });
}

function injectRuntimeAssets(indexHtml: string, styleContent: string, scriptContent: string): string {
  let html = indexHtml || "";
  if (styleContent) {
    html = html.replace(/<link[^>]*href=["']style\.css["'][^>]*>/gi, "");
    if (/<\/head>/i.test(html)) {
      html = html.replace(/<\/head>/i, `<style data-live-preview="style.css">\n${styleContent}\n</style>\n</head>`);
    } else {
      html = `<style data-live-preview="style.css">\n${styleContent}\n</style>\n${html}`;
    }
  }
  if (scriptContent) {
    html = html.replace(/<script[^>]*src=["']script\.js["'][^>]*>\s*<\/script>/gi, "");
    if (/<\/body>/i.test(html)) {
      html = html.replace(/<\/body>/i, `<script data-live-preview="script.js">\n${scriptContent}\n</script>\n</body>`);
    } else {
      html = `${html}\n<script data-live-preview="script.js">\n${scriptContent}\n</script>`;
    }
  }
  return html;
}

function formatBytes(value?: number): string {
  if (!value || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

function StatusBadge({ status }: { status: string }) {
  const meta = getGenerationStatusMeta(status);
  const icon =
    meta.icon === "play"
      ? <FiPlay />
      : meta.icon === "pause"
        ? <FiPause />
        : meta.icon === "check"
          ? <FiCheck />
          : meta.icon === "alert"
            ? <FiAlertTriangle />
            : meta.icon === "x"
              ? <FiX />
              : <FiClock />;
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

function LinkTaskStatusBadge({ status }: { status: string }) {
  const meta = getLinkTaskStatusMeta(status);
  const icon =
    meta.icon === "refresh" ? <FiRefreshCw /> : meta.icon === "check" ? <FiCheck /> : meta.icon === "alert" ? <FiAlertTriangle /> : <FiClock />;
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}
