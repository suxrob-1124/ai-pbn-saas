'use client';

import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { authFetch, authFetchCached, patch } from '@/lib/http';
import { useAuthGuard } from '@/lib/useAuth';
import { showToast } from '@/lib/toastStore';
import { FiPlay, FiRefreshCw } from 'react-icons/fi';
import {
  Edit3,
  ExternalLink,
  Activity,
  Settings,
  List,
  Terminal,
  ShieldAlert,
  ChevronRight,
  Globe,
} from 'lucide-react';
import Link from 'next/link';

import PipelineSteps from '@/components/PipelineSteps';
import { computeDisplayProgress } from '@/lib/pipelineProgress';
import { getLinkActionLabel } from '@/features/domain-project/services/statusCta';
import {
  canEditPromptOverrides,
  canOpenDomainEditor,
  isMainGenerationActionDisabled,
} from '@/features/domain-project/services/actionGuards';
import {
  deriveDomainLinkActionMeta,
  deriveMainGenerationMeta,
} from '@/features/domain-project/services/statusMeta';
import { useDomainActions } from '@/features/domain-project/hooks/useDomainActions';

import { DomainHeaderActionsSection } from '@/features/domain-project/components/DomainHeaderActionsSection';
import { DomainGenerationStatusSection } from '@/features/domain-project/components/DomainGenerationStatusSection';
import { DomainLogsSection } from '@/features/domain-project/components/DomainLogsSection';
import { DomainStatusBadge } from '@/features/domain-project/components/DomainStatusBadges';
import { DomainSettingsSection } from '@/features/domain-project/components/DomainSettingsSection';
import { getGenerationTypeLabel, isGenerationTypeAvailable } from '@/features/domain-project/services/generationTypes';
import { DomainLinkStatusSection } from '@/features/domain-project/components/DomainLinkStatusSection';
import { DomainResultSection } from '@/features/domain-project/components/DomainResultSection';

// --- ТИПЫ (БЕЗ ИЗМЕНЕНИЙ) ---
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
  link_status_source?: 'domain' | 'active_task';
  index_check_enabled?: boolean;
  generation_type?: string;
};
type Generation = {
  id: string;
  domain_id?: string;
  generation_type?: string;
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
  my_role?: 'admin' | 'owner' | 'manager' | 'editor' | 'viewer';
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
  const [myRole, setMyRole] = useState<'admin' | 'owner' | 'manager' | 'editor' | 'viewer'>(
    'viewer',
  );
  const [projectName, setProjectName] = useState<string>('');

  const [kw, setKw] = useState('');
  const [country, setCountry] = useState('');
  const [language, setLanguage] = useState('');
  const [exclude, setExclude] = useState('');
  const [server, setServer] = useState('');

  const [loading, setLoading] = useState(false);
  const [visibleGens, setVisibleGens] = useState(2);
  const [error, setError] = useState<string | null>(null);
  const [pipelineStepInFlight, setPipelineStepInFlight] = useState<string | null>(null);

  const [linkAnchor, setLinkAnchor] = useState('');
  const [linkAcceptor, setLinkAcceptor] = useState('');
  const [linkTasks, setLinkTasks] = useState<LinkTask[]>([]);
  const [linkTasksLoading, setLinkTasksLoading] = useState(false);
  const [linkTasksError, setLinkTasksError] = useState<string | null>(null);
  const [linkNotice, setLinkNotice] = useState<string | null>(null);
  const [deployments, setDeployments] = useState<DeploymentAttempt[]>([]);

  const [activeTab, setActiveTab] = useState<'pipeline' | 'logs'>('pipeline');
  const [indexCheckEnabled, setIndexCheckEnabled] = useState(true);
  const [indexCheckLoading, setIndexCheckLoading] = useState(false);
  const [generationType, setGenerationType] = useState('single_page');

  const hasExtendedAccess = myRole === 'admin' || myRole === 'owner' || myRole === 'manager';

  const load = async (force = false) => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const summary = await authFetchCached<DomainSummary>(
        `/api/domains/${id}/summary?gen_limit=10&link_limit=20`,
        undefined,
        { ttlMs: 15000, bypassCache: force },
      );
      const d = summary?.domain || null;
      setDomain(d);
      setProjectName(summary?.project_name || '');
      setMyRole(summary?.my_role || 'viewer');
      setKw(d?.main_keyword || '');
      setCountry(d?.target_country || '');
      setLanguage(d?.target_language || '');
      setExclude(d?.exclude_domains || '');
      setServer(d?.server_id || '');
      setLinkAnchor(d?.link_anchor_text || '');
      setLinkAcceptor(d?.link_acceptor_url || '');
      setIndexCheckEnabled(d?.index_check_enabled !== false);
      setGenerationType(d?.generation_type || 'single_page');

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
      setError(err?.message || 'Не удалось загрузить домен');
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
        link_acceptor_url: linkAcceptor,
        generation_type: generationType,
      });
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Не удалось сохранить');
    } finally {
      setLoading(false);
    }
  };

  const handleToggleIndexCheck = async (enabled: boolean) => {
    if (!id) return;
    setIndexCheckLoading(true);
    try {
      const { setDomainIndexCheckerControl } = await import('@/lib/indexChecksApi');
      await setDomainIndexCheckerControl(id, enabled);
      setIndexCheckEnabled(enabled);
      showToast({
        type: 'success',
        title: enabled ? 'Проверка индексации включена' : 'Проверка индексации отключена',
      });
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    } finally {
      setIndexCheckLoading(false);
    }
  };

  const { currentAttempt, isRegenerate, mainButtonText } = deriveMainGenerationMeta(
    latestAttempt,
    gens,
  );
  const latestAttemptDetail = currentAttempt?.id ? generationDetails[currentAttempt.id] : undefined;
  const latestSuccessDetail = latestSuccess?.id ? generationDetails[latestSuccess.id] : undefined;
  const latestDisplayProgress = currentAttempt
    ? computeDisplayProgress(
        latestAttemptDetail?.artifacts,
        currentAttempt.progress,
        currentAttempt.status,
      )
    : 0;
  const mainButtonIcon = isRegenerate ? <FiRefreshCw /> : <FiPlay />;
  const mainButtonDisabled = isMainGenerationActionDisabled(loading, currentAttempt?.status);
  const { hasActiveLink, linkInProgress, canRemoveLink } = deriveDomainLinkActionMeta(
    domain,
    linkTasks,
  );
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
    linkFlow,
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
    setLinkTasks,
  });

  const latestArtifacts = useMemo<Record<string, any> | null>(() => {
    if (latestSuccessDetail?.artifacts && typeof latestSuccessDetail.artifacts === 'object')
      return latestSuccessDetail.artifacts;
    return null;
  }, [latestSuccessDetail?.artifacts]);

  const latestSuccessSummary = useMemo<Record<string, any>>(() => {
    if (latestSuccess?.artifacts_summary && typeof latestSuccess.artifacts_summary === 'object')
      return latestSuccess.artifacts_summary;
    if (
      latestSuccessDetail?.artifacts_summary &&
      typeof latestSuccessDetail.artifacts_summary === 'object'
    )
      return latestSuccessDetail.artifacts_summary;
    return {};
  }, [latestSuccess?.artifacts_summary, latestSuccessDetail?.artifacts_summary]);

  const legacyDecodeMeta = useMemo(() => getLegacyDecodeMeta(latestArtifacts), [latestArtifacts]);
  const summaryLegacyDecodeMeta = useMemo(
    () => getLegacyDecodeMeta(latestSuccessSummary),
    [latestSuccessSummary],
  );
  const effectiveLegacyDecodeMeta = legacyDecodeMeta || summaryLegacyDecodeMeta;
  const hasArtifacts =
    Boolean(latestArtifacts && Object.keys(latestArtifacts).length > 0) ||
    Boolean(
      latestSuccessSummary.has_final_html ||
      latestSuccessSummary.has_zip_archive ||
      latestSuccessSummary.has_generated_files,
    );
  const showResultBlock = Boolean(latestSuccess) || hasArtifacts || domain?.status === 'published';
  const finalHTML = useMemo(
    () =>
      getArtifactText(latestArtifacts, 'final_html') ||
      getArtifactText(latestArtifacts, 'html_raw'),
    [latestArtifacts],
  );
  const zipArchive = useMemo(
    () => getArtifactText(latestArtifacts, 'zip_archive'),
    [latestArtifacts],
  );

  const canOpenEditor = canOpenDomainEditor(domain as any);
  const resultSourceLabel = effectiveLegacyDecodeMeta ? 'Legacy Decoded' : 'Generated';
  const resultBlockLabels = {
    title: 'Результат',
    htmlAction: 'Просмотр HTML',
    zipAction: 'Скачать ZIP',
    artifactsAction: 'К артефактам',
    editorAction: 'Открыть в редакторе',
    editorDisabledHint: 'Редактор доступен после публикации и синхронизации файлов',
    backfillHint: 'go run ./cmd/import_legacy --mode apply --source auto',
  };
  const canEditPrompts = canEditPromptOverrides(myRole);

  const ensureGenerationDetails = async (generationId: string) => {
    if (!generationId || generationDetails[generationId]) return;
    try {
      const detail = await authFetch<GenerationDetail>(`/api/generations/${generationId}`);
      setGenerationDetails((prev) => ({ ...prev, [generationId]: detail }));
    } catch {}
  };

  useEffect(() => {
    load();
  }, [id]);
  useEffect(() => {
    const ids = [latestAttempt?.id, latestSuccess?.id].filter((value): value is string =>
      Boolean(value),
    );
    ids.forEach((generationId) => ensureGenerationDetails(generationId));
  }, [latestAttempt?.id, latestSuccess?.id]);
  useEffect(() => {
    setVisibleGens(2);
  }, [gens.length]);

  const editorHref = { pathname: `/domains/${id}/editor` } as any;

  if (!domain && loading) {
    return (
      <div className="p-10 flex justify-center text-slate-500">
        <FiRefreshCw className="w-5 h-5 animate-spin mr-2" /> Загрузка данных сайта...
      </div>
    );
  }

  // --- VIEW ДЛЯ РЕДАКТОРА ---
  if (!hasExtendedAccess) {
    return (
      <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
        <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10 flex items-center justify-between">
          <div>
            <div className="flex items-center text-sm text-slate-500 dark:text-slate-400 mb-1">
              <Link href="/projects" className="hover:text-indigo-600 transition-colors">
                Мои сайты
              </Link>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              {domain?.url} <DomainStatusBadge status={domain?.status || ''} />
              {domain?.generation_type && domain.generation_type !== 'single_page' && (
                <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                  {getGenerationTypeLabel(domain.generation_type)}
                </span>
              )}
            </h1>
          </div>
          <Link
            href={editorHref}
            className="inline-flex items-center gap-2 px-6 py-3 rounded-xl font-bold transition-all shadow-lg bg-indigo-600 text-white hover:bg-indigo-500 hover:shadow-indigo-500/25 active:scale-95">
            <Edit3 className="w-5 h-5" /> Открыть редактор контента
          </Link>
        </header>
        <main className="flex-1 overflow-y-auto p-6">
          <div className="max-w-5xl mx-auto">
            {error && <div className="p-4 bg-red-50 text-red-600 rounded-xl mb-6">{error}</div>}
            <div className="rounded-2xl border border-slate-200 bg-white p-12 text-center shadow-sm dark:border-slate-800 dark:bg-[#0f1117]">
              <Globe className="w-16 h-16 text-indigo-100 dark:text-indigo-900 mx-auto mb-4" />
              <h2 className="text-xl font-bold text-slate-900 dark:text-white mb-2">
                Сайт готов к редактированию
              </h2>
              <p className="text-slate-500 dark:text-slate-400 max-w-md mx-auto">
                Перейдите в редактор контента (кнопка сверху), чтобы просмотреть и отредактировать файлы сайта.
              </p>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // --- VIEW ДЛЯ АДМИНА/ВЛАДЕЛЬЦА ---
  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      {/* HEADER: Пульт управления */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-20">
        <div className="max-w-7xl mx-auto flex flex-col xl:flex-row xl:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm font-medium text-slate-500 dark:text-slate-400 mb-1">
              <Link href="/projects" className="hover:text-indigo-600 transition-colors">
                Проекты
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <Link
                href={`/projects/${domain?.project_id}`}
                className="hover:text-indigo-600 transition-colors">
                {projectName || 'Проект'}
              </Link>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              {domain?.url} <DomainStatusBadge status={domain?.status || ''} />
              {domain?.generation_type && domain.generation_type !== 'single_page' && (
                <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                  {getGenerationTypeLabel(domain.generation_type)}
                </span>
              )}
            </h1>
          </div>

          <div className="flex items-center gap-2">
            <DomainHeaderActionsSection
              domain={domain as any}
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
            {/* Большая кнопка редактора всегда под рукой */}
            <Link
              href={editorHref}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl font-bold transition-all ml-2 shadow-sm bg-indigo-600 text-white hover:bg-indigo-500 active:scale-95">
              <Edit3 className="w-4 h-4" /> В редактор
            </Link>
          </div>
        </div>
      </header>

      {/* ОСНОВНАЯ ЗОНА (Сетка из двух колонок) */}
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto">
          {error && (
            <div className="p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50 mb-6 flex items-center gap-2">
              <ShieldAlert className="w-5 h-5" /> {error}
            </div>
          )}

          <div className="grid grid-cols-1 xl:grid-cols-[1fr_400px] gap-6 items-start">
            {/* ЛЕВАЯ КОЛОНКА (Пайплайны и Результат) */}
            <div className="space-y-6">
              {/* ВИЗУАЛЬНЫЙ ПАЙПЛАЙН */}
              <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex items-center justify-between">
                  <h3 className="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <Activity className="w-4 h-4 text-indigo-500" /> Процесс генерации
                  </h3>
                  <span className="text-xs font-mono text-slate-500">
                    {latestDisplayProgress}% завершено
                  </span>
                </div>
                <div className="p-6">
                  <PipelineSteps
                    domainId={id}
                    generation={
                      latestAttemptDetail?.artifacts
                        ? ({ ...currentAttempt, artifacts: latestAttemptDetail.artifacts } as any)
                        : (currentAttempt as any) || undefined
                    }
                    disabled={loading}
                    activeStep={pipelineStepInFlight}
                    onForceStep={handleForceStep}
                  />
                </div>
              </div>

              {/* БЛОК РЕЗУЛЬТАТА (Артефакты) */}
              <div className="animate-in fade-in">
                <DomainResultSection
                  showResultBlock={showResultBlock}
                  domainId={id}
                  domain={domain as any}
                  latestAttempt={latestAttempt as any}
                  latestSuccess={latestSuccess as any}
                  hasArtifacts={hasArtifacts}
                  resultSourceLabel={resultSourceLabel}
                  legacyDecodedAt={effectiveLegacyDecodeMeta?.decoded_at}
                  zipArchive={zipArchive}
                  finalHTML={finalHTML}
                  canOpenEditor={canOpenEditor}
                  editorHref={editorHref}
                  deployments={deployments as any}
                  renderStatusBadge={(status) => <DomainStatusBadge status={status} />}
                  labels={resultBlockLabels}
                />
              </div>

              {/* ТАБЫ: ИСТОРИЯ И ЛОГИ (Скрыты в гармошку) */}
              <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                <div className="flex border-b border-slate-100 dark:border-slate-800/60 bg-slate-50 dark:bg-[#0a1020]">
                  <button
                    onClick={() => setActiveTab('pipeline')}
                    className={`flex-1 py-3 text-sm font-semibold transition-colors ${activeTab === 'pipeline' ? 'text-indigo-600 border-b-2 border-indigo-600' : 'text-slate-500 hover:text-slate-700'}`}>
                    История запусков
                  </button>
                  <button
                    onClick={() => setActiveTab('logs')}
                    className={`flex-1 py-3 text-sm font-semibold transition-colors ${activeTab === 'logs' ? 'text-indigo-600 border-b-2 border-indigo-600' : 'text-slate-500 hover:text-slate-700'}`}>
                    Логи воркеров
                  </button>
                </div>
                <div className="p-5">
                  {activeTab === 'pipeline' && (
                    <DomainGenerationStatusSection
                      generations={gens as any}
                      visibleGenerations={visibleGens}
                      loading={loading}
                      renderStatusBadge={(status) => <DomainStatusBadge status={status} />}
                      computeProgress={(g) =>
                        computeDisplayProgress(g.artifacts, g.progress, g.status)
                      }
                      onResumeGeneration={resumeGeneration}
                      onPauseGeneration={pauseGeneration}
                      onCancelGeneration={cancelGeneration}
                      onDeleteGeneration={deleteGeneration}
                      onShowMore={() => setVisibleGens((v) => Math.min(gens.length, v + 3))}
                    />
                  )}
                  {activeTab === 'logs' && (
                    <DomainLogsSection
                      currentAttempt={currentAttempt as any}
                      latestSuccess={latestSuccess as any}
                      latestAttemptDetail={latestAttemptDetail as any}
                      latestSuccessDetail={latestSuccessDetail as any}
                      latestDisplayProgress={latestDisplayProgress}
                      renderStatusBadge={(status) => <DomainStatusBadge status={status} />}
                    />
                  )}
                </div>
              </div>
            </div>

            {/* ПРАВАЯ КОЛОНКА (Сайдбар: Настройки и Ссылки) */}
            <div className="space-y-6">
              {/* НАСТРОЙКИ */}
              <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                  <h3 className="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <Settings className="w-4 h-4 text-indigo-500" /> Настройки сайта
                  </h3>
                </div>
                <div className="p-5">
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
                    generationType={generationType}
                    canEditPrompts={canEditPrompts}
                    onKwChange={setKw}
                    onLinkAnchorChange={setLinkAnchor}
                    onLinkAcceptorChange={setLinkAcceptor}
                    onServerChange={setServer}
                    onCountryChange={setCountry}
                    onLanguageChange={setLanguage}
                    onExcludeChange={setExclude}
                    onGenerationTypeChange={setGenerationType}
                    onSave={save}
                    indexCheckEnabled={indexCheckEnabled}
                    indexCheckLoading={indexCheckLoading}
                    onToggleIndexCheck={handleToggleIndexCheck}
                  />
                </div>
              </div>

              {/* ЛИНКБИЛДИНГ (Link Tasks) */}
              <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                  <h3 className="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <ExternalLink className="w-4 h-4 text-indigo-500" /> Линкбилдинг
                  </h3>
                </div>
                <div className="p-5">
                  <DomainLinkStatusSection
                    domainId={id}
                    domain={domain as any}
                    linkTasks={linkTasks as any}
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
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

// --- УТИЛИТЫ ---
function getArtifactText(artifacts: Record<string, any> | null, key: string): string {
  if (!artifacts) return '';
  const value = artifacts[key];
  if (typeof value === 'string') return value;
  return '';
}

function getLegacyDecodeMeta(
  artifacts: Record<string, any> | null,
): { source?: string; decoded_at?: string } | null {
  if (!artifacts) return null;
  const raw = artifacts.legacy_decode_meta;
  if (!raw || typeof raw !== 'object') return null;
  const source = typeof (raw as any).source === 'string' ? (raw as any).source : undefined;
  const decodedAt =
    typeof (raw as any).decoded_at === 'string' ? (raw as any).decoded_at : undefined;
  return { source, decoded_at: decodedAt };
}
