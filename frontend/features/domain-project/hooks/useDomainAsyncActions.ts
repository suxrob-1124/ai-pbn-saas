import type { Dispatch, SetStateAction } from "react";
import { authFetch, del, patch } from "../../../lib/http";
import { showToast } from "../../../lib/toastStore";
import { useActionLocks } from "../../editor-v3/hooks/useActionLocks";
import { isLinkTaskInProgress } from "../../../lib/linkTaskStatus";

type DomainLite = {
  url?: string;
  link_status?: string;
  link_status_effective?: string;
};

type GenerationLite = {
  id: string;
  status: string;
  progress: number;
};

type LinkTaskLite = {
  id: string;
  domain_id: string;
  anchor_text: string;
  target_url: string;
  scheduled_for: string;
  status: string;
  action?: string;
  found_location?: string;
  generated_content?: string;
  error_message?: string;
  log_lines?: string[];
  attempts: number;
  created_by: string;
  created_at: string;
  completed_at?: string;
};

type UseDomainAsyncActionsParams = {
  id: string;
  kw: string;
  domain: DomainLite | null;
  gens: GenerationLite[];
  latestAttempt: GenerationLite | null;
  linkTasks: LinkTaskLite[];
  linkAnchor: string;
  linkAcceptor: string;
  canRemoveLink: boolean;
  load: (force?: boolean) => Promise<void>;
  setLoading: Dispatch<SetStateAction<boolean>>;
  setError: Dispatch<SetStateAction<string | null>>;
  setGens: Dispatch<SetStateAction<GenerationLite[]>>;
  setLatestAttempt: Dispatch<SetStateAction<GenerationLite | null>>;
  setPipelineStepInFlight: Dispatch<SetStateAction<string | null>>;
  setLinkTasksLoading: Dispatch<SetStateAction<boolean>>;
  setLinkTasksError: Dispatch<SetStateAction<string | null>>;
  setLinkNotice: Dispatch<SetStateAction<string | null>>;
  setLinkTasks: Dispatch<SetStateAction<LinkTaskLite[]>>;
};

export function useDomainAsyncActions({
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
}: UseDomainAsyncActionsParams) {
  const { runLocked } = useActionLocks();

  const triggerGeneration = async (forceStep?: string) => {
    if (!id) return;
    if (!kw.trim()) {
      setError("Сначала задайте ключевое слово");
      return;
    }
    const latestGen = latestAttempt || gens[0];
    if (!forceStep && latestGen?.status === "success") {
      if (!confirm("Генерация уже завершена. Запустить заново?")) {
        return;
      }
    }

    await runLocked(
      `domain:${id}:generation:${forceStep || "main"}`,
      async () => {
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
        setLatestAttempt((prev) => (prev ? { ...prev, status: "processing", progress: 0 } : prev));
        try {
          const payload = forceStep ? { force_step: forceStep } : undefined;
          const headers = payload ? { "Content-Type": "application/json" } : undefined;
          await authFetch(`/api/domains/${id}/generate`, {
            method: "POST",
            headers,
            body: payload ? JSON.stringify(payload) : undefined
          });
          await load(true);
        } catch (err: any) {
          const msg = err?.message || "Не удалось запустить генерацию";
          setError(msg);
          throw err;
        } finally {
          setLoading(false);
        }
      },
      "Генерация уже запускается"
    );
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

  const runLinkTask = async () => {
    if (!id) return;
    await runLocked(
      `domain:${id}:link:run`,
      async () => {
        setLinkTasksError(null);
        setLinkNotice(null);
        const domainLinkStatus = domain?.link_status_effective || domain?.link_status;
        const linkInProgressNow =
          isLinkTaskInProgress(domainLinkStatus) ||
          linkTasks.some((task) => isLinkTaskInProgress(task.status));
        if (linkInProgressNow) {
          showToast({
            type: "error",
            title: "Задача уже выполняется",
            message: "Дождитесь завершения текущей задачи по ссылке."
          });
          return;
        }
        if (!linkAnchor.trim() || !linkAcceptor.trim()) {
          setLinkTasksError("Заполните анкор и акцептор");
          showToast({
            type: "error",
            title: "Ссылка не настроена",
            message: "Заполните анкор и акцептор перед запуском."
          });
          return;
        }
        setLinkTasksLoading(true);
        try {
          await authFetch(`/api/domains/${id}/link/run`, { method: "POST" });
          setLinkNotice("Запуск добавления ссылки инициирован");
          showToast({
            type: "success",
            title: "Добавление ссылки запущено",
            message: domain?.url || undefined
          });
          await load(true);
        } catch (err: any) {
          const msg = err?.message || "Не удалось запустить добавление ссылки";
          setLinkTasksError(msg);
          showToast({
            type: "error",
            title: "Ошибка запуска",
            message: msg
          });
        } finally {
          setLinkTasksLoading(false);
        }
      },
      "Задача по ссылке уже запускается"
    );
  };

  const removeLinkTask = async () => {
    if (!id) return;
    await runLocked(
      `domain:${id}:link:remove`,
      async () => {
        const domainLinkStatus = domain?.link_status_effective || domain?.link_status;
        const linkInProgressNow =
          isLinkTaskInProgress(domainLinkStatus) ||
          linkTasks.some((task) => isLinkTaskInProgress(task.status));
        if (linkInProgressNow) {
          showToast({
            type: "error",
            title: "Задача уже выполняется",
            message: "Дождитесь завершения текущей задачи по ссылке."
          });
          return;
        }
        if (!canRemoveLink) {
          showToast({
            type: "error",
            title: "Удалять нечего",
            message: "Ссылка на сайте не найдена."
          });
          return;
        }
        if (!confirm("Удалить ссылку с сайта?")) return;
        setLinkTasksError(null);
        setLinkNotice(null);
        setLinkTasksLoading(true);
        try {
          await authFetch(`/api/domains/${id}/link/remove`, { method: "POST" });
          setLinkNotice("Запуск удаления ссылки инициирован");
          showToast({
            type: "success",
            title: "Удаление ссылки запущено",
            message: domain?.url || undefined
          });
          await load(true);
        } catch (err: any) {
          const msg = err?.message || "Не удалось запустить удаление ссылки";
          setLinkTasksError(msg);
          showToast({
            type: "error",
            title: "Ошибка удаления",
            message: msg
          });
        } finally {
          setLinkTasksLoading(false);
        }
      },
      "Удаление ссылки уже запускается"
    );
  };

  const refreshLinkTasks = async () => {
    if (!id) return;
    await runLocked(
      `domain:${id}:link:refresh`,
      async () => {
        setLinkTasksLoading(true);
        setLinkTasksError(null);
        try {
          const tasks = await authFetch<LinkTaskLite[]>(`/api/domains/${id}/links`);
          const list = Array.isArray(tasks) ? tasks : [];
          list.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
          setLinkTasks(list);
        } catch (err: any) {
          setLinkTasksError(err?.message || "Не удалось загрузить задачи ссылок");
        } finally {
          setLinkTasksLoading(false);
        }
      },
      "Обновление задач по ссылкам уже выполняется"
    );
  };

  const deleteGeneration = async (genId: string) => {
    if (!confirm("Удалить этот запуск?")) return;
    await runLocked(
      `generation:${genId}:delete`,
      async () => {
        setLoading(true);
        setError(null);
        try {
          await del(`/api/generations/${genId}`);
          await load(true);
        } catch (err: any) {
          setError(err?.message || "Не удалось удалить запуск");
        } finally {
          setLoading(false);
        }
      },
      "Удаление запуска уже выполняется"
    );
  };

  const pauseGeneration = async (genId: string) => {
    if (!confirm("Приостановить выполнение задачи?")) return;
    await runLocked(
      `generation:${genId}:pause`,
      async () => {
        setLoading(true);
        setError(null);
        try {
          await patch(`/api/generations/${genId}`, { action: "pause" });
          await load(true);
        } catch (err: any) {
          setError(err?.message || "Не удалось приостановить задачу");
        } finally {
          setLoading(false);
        }
      },
      "Пауза уже запрашивается"
    );
  };

  const resumeGeneration = async (genId: string) => {
    await runLocked(
      `generation:${genId}:resume`,
      async () => {
        setLoading(true);
        setError(null);
        try {
          await patch(`/api/generations/${genId}`, { action: "resume" });
          await load(true);
        } catch (err: any) {
          setError(err?.message || "Не удалось возобновить задачу");
        } finally {
          setLoading(false);
        }
      },
      "Возобновление уже выполняется"
    );
  };

  const cancelGeneration = async (genId: string) => {
    if (!confirm("Отменить выполнение задачи?")) return;
    await runLocked(
      `generation:${genId}:cancel`,
      async () => {
        setLoading(true);
        setError(null);
        try {
          await patch(`/api/generations/${genId}`, { action: "cancel" });
          await load(true);
        } catch (err: any) {
          setError(err?.message || "Не удалось отменить задачу");
        } finally {
          setLoading(false);
        }
      },
      "Отмена уже запрашивается"
    );
  };

  return {
    runLinkTask,
    removeLinkTask,
    refreshLinkTasks,
    handleMainAction,
    handleForceStep,
    deleteGeneration,
    pauseGeneration,
    resumeGeneration,
    cancelGeneration
  };
}
