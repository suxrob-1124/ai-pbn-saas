"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import { authFetch, post, patch, del } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import { FiPlay, FiRefreshCw, FiList, FiClock, FiPauseCircle, FiCheck, FiTrash2, FiUsers, FiX, FiKey, FiAlertCircle, FiLink } from "react-icons/fi";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { showToast } from "../../../lib/toastStore";
import { createSchedule, deleteSchedule, listSchedules, triggerSchedule, updateSchedule } from "../../../lib/schedulesApi";
import { deleteLinkSchedule, getLinkSchedule, triggerLinkSchedule, upsertLinkSchedule } from "../../../lib/linkSchedulesApi";
import { deleteQueueItem, listQueue } from "../../../lib/queueApi";
import type { ScheduleDTO } from "../../../types/schedules";
import type { QueueItemDTO } from "../../../types/queue";
import { ScheduleForm } from "../../../components/ScheduleForm";
import { ScheduleList } from "../../../components/ScheduleList";
import type { ScheduleFormValue } from "../../../lib/scheduleFormValidation";

type Project = {
  id: string;
  name: string;
  target_country?: string;
  target_language?: string;
  status?: string;
  ownerHasApiKey?: boolean;
};

type Domain = {
  id: string;
  project_id: string;
  url: string;
  main_keyword?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  status: string;
  last_generation_id?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_status?: string;
  link_last_task_id?: string;
  updated_at?: string;
};

type Generation = {
  id: string;
  status: string;
  progress: number;
  error?: string;
  created_at?: string;
  finished_at?: string;
};

const QUEUE_STATUS_LABELS: Record<string, string> = {
  pending: "Ожидает",
  queued: "В очереди",
  completed: "Завершено",
  failed: "Ошибка"
};

export default function ProjectDetailPage() {
  const { me } = useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const projectId = params?.id as string;
  const [project, setProject] = useState<Project | null>(null);
  const [domains, setDomains] = useState<Domain[]>([]);
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

  const [url, setUrl] = useState("");
  const [keyword, setKeyword] = useState("");
  const [country, setCountry] = useState("");
  const [language, setLanguage] = useState("");
  const [exclude, setExclude] = useState("");
  const [importText, setImportText] = useState("");
  const [keywordEdits, setKeywordEdits] = useState<Record<string, string>>({});
  const [linkEdits, setLinkEdits] = useState<Record<string, { anchor: string; acceptor: string }>>({});
  const [activeTab, setActiveTab] = useState<"domains" | "members" | "schedules" | "queue">("domains");
  const [members, setMembers] = useState<Array<{ email: string; role: string; createdAt: string }>>([]);
  const [newMemberEmail, setNewMemberEmail] = useState("");
  const [newMemberRole, setNewMemberRole] = useState("editor");
  const [schedules, setSchedules] = useState<ScheduleDTO[]>([]);
  const [scheduleForm, setScheduleForm] = useState<ScheduleFormValue>({
    name: "",
    description: "",
    strategy: "daily",
    isActive: true,
    dailyLimit: "5",
    dailyTime: "09:00",
    weeklyLimit: "3",
    weeklyDay: "mon",
    weeklyTime: "09:00",
    customCron: "0 9 * * *"
  });
  const [editingSchedule, setEditingSchedule] = useState<ScheduleDTO | null>(null);
  const [schedulesMultiple, setSchedulesMultiple] = useState(false);
  const [schedulesLoading, setSchedulesLoading] = useState(false);
  const [schedulesError, setSchedulesError] = useState<string | null>(null);
  const [schedulesPermission, setSchedulesPermission] = useState(false);
  const [linkSchedule, setLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleForm, setLinkScheduleForm] = useState<ScheduleFormValue>({
    name: "",
    description: "",
    strategy: "daily",
    isActive: true,
    dailyLimit: "5",
    dailyTime: "09:00",
    weeklyLimit: "3",
    weeklyDay: "mon",
    weeklyTime: "09:00",
    customCron: "0 9 * * *"
  });
  const [editingLinkSchedule, setEditingLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleLoading, setLinkScheduleLoading] = useState(false);
  const [linkScheduleError, setLinkScheduleError] = useState<string | null>(null);
  const [linkSchedulePermission, setLinkSchedulePermission] = useState(false);
  const [queueItems, setQueueItems] = useState<QueueItemDTO[]>([]);
  const [queueLoading, setQueueLoading] = useState(false);
  const [queueError, setQueueError] = useState<string | null>(null);
  const [queuePermission, setQueuePermission] = useState(false);
  const [linkLoadingId, setLinkLoadingId] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const p = await authFetch<Project>(`/api/projects/${projectId}`);
      setProject(p);
      const d = await authFetch<Domain[]>(`/api/projects/${projectId}/domains`);
      setDomains(Array.isArray(d) ? d : []);
      const edits: Record<string, string> = {};
      const linkDrafts: Record<string, { anchor: string; acceptor: string }> = {};
      (Array.isArray(d) ? d : []).forEach((item) => {
        edits[item.id] = item.main_keyword || "";
        linkDrafts[item.id] = {
          anchor: item.link_anchor_text || "",
          acceptor: item.link_acceptor_url || ""
        };
      });
      setKeywordEdits(edits);
      setLinkEdits(linkDrafts);
      setCountry(p?.target_country || "");
      setLanguage(p?.target_language || "");
      
      // Загружаем участников
      try {
        const m = await authFetch<Array<{ email: string; role: string; createdAt: string }>>(`/api/projects/${projectId}/members`);
        setMembers(Array.isArray(m) ? m : []);
      } catch {
        // Игнорируем ошибки загрузки участников (может не быть прав)
      }
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить проект");
    } finally {
      setLoading(false);
    }
  };

  const isPermissionError = (message: string) =>
    /permission|access denied|admin only|forbidden/i.test(message);

  const loadSchedules = async () => {
    if (!projectId) return;
    setSchedulesLoading(true);
    setSchedulesError(null);
    setSchedulesPermission(false);
    try {
      const list = await listSchedules(projectId);
      const normalized = Array.isArray(list) ? list : [];
      setSchedules(normalized);
      setSchedulesMultiple(normalized.length > 1);
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
      setLinkSchedule(schedule);
      if (schedule) {
        applyLinkScheduleToForm(schedule);
      } else {
        resetLinkScheduleForm();
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить расписание ссылок";
      if (isPermissionError(msg)) {
        setLinkSchedulePermission(true);
      } else {
        setLinkScheduleError(msg);
      }
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const loadQueue = async () => {
    if (!projectId) return;
    setQueueLoading(true);
    setQueueError(null);
    setQueuePermission(false);
    try {
      const list = await listQueue(projectId);
      setQueueItems(Array.isArray(list) ? list : []);
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить очередь";
      if (isPermissionError(msg)) {
        setQueuePermission(true);
      } else {
        setQueueError(msg);
      }
    } finally {
      setQueueLoading(false);
    }
  };

  const resetScheduleForm = () => {
    setScheduleForm({
      name: "",
      description: "",
      strategy: "daily",
      isActive: true,
      dailyLimit: "5",
      dailyTime: "09:00",
      weeklyLimit: "3",
      weeklyDay: "mon",
      weeklyTime: "09:00",
      customCron: "0 9 * * *"
    });
    setEditingSchedule(null);
  };

  const resetLinkScheduleForm = () => {
    setLinkScheduleForm({
      name: "",
      description: "",
      strategy: "daily",
      isActive: true,
      dailyLimit: "5",
      dailyTime: "09:00",
      weeklyLimit: "3",
      weeklyDay: "mon",
      weeklyTime: "09:00",
      customCron: "0 9 * * *"
    });
    setEditingLinkSchedule(null);
  };

  const applyScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    setScheduleForm({
      name: schedule.name,
      description: schedule.description ?? "",
      strategy: schedule.strategy,
      isActive: schedule.isActive,
      dailyLimit: typeof config.limit === "number" ? String(config.limit) : "5",
      dailyTime: typeof config.time === "string" ? config.time : "09:00",
      weeklyLimit: typeof config.limit === "number" ? String(config.limit) : "3",
      weeklyDay: typeof config.day === "string" ? config.day : "mon",
      weeklyTime: typeof config.time === "string" ? config.time : "09:00",
      customCron: typeof config.cron === "string" ? config.cron : "0 9 * * *"
    });
    setEditingSchedule(schedule);
  };

  const applyLinkScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    setLinkScheduleForm({
      name: schedule.name,
      description: schedule.description ?? "",
      strategy: schedule.strategy,
      isActive: schedule.isActive,
      dailyLimit: typeof config.limit === "number" ? String(config.limit) : "5",
      dailyTime: typeof config.time === "string" ? config.time : "09:00",
      weeklyLimit: typeof config.limit === "number" ? String(config.limit) : "3",
      weeklyDay: typeof config.day === "string" ? config.day : "mon",
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
          isActive: scheduleForm.isActive
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
          isActive: scheduleForm.isActive
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
      setActiveTab("schedules");
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
        let queuedForSchedule = 0;
        try {
          const list = await listQueue(projectId);
          const items = Array.isArray(list) ? list : [];
          setQueueItems(items);
          queuedForSchedule = items.filter(
            (item) =>
              item.schedule_id === sched.id &&
              (item.status === "pending" || item.status === "queued")
          ).length;
        } catch {
          queuedForSchedule = 0;
        }
        if (queuedForSchedule > 0) {
          showToast({
            type: "info",
            title: "Уже в очереди",
            message: `${sched.name} · ${queuedForSchedule} задач`
          });
        } else {
          showToast({
            type: "warning",
            title: "Нечего запускать",
            message: `${sched.name} · нет доменов для запуска`
          });
        }
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
        isActive: linkScheduleForm.isActive
      });
      showToast({
        type: "success",
        title: "Расписание ссылок сохранено",
        message: saved.name
      });
      setLinkSchedule(saved);
      applyLinkScheduleToForm(saved);
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
      setActiveTab("schedules");
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
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleDeleteQueueItem = async (item: QueueItemDTO) => {
    if (!confirm(`Удалить элемент очереди ${item.id}?`)) return;
    setQueueLoading(true);
    setQueueError(null);
    try {
      await deleteQueueItem(item.id);
      showToast({
        type: "success",
        title: "Элемент очереди удален",
        message: item.id
      });
      await loadQueue();
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить элемент очереди";
      setQueueError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setQueueLoading(false);
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
    if (activeTab === "queue") {
      loadQueue();
    }
  }, [activeTab, projectId]);

  const addDomain = async () => {
    if (!url.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains`, { url, keyword, country, language, exclude_domains: exclude });
      setUrl("");
      setKeyword("");
      setExclude("");
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось добавить домен");
    } finally {
      setLoading(false);
    }
  };

  const importDomains = async () => {
    if (!importText.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains/import`, { text: importText });
      setImportText("");
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось импортировать");
    } finally {
      setLoading(false);
    }
  };

  const runGeneration = async (id: string) => {
    const domain = domains.find((d) => d.id === id);
    if (!(keywordEdits[id] || "").trim() && !(domain?.main_keyword || "").trim()) {
      setError("Сначала задайте ключевое слово");
      return;
    }
    if (domain?.status === "processing" || domain?.status === "pending") {
      setError("У этого домена уже есть запущенная генерация");
      return;
    }
    // Проверяем наличие API ключа у владельца проекта
    if (project && project.ownerHasApiKey === false) {
      setError("API ключ не настроен у владельца проекта. Настройте ключ в профиле для запуска генерации.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await post(`/api/domains/${id}/generate`);
      await load();
    } catch (err: any) {
      const errMsg = err?.message || "Не удалось запустить генерацию";
      // Улучшаем сообщение об ошибке, если это связано с API ключом
      if (errMsg.includes("API key") || errMsg.includes("api key")) {
        setError(`${errMsg} Настройте API ключ в профиле.`);
      } else {
        setError(errMsg);
      }
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
      await load();
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
      await load();
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

  const deleteDomain = async (id: string) => {
    if (!confirm("Удалить домен?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/domains/${id}`);
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить домен");
    } finally {
      setLoading(false);
    }
  };

  const runLinkTask = async (id: string) => {
    const domain = domainById[id];
    if (!domain) return;
    const anchor = (domain.link_anchor_text || "").trim();
    const acceptor = (domain.link_acceptor_url || "").trim();
    const draft = linkEdits[id] || { anchor, acceptor };
    const draftAnchor = (draft.anchor || "").trim();
    const draftAcceptor = (draft.acceptor || "").trim();
    if (draftAnchor !== anchor || draftAcceptor !== acceptor) {
      showToast({
        type: "error",
        title: "Сначала сохраните ссылку",
        message: "В полях есть несохранённые изменения."
      });
      return;
    }
    if (!anchor || !acceptor) {
      showToast({
        type: "error",
        title: "Ссылка не настроена",
        message: "Заполните анкор и акцептор в настройках домена."
      });
      return;
    }
    const hasExistingLink = Boolean(domain.link_status || domain.link_last_task_id);
    setLinkLoadingId(id);
    try {
      await post(`/api/domains/${id}/link/run`);
      showToast({
        type: "success",
        title: hasExistingLink ? "Ссылка обновляется" : "Ссылка добавляется",
        message: domain.url
      });
      await load();
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось запустить ссылку",
        message: err?.message || "Попробуйте позже"
      });
    } finally {
      setLinkLoadingId(null);
    }
  };

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
      await load();
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
      await load();
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
      await load();
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
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold">{project?.name || "Проект"}</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Страна: {project?.target_country || "—"} · Язык: {project?.target_language || "—"}
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            <button
              onClick={deleteProject}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
            >
              <FiTrash2 /> Удалить
            </button>
          </div>
        </div>
        {error && <div className="text-red-500 text-sm mt-2">{error}</div>}
        
        {/* Индикатор API ключа */}
        {project && (
          <div className="mt-4">
            {project.ownerHasApiKey === false ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/20 p-3">
                <div className="flex items-start gap-2">
                  <FiAlertCircle className="text-amber-600 dark:text-amber-400 mt-0.5" />
                  <div className="flex-1">
                    <div className="text-sm font-semibold text-amber-800 dark:text-amber-200">
                      ⚠️ API ключ не настроен
                    </div>
                    <div className="text-xs text-amber-700 dark:text-amber-300 mt-1">
                      Генерация не будет работать без API ключа владельца проекта.{" "}
                      <a href="/me" className="underline hover:no-underline">
                        Настроить в профиле →
                      </a>
                    </div>
                  </div>
                </div>
              </div>
            ) : project.ownerHasApiKey === true ? (
              <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/40 p-3">
                <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-400">
                  <FiKey className="text-emerald-600 dark:text-emerald-400" />
                  <span>API ключ настроен. Генерация будет использовать ключ владельца проекта.</span>
                </div>
              </div>
            ) : null}
          </div>
        )}
      </div>

      {/* Tabs */}
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <div className="flex gap-2 border-b border-slate-200 dark:border-slate-800 mb-4">
          <button
            onClick={() => setActiveTab("domains")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "domains"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Домены
          </button>
          <button
            onClick={() => setActiveTab("schedules")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "schedules"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Расписания
          </button>
          <button
            onClick={() => setActiveTab("queue")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "queue"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Очередь
          </button>
          <button
            onClick={() => setActiveTab("members")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "members"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            <FiUsers className="inline mr-1" /> Участники
          </button>
        </div>

        {activeTab === "domains" && (
          <>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3 mb-3">
        <h3 className="font-semibold">Добавить домен</h3>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="example.com"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Ключевое слово"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
          <div className="flex gap-2">
            <button
              onClick={addDomain}
              disabled={loading || !url.trim()}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
            >
              Добавить
            </button>
          </div>
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Страна (по умолчанию из проекта)"
            value={country}
            onChange={(e) => setCountry(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Язык (по умолчанию из проекта)"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Исключить домены (через запятую)"
            value={exclude}
            onChange={(e) => setExclude(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Импорт списком (url[,ключевое слово] на строку). Пример: <code>example.com,casino</code>
          </p>
          <textarea
            className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            rows={4}
            placeholder="example.com,ключевое слово&#10;example.org"
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
          />
          <button
            onClick={importDomains}
            disabled={loading || !importText.trim()}
            className="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-4 py-2 text-sm font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
          >
            Импортировать
          </button>
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Домены</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {domains.length}</span>
        </div>
        <div className="space-y-3">
          {domains.map((d) => (
            <div
              key={d.id}
              className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white/60 dark:bg-slate-900/40 p-4 shadow-sm space-y-3"
            >
              <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Link href={`/domains/${d.id}`} className="text-indigo-600 hover:underline font-semibold">
                      {d.url}
                    </Link>
                    <StatusBadge status={d.status} />
                    <LinkStatusBadge domain={d} />
                  </div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">
                    Обновлено: {d.updated_at ? new Date(d.updated_at).toLocaleString() : "—"}
                  </div>
                  <div className="text-xs text-slate-400">
                    Страна: {d.target_country || "—"} · Язык: {d.target_language || "—"}
                  </div>
                  {d.exclude_domains && <div className="text-xs text-slate-400">Исключить: {d.exclude_domains}</div>}
                </div>
                <div className="flex flex-wrap items-center gap-2 md:justify-end">
                  <button
                    onClick={() => runLinkTask(d.id)}
                    disabled={loading || linkLoadingId === d.id}
                    className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <FiLink />
                    {d.link_status || d.link_last_task_id ? "Обновить ссылку" : "Добавить ссылку"}
                  </button>
                  <button
                    onClick={() => runLinkTask(d.id)}
                    disabled={loading || linkLoadingId === d.id}
                    className="inline-flex sm:hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    title={d.link_status || d.link_last_task_id ? "Обновить ссылку" : "Добавить ссылку"}
                    aria-label={d.link_status || d.link_last_task_id ? "Обновить ссылку" : "Добавить ссылку"}
                  >
                    <FiLink />
                  </button>
                  <button
                    onClick={() => loadGens(d.id)}
                    className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <FiList /> Запуски {openRuns[d.id] && gens[d.id] && `(${gens[d.id].length})`}
                  </button>
                  <button
                    onClick={() => loadGens(d.id)}
                    className="inline-flex sm:hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    title={`Запуски ${openRuns[d.id] && gens[d.id] ? `(${gens[d.id].length})` : ""}`}
                    aria-label="Запуски"
                  >
                    <FiList />
                  </button>
                  <button
                    onClick={() => deleteDomain(d.id)}
                    disabled={loading}
                    className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                  >
                    Удалить
                  </button>
                  <button
                    onClick={() => deleteDomain(d.id)}
                    disabled={loading}
                    className="inline-flex sm:hidden items-center justify-center rounded-lg border border-red-200 bg-white px-2 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                    title="Удалить"
                    aria-label="Удалить"
                  >
                    <FiTrash2 />
                  </button>
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                {(() => {
                  const keywordValue = keywordEdits[d.id] ?? "";
                  const keywordDirty = keywordValue.trim() !== (d.main_keyword || "").trim();
                  return (
                    <div className="space-y-1">
                      <div className="text-xs uppercase tracking-wide text-slate-400 flex items-center gap-2">
                        Ключевое слово
                        {keywordDirty && (
                          <span className="rounded-full bg-amber-900/30 px-2 py-0.5 text-[10px] text-amber-300">
                            несохранено
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <input
                          className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${keywordDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""}`}
                          value={keywordValue}
                          onChange={(e) => setKeywordEdits((prev) => ({ ...prev, [d.id]: e.target.value }))}
                          placeholder="Ключевое слово"
                        />
                        <button
                          onClick={() => updateKeyword(d.id)}
                          disabled={loading || !keywordDirty}
                          className="hidden sm:inline-flex items-center gap-1 rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                        >
                          Сохранить
                        </button>
                        <button
                          onClick={() => updateKeyword(d.id)}
                          disabled={loading || !keywordDirty}
                          className="inline-flex sm:hidden items-center justify-center rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                          title="Сохранить ключевое слово"
                          aria-label="Сохранить ключевое слово"
                        >
                          <FiCheck />
                        </button>
                      </div>
                    </div>
                  );
                })()}
                {(() => {
                  const link = linkEdits[d.id] || { anchor: d.link_anchor_text || "", acceptor: d.link_acceptor_url || "" };
                  const anchorValue = link.anchor ?? "";
                  const acceptorValue = link.acceptor ?? "";
                  const linkDirty =
                    anchorValue.trim() !== (d.link_anchor_text || "").trim() ||
                    acceptorValue.trim() !== (d.link_acceptor_url || "").trim();
                  return (
                    <div className="space-y-1">
                      <div className="text-xs uppercase tracking-wide text-slate-400 flex items-center gap-2">
                        Ссылка
                        {linkDirty && (
                          <span className="rounded-full bg-amber-900/30 px-2 py-0.5 text-[10px] text-amber-300">
                            несохранено
                          </span>
                        )}
                      </div>
                      <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                        <input
                          className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${linkDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""}`}
                          value={anchorValue}
                          onChange={(e) =>
                            setLinkEdits((prev) => ({
                              ...prev,
                              [d.id]: { anchor: e.target.value, acceptor: prev[d.id]?.acceptor ?? "" }
                            }))
                          }
                          placeholder="Анкор"
                        />
                        <input
                          className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${linkDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""}`}
                          value={acceptorValue}
                          onChange={(e) =>
                            setLinkEdits((prev) => ({
                              ...prev,
                              [d.id]: { anchor: prev[d.id]?.anchor ?? "", acceptor: e.target.value }
                            }))
                          }
                          placeholder="https://acceptor.example"
                        />
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => updateLinkSettings(d.id)}
                            disabled={loading || !linkDirty}
                            className="hidden sm:inline-flex items-center gap-1 rounded-lg bg-slate-200 px-3 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                          >
                            Сохранить
                          </button>
                          <button
                            onClick={() => updateLinkSettings(d.id)}
                            disabled={loading || !linkDirty}
                            className="inline-flex sm:hidden items-center justify-center rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                            title="Сохранить ссылку"
                            aria-label="Сохранить ссылку"
                          >
                            <FiCheck />
                          </button>
                        </div>
                      </div>
                    </div>
                  );
                })()}
              </div>

              {openRuns[d.id] && gens[d.id] && <RunsList runs={gens[d.id]} />}
            </div>
          ))}
        </div>
      </div>
          </>
        )}

        {activeTab === "schedules" && (
          <div className="space-y-4">
            <div className="space-y-3">
              <h3 className="text-base font-semibold">Расписание генерации</h3>
              {schedulesMultiple && (
                <div className="text-sm text-amber-600 dark:text-amber-400">
                  Обнаружено несколько расписаний. Отображается и редактируется только первое.
                </div>
              )}
              <ScheduleForm
                value={scheduleForm}
                loading={schedulesLoading}
                error={schedulesError}
                title={editingSchedule ? "Редактировать расписание генерации" : "Новое расписание генерации"}
                submitLabel={editingSchedule ? "Сохранить изменения" : "Создать расписание"}
                onChange={setScheduleForm}
                onSubmit={handleSubmitSchedule}
              />
              <ScheduleList
                title="Расписание генерации"
                schedules={schedules.slice(0, 1)}
                loading={schedulesLoading}
                error={schedulesError}
                permissionDenied={schedulesPermission}
                onRefresh={loadSchedules}
                onTrigger={handleTriggerSchedule}
                onToggle={handleToggleSchedule}
                onEdit={handleEditSchedule}
                onDelete={handleDeleteSchedule}
              />
            </div>

            <div className="space-y-3">
              <h3 className="text-base font-semibold">Расписание ссылок</h3>
              <ScheduleForm
                value={linkScheduleForm}
                loading={linkScheduleLoading}
                error={linkScheduleError}
                title={editingLinkSchedule ? "Редактировать расписание ссылок" : "Новое расписание ссылок"}
                submitLabel={editingLinkSchedule ? "Сохранить изменения" : "Создать расписание"}
                onChange={setLinkScheduleForm}
                onSubmit={handleSubmitLinkSchedule}
              />
              <ScheduleList
                title="Расписание ссылок"
                schedules={linkSchedule ? [linkSchedule] : []}
                loading={linkScheduleLoading}
                error={linkScheduleError}
                permissionDenied={linkSchedulePermission}
                onRefresh={loadLinkSchedule}
                onTrigger={handleTriggerLinkSchedule}
                onToggle={handleToggleLinkSchedule}
                onEdit={handleEditLinkSchedule}
                onDelete={handleDeleteLinkSchedule}
              />
            </div>
          </div>
        )}

        {activeTab === "queue" && (
          <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold">Очередь проекта</h3>
              <button
                onClick={loadQueue}
                disabled={queueLoading}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                <FiRefreshCw /> Обновить
              </button>
            </div>
            {queueLoading && (
              <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка очереди...</div>
            )}
            {!queueLoading && queuePermission && (
              <div className="text-sm text-amber-600 dark:text-amber-400">
                Недостаточно прав для просмотра очереди.
              </div>
            )}
            {!queueLoading && !queuePermission && queueError && (
              <div className="text-sm text-red-500">{queueError}</div>
            )}
            {!queueLoading &&
              !queuePermission &&
              !queueError &&
              queueItems.length === 0 && (
                <div className="text-sm text-slate-500 dark:text-slate-400">
                  Очередь пуста.
                </div>
              )}
            {!queueLoading &&
              !queuePermission &&
              !queueError &&
              queueItems.length > 0 && (
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                        <th className="py-2 pr-4">ID</th>
                        <th className="py-2 pr-4">Домен</th>
                        <th className="py-2 pr-4">Статус</th>
                        <th className="py-2 pr-4">Запланировано</th>
                        <th className="py-2 pr-4 text-right">Действия</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                      {queueItems.map((item) => {
                        const domain = domainById[item.domain_id];
                        const domainLabel = item.domain_url || domain?.url || item.domain_id;
                        const statusLabel = QUEUE_STATUS_LABELS[item.status] || item.status;
                        return (
                          <tr key={item.id}>
                            <td className="py-3 pr-4 font-mono text-xs">{item.id.slice(0, 8)}</td>
                            <td className="py-3 pr-4">
                              {domain || item.domain_url ? (
                                <Link href={`/domains/${domain?.id || item.domain_id}`} className="text-indigo-600 hover:underline">
                                  {domainLabel}
                                </Link>
                              ) : (
                                <span className="text-slate-500 dark:text-slate-400">{domainLabel}</span>
                              )}
                            </td>
                            <td className="py-3 pr-4">{statusLabel}</td>
                            <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                              {new Date(item.scheduled_for).toLocaleString()}
                            </td>
                            <td className="py-3 pr-4 text-right">
                              <button
                                onClick={() => handleDeleteQueueItem(item)}
                                disabled={queueLoading}
                                className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                              >
                                <FiTrash2 /> Удалить
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            <Link href="/queue" className="text-xs text-indigo-600 hover:underline">
              Открыть общую очередь →
            </Link>
          </div>
        )}

        {activeTab === "members" && (
          <div className="space-y-4">
            <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
              <h3 className="font-semibold">Добавить участника</h3>
              <div className="grid gap-3 md:grid-cols-3">
                <input
                  className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  placeholder="email@example.com"
                  value={newMemberEmail}
                  onChange={(e) => setNewMemberEmail(e.target.value)}
                />
                <select
                  className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  value={newMemberRole}
                  onChange={(e) => setNewMemberRole(e.target.value)}
                >
                  <option value="viewer">Наблюдатель</option>
                  <option value="editor">Редактор</option>
                  <option value="owner">Владелец</option>
                </select>
                <button
                  onClick={addMember}
                  disabled={loading || !newMemberEmail.trim()}
                  className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
                >
                  Добавить
                </button>
              </div>
            </div>

            <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="font-semibold">Участники проекта</h3>
                <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {members.length}</span>
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                      <th className="py-2 pr-4">Почта</th>
                      <th className="py-2 pr-4">Роль</th>
                      <th className="py-2 pr-4">Добавлен</th>
                      <th className="py-2 pr-4 text-right">Действия</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                    {members.map((m) => (
                      <tr key={m.email}>
                        <td className="py-3 pr-4 font-medium">{m.email}</td>
                        <td className="py-3 pr-4">
                          {m.role === "owner" ? (
                            <span className="text-sm text-slate-600 dark:text-slate-400">Владелец</span>
                          ) : (
                            <select
                              className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                              value={m.role}
                              onChange={(e) => updateMemberRole(m.email, e.target.value)}
                            >
                              <option value="viewer">Наблюдатель</option>
                              <option value="editor">Редактор</option>
                              <option value="owner">Владелец</option>
                            </select>
                          )}
                        </td>
                        <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                          {m.createdAt ? new Date(m.createdAt).toLocaleString() : "—"}
                        </td>
                        <td className="py-3 pr-4 text-right">
                          {m.role !== "owner" && (
                            <button
                              onClick={() => removeMember(m.email)}
                              disabled={loading}
                              className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                            >
                              <FiX /> Удалить
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {members.length === 0 && <div className="text-sm text-slate-500 mt-2">Участников пока нет.</div>}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function RunsList({ runs }: { runs: Generation[] }) {
  if (!Array.isArray(runs) || runs.length === 0) return null;
  // Показываем только последние 4 запуска
  const displayRuns = runs.slice(0, 4);
  return (
    <div className="mt-2 text-left text-xs bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700 rounded-lg p-2 space-y-1">
      {displayRuns.map((r) => (
        <Link
          key={r.id}
          href={`/queue/${r.id}`}
          className="flex items-center justify-between rounded-lg px-2 py-1 hover:bg-slate-100 dark:hover:bg-slate-700/60"
        >
          <span className="font-semibold">{r.id.slice(0, 8)}</span>
          <div className="flex items-center gap-2">
            <StatusBadge status={r.status} />
            <span className="text-slate-500 dark:text-slate-400">{r.progress}%</span>
            {r.error && <span className="text-red-500">ошибка</span>}
          </div>
        </Link>
      ))}
      {runs.length > 4 && (
        <div className="text-xs text-slate-500 dark:text-slate-400 px-2 py-1">
          ... и еще {runs.length - 4} запусков
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; color: string; icon: React.ReactNode }> = {
    waiting: { text: "Ожидает генерацию", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiClock /> },
    processing: { text: "Генерация", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPlay /> },
    published: { text: "Опубликован", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiPlay /> },
    draft: { text: "Черновик", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> },
    active: { text: "Активен", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiPlay /> },
    paused: { text: "Приостановлено", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> },
    pause_requested: { text: "Пауза запрошена", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-200", icon: <FiPauseCircle /> },
    cancelling: { text: "Отмена...", color: "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-200", icon: <FiPauseCircle /> },
    cancelled: { text: "Отменено", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiPauseCircle /> },
    pending: { text: "В очереди", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiClock /> },
    success: { text: "Готово", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiCheck /> },
    error: { text: "Ошибка", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiPauseCircle /> },
    running: { text: "В работе", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPlay /> }
  };
  const cfg = map[status] || { text: status, color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${cfg.color}`}>
      {cfg.icon} {cfg.text}
    </span>
  );
}

function LinkStatusBadge({ domain }: { domain: { link_anchor_text?: string; link_acceptor_url?: string; link_status?: string; link_last_task_id?: string } }) {
  const hasSettings = Boolean((domain.link_anchor_text || "").trim()) && Boolean((domain.link_acceptor_url || "").trim());
  if (!hasSettings) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200">
        <FiClock /> Ожидает настройки
      </span>
    );
  }
  const status = domain.link_status || (domain.link_last_task_id ? "pending" : "ready");
  const map: Record<string, { text: string; color: string; icon: React.ReactNode }> = {
    ready: { text: "Готово к запуску", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiClock /> },
    pending: { text: "Ожидает добавления", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiClock /> },
    searching: { text: "Поиск места", color: "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200", icon: <FiRefreshCw /> },
    found: { text: "Место найдено", color: "bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200", icon: <FiCheck /> },
    inserted: { text: "Ссылка вставлена", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiCheck /> },
    generated: { text: "Текст сгенерирован", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-200", icon: <FiCheck /> },
    failed: { text: "Ошибка ссылки", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiPauseCircle /> }
  };
  const cfg = map[status] || map.ready;
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${cfg.color}`}>
      {cfg.icon} {cfg.text}
    </span>
  );
}
