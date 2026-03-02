"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import { authFetch, authFetchCached, patch } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import { FiPlay, FiRefreshCw } from "react-icons/fi";
import PipelineSteps from "../../../components/PipelineSteps";
import { computeDisplayProgress } from "../../../lib/pipelineProgress";
import { getLinkActionLabel } from "../../../features/domain-project/services/statusCta";
import { canEditPromptOverrides, canOpenDomainEditor, isMainGenerationActionDisabled } from "../../../features/domain-project/services/actionGuards";
import { deriveDomainLinkActionMeta, deriveMainGenerationMeta } from "../../../features/domain-project/services/statusMeta";
import { useDomainActions } from "../../../features/domain-project/hooks/useDomainActions";
import { DomainHeaderActionsSection } from "../../../features/domain-project/components/DomainHeaderActionsSection";
import { DomainGenerationStatusSection } from "../../../features/domain-project/components/DomainGenerationStatusSection";
import { DomainLogsSection } from "../../../features/domain-project/components/DomainLogsSection";
import { DomainStatusBadge } from "../../../features/domain-project/components/DomainStatusBadges";
import { DomainSettingsSection } from "../../../features/domain-project/components/DomainSettingsSection";
import { DomainLinkStatusSection } from "../../../features/domain-project/components/DomainLinkStatusSection";
import { DomainResultSection } from "../../../features/domain-project/components/DomainResultSection";

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
  const [deployments, setDeployments] = useState<DeploymentAttempt[]>([]);

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
  } = useDomainActions({
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
  const showResultBlock = Boolean(latestSuccess) || hasArtifacts || domain?.status === "published";
  const finalHTML = useMemo(() => getArtifactText(latestArtifacts, "final_html") || getArtifactText(latestArtifacts, "html_raw"), [latestArtifacts]);
  const zipArchive = useMemo(() => getArtifactText(latestArtifacts, "zip_archive"), [latestArtifacts]);
  const canOpenEditor = canOpenDomainEditor(domain);
  const resultSourceLabel = effectiveLegacyDecodeMeta ? "Legacy Decoded" : "Generated";
  const resultBlockLabels = {
    title: "Результат",
    htmlAction: "Просмотр HTML",
    zipAction: "Скачать ZIP",
    artifactsAction: "К артефактам",
    editorAction: "Открыть в редакторе",
    editorDisabledHint: "Редактор доступен после публикации и синхронизации файлов",
    backfillHint: "go run ./cmd/import_legacy --mode apply --source auto"
  };
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

  const editorHref = { pathname: `/domains/${id}/editor` } as const;

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
        renderStatusBadge={(status) => <DomainStatusBadge status={status} />}
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

      <DomainSettingsSection
        domainId={id}
        loading={loading}
        kw={kw}
        linkAnchor={linkAnchor}
        linkAcceptor={linkAcceptor}
        server={server}
        country={country}
        language={language}
        exclude={exclude}
        canEditPrompts={canEditPrompts}
        onKwChange={setKw}
        onLinkAnchorChange={setLinkAnchor}
        onLinkAcceptorChange={setLinkAcceptor}
        onServerChange={setServer}
        onCountryChange={setCountry}
        onLanguageChange={setLanguage}
        onExcludeChange={setExclude}
        onSave={save}
      />

      <DomainLinkStatusSection
        domainId={id}
        domain={domain}
        linkTasks={linkTasks}
        linkTasksLoading={linkTasksLoading}
        linkTasksError={linkTasksError}
        linkNotice={linkNotice}
        linkAnchor={linkAnchor}
        linkAcceptor={linkAcceptor}
        linkInProgress={linkInProgress}
        canRemoveLink={canRemoveLink}
        linkActionLabel={linkActionLabel}
        onRefreshLinkTasks={refreshLinkTasks}
        onRunLinkTask={runLinkTask}
        onRemoveLinkTask={removeLinkTask}
      />

      <DomainResultSection
        showResultBlock={showResultBlock}
        domainId={id}
        domain={domain}
        latestAttempt={latestAttempt}
        latestSuccess={latestSuccess}
        hasArtifacts={hasArtifacts}
        resultSourceLabel={resultSourceLabel}
        legacyDecodedAt={effectiveLegacyDecodeMeta?.decoded_at}
        zipArchive={zipArchive}
        finalHTML={finalHTML}
        canOpenEditor={canOpenEditor}
        editorHref={editorHref}
        deployments={deployments}
        renderStatusBadge={(status) => <DomainStatusBadge status={status} />}
        labels={resultBlockLabels}
      />

      <DomainGenerationStatusSection
        generations={gens}
        visibleGenerations={visibleGens}
        loading={loading}
        renderStatusBadge={(status) => <DomainStatusBadge status={status} />}
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
        renderStatusBadge={(status) => <DomainStatusBadge status={status} />}
      />
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
