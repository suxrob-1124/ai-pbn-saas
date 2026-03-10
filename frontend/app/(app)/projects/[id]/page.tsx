'use client';

import { useEffect, useMemo, useState } from 'react';
import type { UrlObject } from 'url';
import { useParams, usePathname, useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import {
  Search,
  Plus,
  UploadCloud,
  Globe,
  LayoutTemplate,
  Settings,
  Clock,
  RefreshCcw,
  ShieldAlert,
  X,
  ChevronRight,
} from 'lucide-react';

import { authFetch, authFetchCached, post, patch, del } from '@/lib/http';
import { useAuthGuard } from '@/lib/useAuth';
import { showToast } from '@/lib/toastStore';

import { canEditPromptOverrides } from '@/features/domain-project/services/actionGuards';
import { getEffectiveDomainLinkStatus } from '@/features/domain-project/services/statusMeta';
import { useProjectActions } from '@/features/domain-project/hooks/useProjectActions';
import { useProjectSchedules } from '@/features/domain-project/hooks/useProjectSchedules';
import { ProjectHeaderActionsSection } from '@/features/domain-project/components/ProjectHeaderActionsSection';
import { ProjectDomainsSection } from '@/features/domain-project/components/ProjectDomainsSection';
import { ProjectSchedulesSection } from '@/features/domain-project/components/ProjectSchedulesSection';
import { ProjectDiagnosticsSection } from '@/features/domain-project/components/ProjectDiagnosticsSection';
import { ProjectSettingsSection } from '@/features/domain-project/components/ProjectSettingsSection';
import {
  ProjectLinkStatusBadge,
  ProjectRunsList,
  ProjectStatusBadge,
} from '@/features/domain-project/components/ProjectStatusBadges';
import {
  normalizeDomainForImport,
  parseDomainImportText,
} from '@/features/domain-project/services/domainImport';
import { GENERATION_TYPES } from '@/features/domain-project/services/generationTypes';
import type { GenerationType } from '@/features/domain-project/services/generationTypes';
import { useLegacyImport } from '@/features/domain-project/hooks/useLegacyImport';
import LegacyImportModal from '@/features/domain-project/components/LegacyImportModal';
import LegacyImportProgress from '@/features/domain-project/components/LegacyImportProgress';
import {
  formatDateTime as formatDateTimeWithTimezone,
  formatRelativeTime,
  type Domain,
  type Generation,
  type Project,
  type ProjectSummary,
  type ProjectTab,
} from '@/features/domain-project/services/projectPageUtils';

export default function ProjectDetailPage() {
  const { me } = useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const projectId = params?.id as string;

  const [project, setProject] = useState<Project | null>(null);
  const [myRole, setMyRole] = useState<'admin' | 'owner' | 'editor' | 'viewer'>('viewer');
  const [domains, setDomains] = useState<Domain[]>([]);
  const [domainSearch, setDomainSearch] = useState('');

  const domainById = useMemo(() => {
    const map: Record<string, Domain> = {};
    domains.forEach((domain) => {
      map[domain.id] = domain;
    });
    return map;
  }, [domains]);

  const [gens, setGens] = useState<Record<string, Generation[]>>({});
  const [openRuns, setOpenRuns] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [projectName, setProjectName] = useState('');
  const [projectCountry, setProjectCountry] = useState('');
  const [projectLanguage, setProjectLanguage] = useState('');
  const [projectTimezone, setProjectTimezone] = useState('');
  const [timezoneQuery, setTimezoneQuery] = useState('');
  const [recentTimezones, setRecentTimezones] = useState<string[]>([]);
  const [projectSettingsLoading, setProjectSettingsLoading] = useState(false);
  const [projectSettingsError, setProjectSettingsError] = useState<string | null>(null);

  const [url, setUrl] = useState('');
  const [keyword, setKeyword] = useState('');
  const [country, setCountry] = useState('');
  const [language, setLanguage] = useState('');
  const [exclude, setExclude] = useState('');
  const [addGenType, setAddGenType] = useState('single_page');
  const [importText, setImportText] = useState('');
  const [keywordEdits, setKeywordEdits] = useState<Record<string, string>>({});
  const [linkEdits, setLinkEdits] = useState<Record<string, { anchor: string; acceptor: string }>>(
    {},
  );

  const [members, setMembers] = useState<Array<{ email: string; role: string; createdAt: string }>>(
    [],
  );
  const [newMemberEmail, setNewMemberEmail] = useState('');
  const [newMemberRole, setNewMemberRole] = useState('editor');
  const [linkLoadingId, setLinkLoadingId] = useState<string | null>(null);
  const [projectErrors, setProjectErrors] = useState<Generation[]>([]);
  const [projectErrorsLoading, setProjectErrorsLoading] = useState(false);
  const [projectErrorsError, setProjectErrorsError] = useState<string | null>(null);
  const [indexCheckEnabled, setIndexCheckEnabled] = useState(true);
  const [indexCheckLoading, setIndexCheckLoading] = useState(false);

  const [uiView, setUiView] = useState<'sites' | 'seo' | 'schedules' | 'errors' | 'settings'>(
    'sites',
  );
  const [activeTab, setActiveTab] = useState<ProjectTab>('domains');
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [isImportModalOpen, setIsImportModalOpen] = useState(false);
  const [isLegacyImportModalOpen, setIsLegacyImportModalOpen] = useState(false);
  const [showLegacyProgress, setShowLegacyProgress] = useState(false);
  const [runLegacyAfterImport, setRunLegacyAfterImport] = useState(false);
  const legacyImport = useLegacyImport(projectId);

  const hasExtendedAccess = myRole === 'admin' || myRole === 'owner';

  useEffect(() => {
    if (uiView === 'errors') setActiveTab('errors');
    else if (uiView === 'schedules') setActiveTab('schedules');
    else if (uiView === 'settings') setActiveTab('settings');
    else setActiveTab('domains');
  }, [uiView]);

  const timezoneFallback = useMemo(() => ['UTC', 'Europe/Moscow', 'America/New_York'], []);

  const availableTimezones = useMemo(() => {
    let zones: string[] = [];
    try {
      const supported = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] })
        .supportedValuesOf;
      if (typeof supported === 'function') zones = supported('timeZone') || [];
    } catch {
      zones = [];
    }
    if (zones.length === 0) zones = timezoneFallback;
    const unique = Array.from(new Set(zones)).sort();
    const current = (projectTimezone || '').trim();
    if (current && !unique.includes(current)) unique.unshift(current);
    return unique;
  }, [projectTimezone, timezoneFallback]);

  const filteredTimezones = useMemo(() => {
    const q = timezoneQuery.trim().toLowerCase();
    if (!q) return availableTimezones;
    const filtered = availableTimezones.filter((tz) => tz.toLowerCase().includes(q));
    const current = (projectTimezone || '').trim();
    if (current && !filtered.includes(current)) return [current, ...filtered];
    return filtered;
  }, [availableTimezones, projectTimezone, timezoneQuery]);

  const timezoneGroups = useMemo(() => {
    const groups = new Map<string, string[]>();
    filteredTimezones.forEach((tz) => {
      const parts = tz.split('/');
      const group = parts.length > 1 ? parts[0] : 'Other';
      const list = groups.get(group) || [];
      list.push(tz);
      groups.set(group, list);
    });
    return Array.from(groups.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [filteredTimezones]);

  const recentFiltered = useMemo(() => {
    const q = timezoneQuery.trim().toLowerCase();
    const list = recentTimezones.filter((tz) => availableTimezones.includes(tz));
    if (!q) return list;
    return list.filter((tz) => tz.toLowerCase().includes(q));
  }, [availableTimezones, recentTimezones, timezoneQuery]);

  const getTimezoneOffsetLabel = useMemo(() => {
    const cache = new Map<string, string>();
    return (tz: string) => {
      if (cache.has(tz)) return cache.get(tz) as string;
      try {
        const now = new Date();
        const formatter = new Intl.DateTimeFormat('en-US', {
          timeZone: tz,
          hour12: false,
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
        });
        const parts = formatter.formatToParts(now);
        const partMap: Record<string, string> = {};
        parts.forEach((p) => {
          if (p.type !== 'literal') partMap[p.type] = p.value;
        });
        const asUTC = Date.UTC(
          Number(partMap.year),
          Number(partMap.month) - 1,
          Number(partMap.day),
          Number(partMap.hour),
          Number(partMap.minute),
          Number(partMap.second),
        );
        const offsetMinutes = Math.round((asUTC - now.getTime()) / 60000);
        const sign = offsetMinutes >= 0 ? '+' : '-';
        const abs = Math.abs(offsetMinutes);
        const hh = String(Math.floor(abs / 60)).padStart(2, '0');
        const mm = String(abs % 60).padStart(2, '0');
        const label = `UTC${sign}${hh}:${mm}`;
        cache.set(tz, label);
        return label;
      } catch {
        cache.set(tz, '');
        return '';
      }
    };
  }, []);

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem('obz_recent_timezones');
      if (raw) {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) setRecentTimezones(parsed.filter((v) => typeof v === 'string'));
      }
    } catch {
      /* ignore */
    }
  }, []);

  const updateRecentTimezone = (tz: string) => {
    setRecentTimezones((prev) => {
      const next = [tz, ...prev.filter((v) => v !== tz)].slice(0, 5);
      try {
        window.localStorage.setItem('obz_recent_timezones', JSON.stringify(next));
      } catch {}
      return next;
    });
  };

  const resolvedProjectTimezone = (projectTimezone || project?.timezone || 'UTC').trim() || 'UTC';
  const formatDateTime = (value?: string, tzOverride?: string) =>
    formatDateTimeWithTimezone(value, tzOverride || resolvedProjectTimezone);

  const load = async (force = false) => {
    setLoading(true);
    setError(null);
    try {
      const summary = await authFetchCached<ProjectSummary>(
        `/api/projects/${projectId}/summary`,
        undefined,
        { ttlMs: 15000, bypassCache: force },
      );
      const p = summary?.project || null;
      const d = Array.isArray(summary?.domains) ? summary.domains : [];
      const m = Array.isArray(summary?.members) ? summary.members : [];
      setProject(p);
      setMyRole(summary?.my_role || 'viewer');
      setDomains(d);
      setMembers(m);
      setProjectName(p?.name || '');
      setProjectCountry(p?.target_country || '');
      setProjectLanguage(p?.target_language || '');
      setProjectTimezone(p?.timezone || 'UTC');
      setIndexCheckEnabled(p?.index_check_enabled !== false);
      const edits: Record<string, string> = {};
      const linkDrafts: Record<string, { anchor: string; acceptor: string }> = {};
      d.forEach((item) => {
        edits[item.id] = item.main_keyword || '';
        linkDrafts[item.id] = {
          anchor: item.link_anchor_text || '',
          acceptor: item.link_acceptor_url || '',
        };
      });
      setKeywordEdits(edits);
      setLinkEdits(linkDrafts);
    } catch (err: any) {
      setProject(null);
      setDomains([]);
      setMembers([]);
      setError(err?.message || 'Не удалось загрузить проект');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (projectId) load();
  }, [projectId]);

  const loadProjectErrors = async (force = false) => {
    if (!projectId) return;
    setProjectErrorsLoading(true);
    setProjectErrorsError(null);
    try {
      const list = await authFetchCached<Generation[]>(
        `/api/generations?limit=100&lite=1`,
        undefined,
        { ttlMs: 15000, bypassCache: force },
      );
      const normalized = Array.isArray(list) ? list : [];
      const domainIDs = new Set(domains.map((d) => d.id));
      const errors = normalized
        .filter((g) => g.status === 'error' && g.domain_id && domainIDs.has(g.domain_id))
        .sort(
          (a, b) =>
            new Date((b.updated_at || b.created_at || '') as string).getTime() -
            new Date((a.updated_at || a.created_at || '') as string).getTime(),
        )
        .slice(0, 20);
      setProjectErrors(errors);
    } catch (err: any) {
      setProjectErrorsError(err?.message || 'Не удалось загрузить ошибки');
      setProjectErrors([]);
    } finally {
      setProjectErrorsLoading(false);
    }
  };

  useEffect(() => {
    if (uiView === 'errors') loadProjectErrors();
  }, [uiView, projectId, domains]);

  const saveProjectSettings = async () => {
    if (!projectId) return;
    const name = projectName.trim();
    if (!name) {
      setProjectSettingsError('Название проекта не может быть пустым');
      return;
    }
    setProjectSettingsLoading(true);
    setProjectSettingsError(null);
    try {
      const payload = {
        name,
        country: projectCountry.trim(),
        language: projectLanguage.trim(),
        status: project?.status || 'draft',
        timezone: resolvedProjectTimezone,
      };
      const updated = await authFetch<Project>(`/api/projects/${projectId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      setProject(updated);
      setProjectName(updated.name || '');
      setProjectCountry(updated.target_country || '');
      setProjectLanguage(updated.target_language || '');
      setProjectTimezone(updated.timezone || 'UTC');
      showToast({ type: 'success', title: 'Настройки проекта сохранены', message: updated.name });
    } catch (err: any) {
      setProjectSettingsError(err?.message || 'Ошибка сохранения');
      showToast({ type: 'error', title: 'Ошибка сохранения', message: err?.message });
    } finally {
      setProjectSettingsLoading(false);
    }
  };

  const handleToggleIndexCheck = async (enabled: boolean) => {
    if (!projectId) return;
    setIndexCheckLoading(true);
    try {
      const { setProjectIndexCheckerControl } = await import('@/lib/indexChecksApi');
      await setProjectIndexCheckerControl(projectId, enabled);
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

  const addDomain = async () => {
    if (!url.trim()) return;
    const normalizedDomain = normalizeDomainForImport(url);
    if (!normalizedDomain) {
      setError('Укажите корректный домен.');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains`, {
        url: normalizedDomain,
        keyword,
        country,
        language,
        exclude_domains: exclude,
        generation_type: addGenType,
      });
      setUrl('');
      setKeyword('');
      setExclude('');
      setAddGenType('single_page');
      setIsAddModalOpen(false);
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Не удалось добавить домен');
    } finally {
      setLoading(false);
    }
  };

  const importDomains = async () => {
    if (!importText.trim()) return;
    const parsed = parseDomainImportText(importText);
    if (parsed.errors.length > 0) {
      setError(
        `Импорт остановлен. Проверьте строки: ${parsed.errors
          .slice(0, 3)
          .map((e) => e.line)
          .join(', ')}...`,
      );
      return;
    }
    if (parsed.items.length === 0) {
      setError('Импорт пустой.');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const result = await post<{ created?: number; skipped?: number }>(
        `/api/projects/${projectId}/domains/import`,
        { items: parsed.items },
      );
      showToast({ type: result?.skipped ? 'warning' : 'success', title: 'Импорт завершен' });
      setImportText('');
      setIsImportModalOpen(false);
      await load(true);
      if (runLegacyAfterImport && result?.created) {
        // After bulk import, open legacy import modal for newly added domains
        setIsLegacyImportModalOpen(true);
        setRunLegacyAfterImport(false);
      }
    } catch (err: any) {
      setError(err?.message || 'Не удалось импортировать');
    } finally {
      setLoading(false);
    }
  };

  const updateKeyword = async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/domains/${id}`, { keyword: keywordEdits[id] || '' });
      showToast({ type: 'success', title: 'Сохранено', message: domainById[id]?.url || '' });
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Ошибка');
    } finally {
      setLoading(false);
    }
  };

  const updateLinkSettings = async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      const entry = linkEdits[id] || { anchor: '', acceptor: '' };
      await patch(`/api/domains/${id}`, {
        link_anchor_text: entry.anchor?.trim() || '',
        link_acceptor_url: entry.acceptor?.trim() || '',
      });
      showToast({ type: 'success', title: 'Сохранено', message: domainById[id]?.url || '' });
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Ошибка');
    } finally {
      setLoading(false);
    }
  };

  const deleteProject = async () => {
    if (!confirm('Переместить проект в корзину? Его можно будет восстановить в админ-панели.')) return;
    try {
      await del(`/api/projects/${projectId}`);
      router.push('/projects');
    } catch (err: any) {
      setError(err?.message || 'Ошибка удаления');
    }
  };

  const addMember = async () => {
    if (!newMemberEmail.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/members`, {
        email: newMemberEmail.trim(),
        role: newMemberRole,
      });
      setNewMemberEmail('');
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Ошибка');
      setLoading(false);
    }
  };

  const updateMemberRole = async (email: string, role: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`, { role });
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Ошибка');
      setLoading(false);
    }
  };

  const removeMember = async (email: string) => {
    if (!confirm(`Удалить участника ${email}?`)) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`);
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Ошибка');
      setLoading(false);
    }
  };

  const loadGens = async (id: string) => {
    try {
      const list = await authFetch<Generation[]>(`/api/domains/${id}/generations`);
      setGens((prev) => ({ ...prev, [id]: Array.isArray(list) ? list : [] }));
      setOpenRuns((prev) => ({ ...prev, [id]: !prev[id] }));
    } catch {
      /* ignore */
    }
  };

  const { runGeneration, runLinkTask, removeLinkTask, deleteDomain, generationFlow, linkFlow } =
    useProjectActions({
      projectId,
      project,
      domains,
      domainById,
      keywordEdits,
      linkEdits,
      setLoading,
      setError,
      setLinkLoadingId,
      load,
    });

  const {
    schedulesMultiple,
    scheduleForm,
    setScheduleForm,
    schedulesLoading,
    schedulesError,
    editingSchedule,
    schedules,
    schedulesPermission,
    loadSchedules,
    handleSubmitSchedule,
    handleTriggerSchedule,
    handleToggleSchedule,
    handleEditSchedule,
    handleDeleteSchedule,
    linkScheduleForm,
    setLinkScheduleForm,
    linkScheduleLoading,
    linkScheduleError,
    editingLinkSchedule,
    linkSchedule,
    linkSchedulePermission,
    loadLinkSchedule,
    handleSubmitLinkSchedule,
    handleTriggerLinkSchedule,
    handleToggleLinkSchedule,
    handleEditLinkSchedule,
    handleDeleteLinkSchedule,
  } = useProjectSchedules({ projectId, activeTab, setTab: setActiveTab, resolvedProjectTimezone });

  const filteredDomains = useMemo(() => {
    const term = domainSearch.trim().toLowerCase();
    if (!term) return domains;
    return domains.filter((d) => (d.url || d.id || '').toLowerCase().includes(term));
  }, [domains, domainSearch]);

  const canEditPrompts = canEditPromptOverrides(myRole);

  return (
    <div className="flex flex-col h-full">
      {/* HEADER: Хлебные крошки и табы */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm text-slate-500 dark:text-slate-400 mb-1">
              <Link href="/projects" className="hover:text-indigo-600 transition-colors">
                Проекты
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <span className="text-slate-900 dark:text-slate-200 font-medium">
                {project?.name || 'Загрузка...'}
              </span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              {project?.name}
              {project?.status === 'active' && (
                <span className="bg-emerald-100 text-emerald-700 text-xs px-2.5 py-0.5 rounded-full dark:bg-emerald-900/30 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800">
                  Активен
                </span>
              )}
            </h1>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={() => load(true)}
              title="Обновить"
              className="p-2.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-900 dark:hover:bg-slate-800 transition-colors">
              <RefreshCcw
                className={`w-4 h-4 text-slate-600 dark:text-slate-300 ${loading ? 'animate-spin' : ''}`}
              />
            </button>
            {hasExtendedAccess && (
              <ProjectHeaderActionsSection
                project={project}
                projectId={projectId}
                loading={loading}
                error={error}
                generationFlow={generationFlow}
                linkFlow={linkFlow}
                onRefresh={() => load(true)}
                onDeleteProject={deleteProject}
                onLegacyImport={() => setIsLegacyImportModalOpen(true)}
              />
            )}
          </div>
        </div>

        {hasExtendedAccess && (
          <div className="max-w-7xl mx-auto mt-6 flex flex-wrap items-center gap-6 border-b border-slate-200 dark:border-slate-800">
            <TabButton
              active={uiView === 'sites'}
              onClick={() => setUiView('sites')}
              icon={<LayoutTemplate />}
              label="Сайты"
            />
            <TabButton
              active={uiView === 'seo'}
              onClick={() => setUiView('seo')}
              icon={<Globe />}
              label="Продвинутое SEO"
            />
            <TabButton
              active={uiView === 'schedules'}
              onClick={() => setUiView('schedules')}
              icon={<Clock />}
              label="Автоматизация"
            />
            <TabButton
              active={uiView === 'errors'}
              onClick={() => setUiView('errors')}
              icon={<ShieldAlert />}
              label="Логи и Ошибки"
            />
            <TabButton
              active={uiView === 'settings'}
              onClick={() => setUiView('settings')}
              icon={<Settings />}
              label="Настройки"
            />
          </div>
        )}
      </header>

      {/* CONTENT AREA */}
      <main className="flex-1 overflow-y-auto p-6 bg-slate-50 dark:bg-[#080b13]">
        <div className="max-w-7xl mx-auto space-y-6">
          {error && (
            <div className="p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50 flex items-center gap-2">
              <ShieldAlert className="w-4 h-4" />
              {error}
            </div>
          )}

          {/* VIEW: САЙТЫ (Clean Workspace) */}
          {uiView === 'sites' && (
            <div className="space-y-6 animate-in fade-in duration-300">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div className="relative w-full max-w-md">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                  <input
                    className="w-full pl-10 pr-4 py-2.5 rounded-xl border border-slate-300 bg-white text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none dark:bg-slate-900 dark:border-slate-700 dark:text-slate-100 transition-all shadow-sm"
                    placeholder="Поиск по домену..."
                    value={domainSearch}
                    onChange={(e) => setDomainSearch(e.target.value)}
                  />
                </div>
                {hasExtendedAccess && (
                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => setIsImportModalOpen(true)}
                      className="px-4 py-2.5 text-sm font-medium bg-white border border-slate-300 rounded-xl hover:bg-slate-50 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200 transition-colors shadow-sm">
                      <UploadCloud className="w-4 h-4 inline-block mr-2 text-slate-500" /> Импорт
                      списка
                    </button>
                    <button
                      onClick={() => setIsAddModalOpen(true)}
                      className="px-4 py-2.5 text-sm font-medium bg-indigo-600 text-white rounded-xl hover:bg-indigo-500 transition-colors shadow-sm">
                      <Plus className="w-4 h-4 inline-block mr-1" /> Добавить сайт
                    </button>
                  </div>
                )}
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-5">
                {filteredDomains.map((d) => (
                  <div
                    key={d.id}
                    className="group bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl p-5 shadow-sm hover:shadow-md hover:border-indigo-300 dark:hover:border-indigo-500/50 transition-all flex flex-col justify-between h-50">
                    <div>
                      <div className="flex items-start justify-between mb-3">
                        <div className="w-10 h-10 rounded-xl bg-indigo-50 dark:bg-indigo-900/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400 flex-shrink-0">
                          <Globe className="w-5 h-5" />
                        </div>
                        <ProjectStatusBadge status={d.status} />
                      </div>
                      <h3
                        className="font-semibold text-slate-900 dark:text-white truncate"
                        title={d.url}>
                        {d.url}
                      </h3>
                      <p className="text-xs font-medium text-slate-500 dark:text-slate-400 mt-1.5 pb-2 truncate">
                        {d.target_country ? (
                          <span className="bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md mr-1">
                            {d.target_country}
                          </span>
                        ) : null}
                        {d.target_language ? (
                          <span className="bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
                            {d.target_language}
                          </span>
                        ) : null}
                      </p>
                    </div>

                    <div className="flex items-center gap-2 pt-4 border-t border-slate-100 dark:border-slate-800">
                      <Link
                        href={`/domains/${d.id}/editor`}
                        className="flex-1 bg-indigo-600 hover:bg-indigo-500 text-white text-center py-2.5 rounded-xl text-sm font-medium transition-colors shadow-sm">
                        Редактор
                      </Link>
                      {hasExtendedAccess && (
                        <Link
                          href={`/domains/${d.id}`}
                          className="p-2.5 border border-slate-200 rounded-xl text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800 transition-colors"
                          title="Настройки SEO и Генерации">
                          <Settings className="w-4 h-4" />
                        </Link>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* VIEW: ПРОДВИНУТОЕ SEO (Компактная таблица) */}
          {uiView === 'seo' && hasExtendedAccess && (
            <div className="animate-in fade-in duration-300">
              <ProjectDomainsSection
                loading={loading}
                domainSearch={domainSearch}
                domainsCount={domains.length}
                filteredDomains={filteredDomains}
                linkLoadingId={linkLoadingId}
                openRuns={openRuns}
                gens={gens}
                keywordEdits={keywordEdits}
                linkEdits={linkEdits}
                onDomainSearchChange={setDomainSearch}
                onRunLinkTask={runLinkTask}
                onRemoveLinkTask={removeLinkTask}
                onLoadRuns={loadGens}
                onDeleteDomain={deleteDomain}
                onKeywordEditChange={(domainId, value) =>
                  setKeywordEdits((p) => ({ ...p, [domainId]: value }))
                }
                onUpdateKeyword={updateKeyword}
                onLinkEditChange={(domainId, value) =>
                  setLinkEdits((p) => ({ ...p, [domainId]: value }))
                }
                onUpdateLinkSettings={updateLinkSettings}
                getEffectiveLinkStatus={getEffectiveDomainLinkStatus}
                renderStatusBadge={(status) => <ProjectStatusBadge status={status} />}
                renderLinkStatusBadge={(domain) => (
                  <ProjectLinkStatusBadge domain={domain as any} />
                )}
                renderRunsList={(runs) => <ProjectRunsList runs={runs as Generation[]} />}
                formatDateTime={formatDateTime}
                formatRelativeTime={formatRelativeTime}
              />
            </div>
          )}

          {/* Остальные табы прокидывают старые компоненты (Автоматизация, Настройки, Ошибки) */}
          {uiView === 'schedules' && hasExtendedAccess && (
            <div className="animate-in fade-in duration-300">
              <ProjectSchedulesSection
                schedulesMultiple={schedulesMultiple}
                scheduleForm={scheduleForm}
                schedulesLoading={schedulesLoading}
                schedulesError={schedulesError}
                editingSchedule={editingSchedule}
                resolvedProjectTimezone={resolvedProjectTimezone}
                schedules={schedules}
                schedulesPermission={schedulesPermission}
                onScheduleFormChange={setScheduleForm}
                onSubmitSchedule={handleSubmitSchedule}
                onRefreshSchedules={loadSchedules}
                onTriggerSchedule={handleTriggerSchedule}
                onToggleSchedule={handleToggleSchedule}
                onEditSchedule={handleEditSchedule}
                onDeleteSchedule={handleDeleteSchedule}
                linkScheduleForm={linkScheduleForm}
                linkScheduleLoading={linkScheduleLoading}
                linkScheduleError={linkScheduleError}
                editingLinkSchedule={editingLinkSchedule}
                linkSchedule={linkSchedule}
                linkSchedulePermission={linkSchedulePermission}
                onLinkScheduleFormChange={setLinkScheduleForm}
                onSubmitLinkSchedule={handleSubmitLinkSchedule}
                onRefreshLinkSchedule={loadLinkSchedule}
                onTriggerLinkSchedule={handleTriggerLinkSchedule}
                onToggleLinkSchedule={handleToggleLinkSchedule}
                onEditLinkSchedule={handleEditLinkSchedule}
                onDeleteLinkSchedule={handleDeleteLinkSchedule}
              />
            </div>
          )}
          {uiView === 'settings' && hasExtendedAccess && (
            <div className="animate-in fade-in duration-300">
              <ProjectSettingsSection
                projectId={projectId}
                projectSettingsLoading={projectSettingsLoading}
                projectSettingsError={projectSettingsError}
                projectName={projectName}
                projectCountry={projectCountry}
                projectLanguage={projectLanguage}
                timezoneQuery={timezoneQuery}
                resolvedProjectTimezone={resolvedProjectTimezone}
                projectTimezone={projectTimezone}
                recentFiltered={recentFiltered}
                timezoneGroups={timezoneGroups}
                canEditPrompts={canEditPrompts}
                members={members}
                loading={loading}
                newMemberEmail={newMemberEmail}
                newMemberRole={newMemberRole}
                getTimezoneOffsetLabel={getTimezoneOffsetLabel}
                formatDateTime={formatDateTime}
                onSaveProjectSettings={saveProjectSettings}
                onProjectNameChange={setProjectName}
                onProjectCountryChange={setProjectCountry}
                onProjectLanguageChange={setProjectLanguage}
                onTimezoneQueryChange={setTimezoneQuery}
                onProjectTimezoneChange={(v) => {
                  setProjectTimezone(v);
                  updateRecentTimezone(v);
                }}
                onUseRecentTimezone={(v) => {
                  setProjectTimezone(v);
                  updateRecentTimezone(v);
                }}
                onNewMemberEmailChange={setNewMemberEmail}
                onNewMemberRoleChange={setNewMemberRole}
                onAddMember={addMember}
                onUpdateMemberRole={updateMemberRole}
                onRemoveMember={removeMember}
                indexCheckEnabled={indexCheckEnabled}
                indexCheckLoading={indexCheckLoading}
                onToggleIndexCheck={handleToggleIndexCheck}
              />
            </div>
          )}
          {uiView === 'errors' && hasExtendedAccess && (
            <div className="animate-in fade-in duration-300">
              <ProjectDiagnosticsSection
                loading={projectErrorsLoading}
                error={projectErrorsError}
                items={projectErrors}
                domainById={domainById}
                formatDateTime={formatDateTime}
                onRefresh={() => loadProjectErrors(true)}
                onRetry={runGeneration}
              />
            </div>
          )}
        </div>
      </main>

      {/* МОДАЛКА: ДОБАВИТЬ САЙТ (Улучшенные контрасты инпутов) */}
      {isAddModalOpen && hasExtendedAccess && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
          <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-800/60 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/20">
              <h3 className="text-lg font-bold text-slate-900 dark:text-white">Добавить сайт</h3>
              <button
                onClick={() => setIsAddModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="p-6 space-y-5">
              <div>
                <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
                  URL домена <span className="text-red-500">*</span>
                </label>
                <input
                  autoFocus
                  className="w-full bg-white dark:bg-slate-900 border border-slate-300 dark:border-slate-700 rounded-xl px-4 py-2.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:text-white placeholder:text-slate-400 dark:placeholder:text-slate-500 transition-all"
                  placeholder="example.com"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
                  Ключевое слово (Main Keyword)
                </label>
                <input
                  className="w-full bg-white dark:bg-slate-900 border border-slate-300 dark:border-slate-700 rounded-xl px-4 py-2.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:text-white placeholder:text-slate-400 dark:placeholder:text-slate-500 transition-all"
                  placeholder="Например: best running shoes"
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
                  Тип генерации
                </label>
                <select
                  className="w-full bg-white dark:bg-slate-900 border border-slate-300 dark:border-slate-700 rounded-xl px-4 py-2.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:text-white transition-all"
                  value={addGenType}
                  onChange={(e) => setAddGenType(e.target.value)}>
                  {(
                    Object.entries(GENERATION_TYPES) as [
                      GenerationType,
                      { label: string; available: boolean },
                    ][]
                  ).map(([key, { label, available }]) => (
                    <option key={key} value={key} disabled={!available}>
                      {label}
                      {!available ? ' (Скоро)' : ''}
                    </option>
                  ))}
                </select>
              </div>
            </div>
            <div className="px-6 py-4 bg-slate-50/50 dark:bg-slate-800/20 border-t border-slate-100 dark:border-slate-800/60 flex justify-end gap-3">
              <button
                onClick={() => setIsAddModalOpen(false)}
                className="px-5 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                Отмена
              </button>
              <button
                onClick={addDomain}
                disabled={loading || !url}
                className="px-6 py-2.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-xl disabled:opacity-50 transition-all shadow-sm">
                Добавить
              </button>
            </div>
          </div>
        </div>
      )}

      {/* МОДАЛКА: ИМПОРТ */}
      {isImportModalOpen && hasExtendedAccess && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
          <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl w-full max-w-3xl overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-800/60 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/20">
              <h3 className="text-lg font-bold text-slate-900 dark:text-white">
                Массовый импорт сайтов
              </h3>
              <button
                onClick={() => setIsImportModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors">
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-6">
              {/* Блок с легаси-подсказкой */}
              <div className="bg-indigo-50/50 dark:bg-indigo-900/10 rounded-xl p-4 mb-5 border border-indigo-100 dark:border-indigo-800/30 space-y-2">
                <p className="text-sm text-slate-700 dark:text-slate-300 leading-relaxed">
                  Импорт по строкам в формате: <br />
                  {/* <code className="inline-block mt-1 px-1.5 py-0.5 rounded bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-indigo-600 dark:text-indigo-400 font-mono text-xs">
                    url[,keyword[,country[,language[,server_id[,anchor[,acceptor[,link_placed[,generation_type]]]]]]]]
                  </code> */}
                </p>
                <p className="text-sm text-slate-700 dark:text-slate-300 leading-relaxed">
                  Пример: <br />
                  <code className="inline-block mt-1 px-1.5 py-0.5 rounded bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-indigo-600 dark:text-indigo-400 font-mono text-xs">
                    example.ru,casino,ru,ru,seotech-web-media1,"Лучший
                    бонус","https://acceptor.example/page"
                    <br/>
                    example2.com,casino,ru,ru,seotech-web-media1,"Лучший
                    бонус","https://acceptor.example/page"
                  </code>
                </p>
                {/* <p className="text-xs text-slate-500 dark:text-slate-400 mt-2">
                  * Проверка существования домена на сервере `media1` будет добавлена на этапе
                  серверной интеграции.
                </p> */}
              </div>

              <textarea
                className="w-full bg-white dark:bg-slate-900 border border-slate-300 dark:border-slate-700 rounded-xl p-4 text-sm font-mono h-48 outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600 transition-all resize-none shadow-inner"
                placeholder={
                  'example.com,casino,se,sv,seotech-web-media1,"Лучший бонус","https://acceptor.example/page"\nexample.org'
                }
                value={importText}
                onChange={(e) => setImportText(e.target.value)}
              />

              <label className="flex items-center gap-2 mt-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={runLegacyAfterImport}
                  onChange={(e) => setRunLegacyAfterImport(e.target.checked)}
                  className="rounded border-slate-300 dark:border-slate-600 text-indigo-500 focus:ring-indigo-500/20"
                />
                <span className="text-sm text-slate-600 dark:text-slate-300">
                  Запустить Legacy Import (синхр. файлов, артефакты, ссылки)
                </span>
              </label>
            </div>

            <div className="px-6 py-4 bg-slate-50/50 dark:bg-slate-800/20 border-t border-slate-100 dark:border-slate-800/60 flex justify-end gap-3">
              <button
                onClick={() => setIsImportModalOpen(false)}
                className="px-5 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                Отмена
              </button>
              <button
                onClick={importDomains}
                disabled={loading || !importText.trim()}
                className="px-6 py-2.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-xl disabled:opacity-50 transition-all shadow-sm active:scale-95">
                Импортировать
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Legacy Import Modal */}
      {isLegacyImportModalOpen && hasExtendedAccess && (
        <LegacyImportModal
          projectId={projectId}
          domains={domains.map((d) => ({
            id: d.id,
            url: d.url,
            deployment_mode: d.deployment_mode ?? null,
            server_id: d.server_id ?? null,
          }))}
          onClose={() => setIsLegacyImportModalOpen(false)}
          onStart={(domainIds, force) => {
            setIsLegacyImportModalOpen(false);
            setShowLegacyProgress(true);
            legacyImport.startImport(domainIds, force);
          }}
        />
      )}

      {/* Legacy Import Progress */}
      {showLegacyProgress && legacyImport.activeJob && (
        <LegacyImportProgress
          job={legacyImport.activeJob}
          items={legacyImport.items}
          onClose={() => {
            setShowLegacyProgress(false);
            if (!legacyImport.isImporting) {
              load(true);
            }
          }}
        />
      )}
    </div>
  );
}

function TabButton({ active, icon, label, onClick }: any) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-2 pb-4 px-1 text-sm font-medium border-b-2 transition-all ${
        active
          ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400'
          : 'border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'
      }`}>
      {icon && <span className="flex-shrink-0 opacity-80 [&>svg]:w-4 [&>svg]:h-4">{icon}</span>}
      {label}
    </button>
  );
}
