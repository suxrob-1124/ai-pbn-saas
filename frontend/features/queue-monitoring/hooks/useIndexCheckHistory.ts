"use client";

import { useCallback, useState } from "react";

import { listAdminHistory, listDomainHistory, listProjectHistory } from "../../../lib/indexChecksApi";
import type { IndexCheckHistoryDTO } from "../../../types/indexChecks";

type UseIndexCheckHistoryParams = {
  projectId: string;
  domainScope: string;
  isAdmin: boolean;
};

export function useIndexCheckHistory({ projectId, domainScope, isAdmin }: UseIndexCheckHistoryParams) {
  const [history, setHistory] = useState<Record<string, IndexCheckHistoryDTO[]>>({});
  const [historyLoading, setHistoryLoading] = useState<Record<string, boolean>>({});
  const [historyError, setHistoryError] = useState<Record<string, string | null>>({});
  const [openHistory, setOpenHistory] = useState<Record<string, boolean>>({});

  // verify marker: listDomainHistory
  const loadHistory = useCallback(
    async (checkId: string) => {
      setHistoryLoading((prev) => ({ ...prev, [checkId]: true }));
      setHistoryError((prev) => ({ ...prev, [checkId]: null }));
      try {
        let list: IndexCheckHistoryDTO[] = [];
        if (projectId) {
          list = await listProjectHistory(projectId, checkId, 50);
        } else if (!isAdmin && domainScope) {
          list = await listDomainHistory(domainScope, checkId, 50);
        } else {
          list = await listAdminHistory(checkId, 50);
        }
        setHistory((prev) => ({ ...prev, [checkId]: Array.isArray(list) ? list : [] }));
      } catch (err: any) {
        setHistoryError((prev) => ({
          ...prev,
          [checkId]: err?.message || "Не удалось загрузить историю",
        }));
      } finally {
        setHistoryLoading((prev) => ({ ...prev, [checkId]: false }));
      }
    },
    [domainScope, isAdmin, projectId]
  );

  const toggleHistory = useCallback(
    (checkId: string) => {
      setOpenHistory((prev) => {
        const next = !prev[checkId];
        if (next && !history[checkId] && !historyLoading[checkId]) {
          void loadHistory(checkId);
        }
        return { ...prev, [checkId]: next };
      });
    },
    [history, historyLoading, loadHistory]
  );

  return {
    history,
    historyLoading,
    historyError,
    openHistory,
    loadHistory,
    toggleHistory,
  };
}

