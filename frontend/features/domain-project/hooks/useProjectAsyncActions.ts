import type { Dispatch, SetStateAction } from "react";
import { del, post } from "../../../lib/http";
import { showToast } from "../../../lib/toastStore";
import { hasInsertedLink, isLinkTaskInProgress } from "../../../lib/linkTaskStatus";
import { useActionLocks } from "../../editor-v3/hooks/useActionLocks";

type ProjectLite = {
  ownerHasApiKey?: boolean;
};

type DomainLite = {
  id: string;
  url: string;
  status: string;
  main_keyword?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_status?: string;
  link_status_effective?: string;
};

export type LinkEditDraft = {
  anchor: string;
  acceptor: string;
};

type UseProjectAsyncActionsParams = {
  projectId: string;
  project: ProjectLite | null;
  domains: DomainLite[];
  domainById: Record<string, DomainLite>;
  keywordEdits: Record<string, string>;
  linkEdits: Record<string, LinkEditDraft>;
  setLoading: Dispatch<SetStateAction<boolean>>;
  setError: Dispatch<SetStateAction<string | null>>;
  setLinkLoadingId: Dispatch<SetStateAction<string | null>>;
  load: (force?: boolean) => Promise<void>;
};

const effectiveDomainLinkStatus = (domain: DomainLite | null | undefined) =>
  domain?.link_status_effective || domain?.link_status || "";

export function useProjectAsyncActions({
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
}: UseProjectAsyncActionsParams) {
  const { runLocked } = useActionLocks();

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
    if (project && project.ownerHasApiKey === false) {
      setError("API ключ не настроен у владельца проекта. Настройте ключ в профиле для запуска генерации.");
      return;
    }

    await runLocked(
      `project:${projectId}:domain:${id}:generate`,
      async () => {
        setLoading(true);
        setError(null);
        try {
          await post(`/api/domains/${id}/generate`);
          await load(true);
        } catch (err: any) {
          const errMsg = err?.message || "Не удалось запустить генерацию";
          if (errMsg.includes("API key") || errMsg.includes("api key")) {
            setError(`${errMsg} Настройте API ключ в профиле.`);
          } else {
            setError(errMsg);
          }
        } finally {
          setLoading(false);
        }
      },
      "Генерация уже запускается"
    );
  };

  const runLinkTask = async (id: string) => {
    const domain = domainById[id];
    if (!domain) return;
    const linkStatus = effectiveDomainLinkStatus(domain);
    const hasActiveLink = hasInsertedLink(linkStatus);
    if (isLinkTaskInProgress(linkStatus)) {
      showToast({
        type: "error",
        title: "Задача уже выполняется",
        message: "Дождитесь завершения текущей задачи по ссылке."
      });
      return;
    }
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

    await runLocked(
      `project:${projectId}:domain:${id}:link:run`,
      async () => {
        setLinkLoadingId(id);
        try {
          await post(`/api/domains/${id}/link/run`);
          showToast({
            type: "success",
            title: hasActiveLink ? "Ссылка обновляется" : "Ссылка добавляется",
            message: domain.url
          });
          await load(true);
        } catch (err: any) {
          showToast({
            type: "error",
            title: "Не удалось запустить ссылку",
            message: err?.message || "Попробуйте позже"
          });
        } finally {
          setLinkLoadingId(null);
        }
      },
      "Задача по ссылке уже запускается"
    );
  };

  const removeLinkTask = async (id: string) => {
    const domain = domainById[id];
    if (!domain) return;
    const linkStatus = effectiveDomainLinkStatus(domain);
    const canRemoveLink = hasInsertedLink(linkStatus) && !isLinkTaskInProgress(linkStatus);
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
    if (!canRemoveLink) {
      showToast({
        type: "error",
        title: "Удалять нечего",
        message: "Ссылка на сайте не найдена."
      });
      return;
    }
    if (!confirm(`Удалить ссылку с сайта ${domain.url}?`)) return;

    await runLocked(
      `project:${projectId}:domain:${id}:link:remove`,
      async () => {
        setLinkLoadingId(id);
        try {
          await post(`/api/domains/${id}/link/remove`);
          showToast({
            type: "success",
            title: "Ссылка удаляется",
            message: domain.url
          });
          await load(true);
        } catch (err: any) {
          showToast({
            type: "error",
            title: "Не удалось удалить ссылку",
            message: err?.message || "Попробуйте позже"
          });
        } finally {
          setLinkLoadingId(null);
        }
      },
      "Удаление ссылки уже запускается"
    );
  };

  const deleteDomain = async (id: string) => {
    if (!confirm("Удалить домен?")) return;
    await runLocked(
      `project:${projectId}:domain:${id}:delete`,
      async () => {
        setLoading(true);
        setError(null);
        try {
          await del(`/api/domains/${id}`);
          await load(true);
        } catch (err: any) {
          setError(err?.message || "Не удалось удалить домен");
        } finally {
          setLoading(false);
        }
      },
      "Удаление домена уже выполняется"
    );
  };

  return {
    runGeneration,
    runLinkTask,
    removeLinkTask,
    deleteDomain
  };
}

