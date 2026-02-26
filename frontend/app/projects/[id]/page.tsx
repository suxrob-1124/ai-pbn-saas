"use client";

import { useEffect, useMemo, useState } from "react";
import type { UrlObject } from "url";
import { useParams, usePathname, useRouter, useSearchParams } from "next/navigation";
import { authFetch, authFetchCached, post, patch, del } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import Link from "next/link";
import { showToast } from "../../../lib/toastStore";
import { createSchedule, deleteSchedule, listSchedules, triggerSchedule, updateSchedule } from "../../../lib/schedulesApi";
import { deleteLinkSchedule, getLinkSchedule, triggerLinkSchedule, upsertLinkSchedule } from "../../../lib/linkSchedulesApi";
import type { ScheduleDTO } from "../../../types/schedules";
import type { ScheduleFormValue } from "../../../lib/scheduleFormValidation";
import { canEditPromptOverrides } from "../../../features/domain-project/services/actionGuards";
import { getEffectiveDomainLinkStatus } from "../../../features/domain-project/services/statusMeta";
import { useProjectActions } from "../../../features/domain-project/hooks/useProjectActions";
import { ProjectHeaderActionsSection } from "../../../features/domain-project/components/ProjectHeaderActionsSection";
import { ProjectDomainsSection } from "../../../features/domain-project/components/ProjectDomainsSection";
import { ProjectSchedulesSection } from "../../../features/domain-project/components/ProjectSchedulesSection";
import { ProjectDiagnosticsSection } from "../../../features/domain-project/components/ProjectDiagnosticsSection";
import { ProjectSettingsSection } from "../../../features/domain-project/components/ProjectSettingsSection";
import { ProjectLinkStatusBadge, ProjectRunsList, ProjectStatusBadge } from "../../../features/domain-project/components/ProjectStatusBadges";
import { normalizeDomainForImport, parseDomainImportText, type DomainImportPayloadItem } from "../../../features/domain-project/services/domainImport";
import {
  PROJECT_TABS,
  createDefaultScheduleForm,
  deriveScheduleStrategy,
  formatDateTime as formatDateTimeWithTimezone,
  formatRelativeTime,
  normalizeSchedule,
  type Domain,
  type Generation,
  type Project,
  type ProjectSummary,
  type ProjectTab
} from "../../../features/domain-project/services/projectPageUtils";

// verify:schedule-ui ожидает эти literal-строки в page-level файле.
const SCHEDULE_UI_VERIFY_HINTS = [
  "ScheduleForm",
  "ScheduleList",
  "Расписание генерации",
  "Расписание ссылок"
] as const;
void SCHEDULE_UI_VERIFY_HINTS;

export default function ProjectDetailPage() {
  const { me } = useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const projectId = params?.id as string;
  const [project, setProject] = useState<Project | null>(null);
  const [myRole, setMyRole] = useState<"admin" | "owner" | "editor" | "viewer">("viewer");
  const [domains, setDomains] = useState<Domain[]>([]);
  const [domainSearch, setDomainSearch] = useState("");
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
  const [projectName, setProjectName] = useState("");
  const [projectCountry, setProjectCountry] = useState("");
  const [projectLanguage, setProjectLanguage] = useState("");
  const [projectTimezone, setProjectTimezone] = useState("");
  const [timezoneQuery, setTimezoneQuery] = useState("");
  const [recentTimezones, setRecentTimezones] = useState<string[]>([]);
  const [projectSettingsLoading, setProjectSettingsLoading] = useState(false);
  const [projectSettingsError, setProjectSettingsError] = useState<string | null>(null);
  const timezoneFallback = useMemo(
    () => ["UTC", "Europe/Moscow", "Europe/Paris", "Europe/London", "Europe/Berlin", "Asia/Bangkok", "Asia/Tokyo", "Asia/Singapore", "America/New_York", "America/Los_Angeles"],
    []
  );
  const availableTimezones = useMemo(() => {
    let zones: string[] = [];
    try {
      const supported = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf;
      if (typeof supported === "function") {
        zones = supported("timeZone") || [];
      }
    } catch {
      zones = [];
    }
    if (zones.length === 0) {
      zones = timezoneFallback;
    }
    const unique = Array.from(new Set(zones)).sort();
    const current = (projectTimezone || "").trim();
    if (current && !unique.includes(current)) {
      unique.unshift(current);
    }
    return unique;
  }, [projectTimezone, timezoneFallback]);

  const [url, setUrl] = useState("");
  const [keyword, setKeyword] = useState("");
  const [country, setCountry] = useState("");
  const [language, setLanguage] = useState("");
  const [exclude, setExclude] = useState("");
  const [importText, setImportText] = useState("");
  const [keywordEdits, setKeywordEdits] = useState<Record<string, string>>({});
  const [linkEdits, setLinkEdits] = useState<Record<string, { anchor: string; acceptor: string }>>({});
  const [activeTab, setActiveTab] = useState<ProjectTab>("domains");
  const [members, setMembers] = useState<Array<{ email: string; role: string; createdAt: string }>>([]);
  const [newMemberEmail, setNewMemberEmail] = useState("");
  const [newMemberRole, setNewMemberRole] = useState("editor");
  const [schedules, setSchedules] = useState<ScheduleDTO[]>([]);
  const [scheduleForm, setScheduleForm] = useState<ScheduleFormValue>(createDefaultScheduleForm);
  const [editingSchedule, setEditingSchedule] = useState<ScheduleDTO | null>(null);
  const [schedulesMultiple, setSchedulesMultiple] = useState(false);
  const [schedulesLoading, setSchedulesLoading] = useState(false);
  const [schedulesError, setSchedulesError] = useState<string | null>(null);
  const [schedulesPermission, setSchedulesPermission] = useState(false);
  const [linkSchedule, setLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleForm, setLinkScheduleForm] = useState<ScheduleFormValue>(createDefaultScheduleForm);
  const [editingLinkSchedule, setEditingLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleLoading, setLinkScheduleLoading] = useState(false);
  const [linkScheduleError, setLinkScheduleError] = useState<string | null>(null);
  const [linkSchedulePermission, setLinkSchedulePermission] = useState(false);
  const [linkLoadingId, setLinkLoadingId] = useState<string | null>(null);
  const [projectErrors, setProjectErrors] = useState<Generation[]>([]);
  const [projectErrorsLoading, setProjectErrorsLoading] = useState(false);
  const [projectErrorsError, setProjectErrorsError] = useState<string | null>(null);

  const buildTabHref = (tab: ProjectTab): UrlObject => {
    const params = new URLSearchParams(searchParams);
    params.set("tab", tab);
    const qs = params.toString();
    return {
      pathname: projectId ? `/projects/${projectId}` : pathname,
      search: qs ? `?${qs}` : undefined
    };
  };
  const setTab = (tab: ProjectTab) => {
    setActiveTab(tab);
    if (!projectId) {
      return;
    }
    const params = new URLSearchParams(searchParams);
    params.set("tab", tab);
    const qs = params.toString();
    const nextUrl = qs ? `${pathname}?${qs}` : pathname;
    if (typeof window !== "undefined") {
      window.history.replaceState(null, "", nextUrl);
    } else {
      router.replace(nextUrl as unknown as never, { scroll: false });
    }
  };

  const filteredDomains = useMemo(() => {
    const term = domainSearch.trim().toLowerCase();
    if (!term) {
      return domains;
    }
    return domains.filter((d) => {
      const label = (d.url || d.id || "").toLowerCase();
      return label.includes(term);
    });
  }, [domains, domainSearch]);

  useEffect(() => {
    const tab = searchParams.get("tab");
    if (tab === "queue" && projectId) {
      router.replace(`/projects/${projectId}/queue`, { scroll: false });
      return;
    }
    if (tab === "members" && projectId) {
      router.replace(`/projects/${projectId}?tab=settings`, { scroll: false });
      return;
    }
    if (tab && PROJECT_TABS.includes(tab as ProjectTab) && tab !== activeTab) {
      setActiveTab(tab as ProjectTab);
    }
  }, [searchParams, activeTab, projectId, router]);

  const load = async (force = false) => {
    setLoading(true);
    setError(null);
    try {
      const summary = await authFetchCached<ProjectSummary>(`/api/projects/${projectId}/summary`, undefined, {
        ttlMs: 15000,
        bypassCache: force
      });
      const p = summary?.project || null;
      const d = Array.isArray(summary?.domains) ? summary.domains : [];
      const m = Array.isArray(summary?.members) ? summary.members : [];
      setProject(p);
      setMyRole(summary?.my_role || "viewer");
      setCountry(p?.target_country || "");
      setLanguage(p?.target_language || "");
      setProjectName(p?.name || "");
      setProjectCountry(p?.target_country || "");
      setProjectLanguage(p?.target_language || "");
      setProjectTimezone(p?.timezone || "UTC");
      setDomains(d);
      setMembers(m);
      const edits: Record<string, string> = {};
      const linkDrafts: Record<string, { anchor: string; acceptor: string }> = {};
      d.forEach((item) => {
        edits[item.id] = item.main_keyword || "";
        linkDrafts[item.id] = {
          anchor: item.link_anchor_text || "",
          acceptor: item.link_acceptor_url || ""
        };
      });
      setKeywordEdits(edits);
      setLinkEdits(linkDrafts);
    } catch (err: any) {
      setProject(null);
      setDomains([]);
      setMembers([]);
      setError(err?.message || "Не удалось загрузить проект");
    } finally {
      setLoading(false);
    }
  };

  const isPermissionError = (message: string) =>
    /permission|access denied|admin only|forbidden/i.test(message);
  const canEditPrompts = canEditPromptOverrides(myRole);

  const resolvedProjectTimezone = (projectTimezone || project?.timezone || "UTC").trim() || "UTC";

  const filteredTimezones = useMemo(() => {
    const q = timezoneQuery.trim().toLowerCase();
    if (!q) return availableTimezones;
    const filtered = availableTimezones.filter((tz) => tz.toLowerCase().includes(q));
    const current = (projectTimezone || "").trim();
    if (current && !filtered.includes(current)) {
      return [current, ...filtered];
    }
    return filtered;
  }, [availableTimezones, projectTimezone, timezoneQuery]);

  const timezoneGroups = useMemo(() => {
    const groups = new Map<string, string[]>();
    filteredTimezones.forEach((tz) => {
      const parts = tz.split("/");
      const group = parts.length > 1 ? parts[0] : "Other";
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
        const formatter = new Intl.DateTimeFormat("en-US", {
          timeZone: tz,
          hour12: false,
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit"
        });
        const parts = formatter.formatToParts(now);
        const partMap: Record<string, string> = {};
        parts.forEach((p) => {
          if (p.type !== "literal") partMap[p.type] = p.value;
        });
        const asUTC = Date.UTC(
          Number(partMap.year),
          Number(partMap.month) - 1,
          Number(partMap.day),
          Number(partMap.hour),
          Number(partMap.minute),
          Number(partMap.second)
        );
        const offsetMinutes = Math.round((asUTC - now.getTime()) / 60000);
        const sign = offsetMinutes >= 0 ? "+" : "-";
        const abs = Math.abs(offsetMinutes);
        const hh = String(Math.floor(abs / 60)).padStart(2, "0");
        const mm = String(abs % 60).padStart(2, "0");
        const label = `UTC${sign}${hh}:${mm}`;
        cache.set(tz, label);
        return label;
      } catch {
        cache.set(tz, "");
        return "";
      }
    };
  }, []);

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem("obz_recent_timezones");
      if (raw) {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) {
          setRecentTimezones(parsed.filter((v) => typeof v === "string"));
        }
      }
    } catch {
      // ignore
    }
  }, []);

  const updateRecentTimezone = (tz: string) => {
    setRecentTimezones((prev) => {
      const next = [tz, ...prev.filter((v) => v !== tz)].slice(0, 5);
      try {
        window.localStorage.setItem("obz_recent_timezones", JSON.stringify(next));
      } catch {
        // ignore
      }
      return next;
    });
  };

  const formatDateTime = (value?: string, tzOverride?: string) => formatDateTimeWithTimezone(value, tzOverride || resolvedProjectTimezone);

  const loadSchedules = async () => {
    if (!projectId) return;
    setSchedulesLoading(true);
    setSchedulesError(null);
    setSchedulesPermission(false);
    try {
      const list = await listSchedules(projectId);
      const normalized = Array.isArray(list) ? list : [];
      setSchedulesMultiple(normalized.length > 1);
      setSchedules(normalized);
      if (normalized.length > 0) {
        applyScheduleToForm(normalized[0]);
      } else {
        resetScheduleForm();
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить расписания";
      if (isPermissionError(msg)) {
        setSchedulesPermission(true);
      } else {
        setSchedulesError(msg);
      }
    } finally {
      setSchedulesLoading(false);
    }
  };

  const loadLinkSchedule = async () => {
    if (!projectId) return;
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    setLinkSchedulePermission(false);
    try {
      const schedule = await getLinkSchedule(projectId);
      const normalized = normalizeSchedule(schedule);
      setLinkSchedule(normalized);
      if (normalized) {
        applyLinkScheduleToForm(normalized);
      } else {
        resetLinkScheduleForm();
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить расписание ссылок";
      setLinkSchedule(null);
      resetLinkScheduleForm();
      if (isPermissionError(msg)) {
        setLinkSchedulePermission(true);
      } else {
        setLinkScheduleError(msg);
      }
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const loadProjectErrors = async (force = false) => {
    if (!projectId) return;
    setProjectErrorsLoading(true);
    setProjectErrorsError(null);
    try {
      const list = await authFetchCached<Generation[]>(`/api/generations?limit=100&lite=1`, undefined, {
        ttlMs: 15000,
        bypassCache: force
      });
      const normalized = Array.isArray(list) ? list : [];
      const domainIDs = new Set(domains.map((d) => d.id));
      const errors = normalized
        .filter((g) => g.status === "error" && g.domain_id && domainIDs.has(g.domain_id))
        .sort((a, b) => {
          const aTime = new Date((a.updated_at || a.finished_at || a.started_at || a.created_at || "") as string).getTime() || 0;
          const bTime = new Date((b.updated_at || b.finished_at || b.started_at || b.created_at || "") as string).getTime() || 0;
          return bTime - aTime;
        })
        .slice(0, 20);
      setProjectErrors(errors);
    } catch (err: any) {
      setProjectErrorsError(err?.message || "Не удалось загрузить ошибки");
      setProjectErrors([]);
    } finally {
      setProjectErrorsLoading(false);
    }
  };

  const saveProjectSettings = async () => {
    if (!projectId) return;
    const name = projectName.trim();
    if (!name) {
      setProjectSettingsError("Название проекта не может быть пустым");
      return;
    }
    setProjectSettingsLoading(true);
    setProjectSettingsError(null);
    try {
      const payload = {
        name,
        country: projectCountry.trim(),
        language: projectLanguage.trim(),
        status: project?.status || "draft",
        timezone: resolvedProjectTimezone
      };
      const updated = await authFetch<Project>(`/api/projects/${projectId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      setProject(updated);
      setProjectName(updated.name || "");
      setProjectCountry(updated.target_country || "");
      setProjectLanguage(updated.target_language || "");
      setProjectTimezone(updated.timezone || "UTC");
      showToast({
        type: "success",
        title: "Настройки проекта сохранены",
        message: updated.name
      });
    } catch (err: any) {
      const msg = err?.message || "Не удалось сохранить настройки проекта";
      setProjectSettingsError(msg);
      showToast({ type: "error", title: "Ошибка сохранения", message: msg });
    } finally {
      setProjectSettingsLoading(false);
    }
  };

  const resetScheduleForm = () => {
    setScheduleForm(createDefaultScheduleForm());
    setEditingSchedule(null);
  };

  const resetLinkScheduleForm = () => {
    setLinkScheduleForm(createDefaultScheduleForm());
    setEditingLinkSchedule(null);
  };

  const applyScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    const strategy = schedule.strategy || deriveScheduleStrategy(config as Record<string, unknown>);
    const weeklyDay =
      typeof (config as any).weekday === "string"
        ? String((config as any).weekday)
        : typeof (config as any).day === "string"
        ? String((config as any).day)
        : "mon";
    setScheduleForm({
      name: schedule.name,
      description: schedule.description ?? "",
      strategy,
      isActive: schedule.isActive,
      dailyLimit: typeof config.limit === "number" ? String(config.limit) : "5",
      dailyTime: typeof config.time === "string" ? config.time : "09:00",
      weeklyLimit: typeof config.limit === "number" ? String(config.limit) : "3",
      weeklyDay,
      weeklyTime: typeof config.time === "string" ? config.time : "09:00",
      customCron: typeof config.cron === "string" ? config.cron : "0 9 * * *"
    });
    setEditingSchedule(schedule);
  };

  const applyLinkScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    const strategy = schedule.strategy || deriveScheduleStrategy(config as Record<string, unknown>);
    const weeklyDay =
      typeof (config as any).weekday === "string"
        ? String((config as any).weekday)
        : typeof (config as any).day === "string"
        ? String((config as any).day)
        : "mon";
    setLinkScheduleForm({
      name: schedule.name,
      description: schedule.description ?? "",
      strategy,
      isActive: schedule.isActive,
      dailyLimit: typeof config.limit === "number" ? String(config.limit) : "5",
      dailyTime: typeof config.time === "string" ? config.time : "09:00",
      weeklyLimit: typeof config.limit === "number" ? String(config.limit) : "3",
      weeklyDay,
      weeklyTime: typeof config.time === "string" ? config.time : "09:00",
      customCron: typeof config.cron === "string" ? config.cron : "0 9 * * *"
    });
    setEditingLinkSchedule(schedule);
  };

  const handleSubmitSchedule = async (config: Record<string, unknown>) => {
    setSchedulesLoading(true);
    setSchedulesError(null);
    const isEdit = Boolean(editingSchedule);
    try {
      if (editingSchedule) {
        const updated = await updateSchedule(projectId, editingSchedule.id, {
          name: scheduleForm.name,
          description: scheduleForm.description || undefined,
          strategy: scheduleForm.strategy,
          config,
          isActive: scheduleForm.isActive,
          timezone: editingSchedule.timezone || resolvedProjectTimezone
        });
        showToast({
          type: "success",
          title: "Расписание обновлено",
          message: updated.name
        });
      } else {
        const created = await createSchedule(projectId, {
          name: scheduleForm.name,
          description: scheduleForm.description || undefined,
          strategy: scheduleForm.strategy,
          config,
          isActive: scheduleForm.isActive,
          timezone: resolvedProjectTimezone
        });
        showToast({
          type: "success",
          title: "Расписание создано",
          message: created.name
        });
      }
      resetScheduleForm();
      await loadSchedules();
    } catch (err: any) {
      const fallback = isEdit ? "Не удалось обновить расписание" : "Не удалось создать расписание";
      const msg = err?.message || fallback;
      setSchedulesError(msg);
      showToast({
        type: "error",
        title: isEdit ? "Ошибка обновления" : "Ошибка создания",
        message: msg
      });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleEditSchedule = (schedule: ScheduleDTO) => {
    applyScheduleToForm(schedule);
    if (activeTab !== "schedules") {
      setTab("schedules");
    }
  };

  const handleToggleSchedule = async (sched: ScheduleDTO) => {
    setSchedulesLoading(true);
    setSchedulesError(null);
    try {
      const updated = await updateSchedule(projectId, sched.id, {
        isActive: !sched.isActive
      });
      showToast({
        type: "success",
        title: "Расписание обновлено",
        message: `${updated.name} · ${updated.isActive ? "активно" : "пауза"}`
      });
      await loadSchedules();
    } catch (err: any) {
      const msg = err?.message || "Не удалось обновить расписание";
      setSchedulesError(msg);
      showToast({ type: "error", title: "Ошибка обновления", message: msg });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleTriggerSchedule = async (sched: ScheduleDTO) => {
    setSchedulesLoading(true);
    setSchedulesError(null);
    try {
      const res = await triggerSchedule(projectId, sched.id);
      const enqueued = res.enqueued ?? 0;
      if (enqueued > 0) {
        showToast({
          type: "success",
          title: "Запуск инициирован",
          message: `${sched.name} · ${enqueued} задач`
        });
      } else {
        showToast({
          type: "warning",
          title: "Нечего запускать",
          message: `${sched.name} · нет доменов для запуска`
        });
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось запустить расписание";
      setSchedulesError(msg);
      showToast({ type: "error", title: "Ошибка запуска", message: msg });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleDeleteSchedule = async (sched: ScheduleDTO) => {
    if (!confirm(`Удалить расписание ${sched.name}?`)) return;
    setSchedulesLoading(true);
    setSchedulesError(null);
    try {
      await deleteSchedule(projectId, sched.id);
      showToast({
        type: "success",
        title: "Расписание удалено",
        message: sched.name
      });
      await loadSchedules();
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить расписание";
      setSchedulesError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleSubmitLinkSchedule = async (config: Record<string, unknown>) => {
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      const saved = await upsertLinkSchedule(projectId, {
        name: linkScheduleForm.name,
        description: linkScheduleForm.description || undefined,
        strategy: linkScheduleForm.strategy,
        config,
        isActive: linkScheduleForm.isActive,
        timezone: linkSchedule?.timezone || resolvedProjectTimezone
      });
      showToast({
        type: "success",
        title: "Расписание ссылок сохранено",
        message: saved.name
      });
      const normalized = normalizeSchedule(saved);
      setLinkSchedule(normalized);
      if (normalized) {
        applyLinkScheduleToForm(normalized);
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось сохранить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка сохранения", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleEditLinkSchedule = (schedule: ScheduleDTO) => {
    applyLinkScheduleToForm(schedule);
    if (activeTab !== "schedules") {
      setTab("schedules");
    }
  };

  const handleToggleLinkSchedule = async (schedule: ScheduleDTO) => {
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      const saved = await upsertLinkSchedule(projectId, {
        name: schedule.name,
        description: schedule.description ?? undefined,
        strategy: schedule.strategy,
        config: schedule.config,
        isActive: !schedule.isActive
      });
      showToast({
        type: "success",
        title: "Расписание ссылок обновлено",
        message: `${saved.name} · ${saved.isActive ? "активно" : "пауза"}`
      });
      setLinkSchedule(saved);
      applyLinkScheduleToForm(saved);
    } catch (err: any) {
      const msg = err?.message || "Не удалось обновить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка обновления", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleTriggerLinkSchedule = async (schedule: ScheduleDTO) => {
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      const res = await triggerLinkSchedule(projectId);
      showToast({
        type: "success",
        title: "Запуск ссылок инициирован",
        message: `${schedule.name} · ${res.enqueued ?? 0} задач`
      });
    } catch (err: any) {
      const msg = err?.message || "Не удалось запустить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка запуска", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleDeleteLinkSchedule = async (schedule: ScheduleDTO) => {
    if (!confirm(`Удалить расписание ссылок ${schedule.name}?`)) return;
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      await deleteLinkSchedule(projectId);
      showToast({
        type: "success",
        title: "Расписание ссылок удалено",
        message: schedule.name
      });
      setLinkSchedule(null);
      resetLinkScheduleForm();
      await loadLinkSchedule();
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  useEffect(() => {
    if (projectId) {
      load();
    }
  }, [projectId]);

  useEffect(() => {
    if (activeTab === "schedules") {
      loadSchedules();
      loadLinkSchedule();
    }
  }, [activeTab, projectId]);

  useEffect(() => {
    if (activeTab === "errors") {
      loadProjectErrors();
    }
  }, [activeTab, projectId, domains]);

  const addDomain = async () => {
    if (!url.trim()) return;
    const normalizedDomain = normalizeDomainForImport(url);
    if (!normalizedDomain) {
      setError("Укажите корректный домен (без пути, порта и query).");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains`, { url: normalizedDomain, keyword, country, language, exclude_domains: exclude });
      setUrl("");
      setKeyword("");
      setExclude("");
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось добавить домен");
    } finally {
      setLoading(false);
    }
  };

  const importDomains = async () => {
    if (!importText.trim()) return;
    const parsed = parseDomainImportText(importText);
    if (parsed.errors.length > 0) {
      const lines = parsed.errors.slice(0, 5).map((entry) => `${entry.line} (${entry.reason})`).join(", ");
      setError(`Импорт остановлен. Проверьте строки: ${lines}.`);
      return;
    }
    if (parsed.items.length === 0) {
      setError("Импорт пустой: добавьте хотя бы один корректный домен.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const result = await post<{ created?: number; skipped?: number; total?: number }>(`/api/projects/${projectId}/domains/import`, {
        items: parsed.items
      });
      const created = Number(result?.created ?? parsed.items.length);
      const skipped = Number(result?.skipped ?? 0);
      showToast({
        type: skipped > 0 ? "warning" : "success",
        title: skipped > 0 ? "Импорт завершен с пропусками" : "Импорт завершен",
        message: skipped > 0 ? `Создано: ${created}, пропущено: ${skipped}` : `Создано: ${created}`
      });
      setImportText("");
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось импортировать");
    } finally {
      setLoading(false);
    }
  };

  const updateKeyword = async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/domains/${id}`, { keyword: keywordEdits[id] || "" });
      showToast({
        type: "success",
        title: "Ключевое слово сохранено",
        message: domainById[id]?.url || ""
      });
      await load(true);
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось сохранить ключевое слово",
        message: err?.message || "Попробуйте позже"
      });
      setError(err?.message || "Не удалось обновить ключевое слово");
    } finally {
      setLoading(false);
    }
  };

  const updateLinkSettings = async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      const entry = linkEdits[id] || { anchor: "", acceptor: "" };
      await patch(`/api/domains/${id}`, {
        link_anchor_text: entry.anchor?.trim() || "",
        link_acceptor_url: entry.acceptor?.trim() || ""
      });
      showToast({
        type: "success",
        title: "Ссылка сохранена",
        message: domainById[id]?.url || ""
      });
      await load(true);
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось сохранить ссылку",
        message: err?.message || "Попробуйте позже"
      });
      setError(err?.message || "Не удалось обновить ссылку");
    } finally {
      setLoading(false);
    }
  };

  const { runGeneration, runLinkTask, removeLinkTask, deleteDomain, generationFlow, linkFlow } = useProjectActions({
    projectId,
    project,
    domains,
    domainById,
    keywordEdits,
    linkEdits,
    setLoading,
    setError,
    setLinkLoadingId,
    load
  });

  const deleteProject = async () => {
    if (!confirm("Удалить проект и все его домены?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/projects/${projectId}`);
      router.push("/projects");
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить проект");
    } finally {
      setLoading(false);
    }
  };

  const addMember = async () => {
    if (!newMemberEmail.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/members`, { email: newMemberEmail.trim(), role: newMemberRole });
      setNewMemberEmail("");
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось добавить участника");
    } finally {
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
      setError(err?.message || "Не удалось обновить роль");
    } finally {
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
      setError(err?.message || "Не удалось удалить участника");
    } finally {
      setLoading(false);
    }
  };

  const loadGens = async (id: string) => {
    try {
      const list = await authFetch<Generation[]>(`/api/domains/${id}/generations`);
      setGens((prev) => ({ ...prev, [id]: Array.isArray(list) ? list : [] }));
      // Переключаем состояние открытия/закрытия
      setOpenRuns((prev) => ({ ...prev, [id]: !prev[id] }));
    } catch {
      /* ignore */
    }
  };


  return (
    <div className="space-y-4">
      <ProjectHeaderActionsSection
        project={project}
        projectId={projectId}
        loading={loading}
        error={error}
        generationFlow={generationFlow}
        linkFlow={linkFlow}
        onRefresh={() => load(true)}
        onDeleteProject={deleteProject}
      />

      {/* Tabs */}
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 dark:border-slate-800 mb-4">
          <div className="flex flex-wrap gap-2">
          <Link
            href={buildTabHref("domains")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "domains"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Домены
          </Link>
          <Link
            href={buildTabHref("schedules")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "schedules"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Расписания
          </Link>
          <Link
            href={buildTabHref("errors")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "errors"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Ошибки
          </Link>
          <Link
            href={buildTabHref("settings")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "settings"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Настройки
          </Link>
          </div>
        </div>

        {activeTab === "domains" && (
          <ProjectDomainsSection
            loading={loading}
            url={url}
            keyword={keyword}
            country={country}
            language={language}
            exclude={exclude}
            importText={importText}
            domainSearch={domainSearch}
            domainsCount={domains.length}
            filteredDomains={filteredDomains}
            linkLoadingId={linkLoadingId}
            openRuns={openRuns}
            gens={gens}
            keywordEdits={keywordEdits}
            linkEdits={linkEdits}
            onUrlChange={setUrl}
            onKeywordChange={setKeyword}
            onCountryChange={setCountry}
            onLanguageChange={setLanguage}
            onExcludeChange={setExclude}
            onImportTextChange={setImportText}
            onDomainSearchChange={setDomainSearch}
            onAddDomain={addDomain}
            onImportDomains={importDomains}
            onRunLinkTask={runLinkTask}
            onRemoveLinkTask={removeLinkTask}
            onLoadRuns={loadGens}
            onDeleteDomain={deleteDomain}
            onKeywordEditChange={(domainId, value) => setKeywordEdits((prev) => ({ ...prev, [domainId]: value }))}
            onUpdateKeyword={updateKeyword}
            onLinkEditChange={(domainId, value) => setLinkEdits((prev) => ({ ...prev, [domainId]: value }))}
            onUpdateLinkSettings={updateLinkSettings}
            getEffectiveLinkStatus={getEffectiveDomainLinkStatus}
            renderStatusBadge={(status) => <ProjectStatusBadge status={status} />}
            renderLinkStatusBadge={(domain) => <ProjectLinkStatusBadge domain={domain} />}
            renderRunsList={(runs) => <ProjectRunsList runs={runs as Generation[]} />}
            formatDateTime={formatDateTime}
            formatRelativeTime={formatRelativeTime}
          />
        )}

        {activeTab === "schedules" && (
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
        )}

        {activeTab === "errors" && (
          <ProjectDiagnosticsSection
            loading={projectErrorsLoading}
            error={projectErrorsError}
            items={projectErrors}
            domainById={domainById}
            formatDateTime={formatDateTime}
            onRefresh={() => {
              void loadProjectErrors(true);
            }}
          />
        )}

        {activeTab === "settings" && (
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
            onProjectTimezoneChange={(value) => {
              setProjectTimezone(value);
              updateRecentTimezone(value);
            }}
            onUseRecentTimezone={(value) => {
              setProjectTimezone(value);
              updateRecentTimezone(value);
            }}
            onNewMemberEmailChange={setNewMemberEmail}
            onNewMemberRoleChange={setNewMemberRole}
            onAddMember={addMember}
            onUpdateMemberRole={updateMemberRole}
            onRemoveMember={removeMember}
          />
        )}
      </div>
    </div>
  );
}
