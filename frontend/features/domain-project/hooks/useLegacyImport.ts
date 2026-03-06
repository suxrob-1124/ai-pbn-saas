import { useCallback, useEffect, useRef, useState } from "react";
import {
  getLegacyImportJob,
  startLegacyImport,
} from "@/lib/legacyImportApi";
import type { LegacyImportJobDetail, LegacyImportItem } from "@/types/legacyImport";

const POLL_INTERVAL = 3000;

export function useLegacyImport(projectId: string) {
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [activeJob, setActiveJob] = useState<LegacyImportJobDetail | null>(null);
  const [isImporting, setIsImporting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const poll = useCallback(async (jobId: string) => {
    try {
      const job = await getLegacyImportJob(projectId, jobId);
      setActiveJob(job);
      if (job.Status === "completed" || job.Status === "failed") {
        setIsImporting(false);
        stopPolling();
      }
    } catch (err: any) {
      setError(err?.message || "Failed to fetch import status");
    }
  }, [projectId, stopPolling]);

  const startImport = useCallback(async (domainIds: string[], force = false) => {
    setError(null);
    setIsImporting(true);
    try {
      const { job_id } = await startLegacyImport(projectId, domainIds, force);
      setActiveJobId(job_id);
      // Initial fetch
      await poll(job_id);
      // Start polling
      timerRef.current = setInterval(() => poll(job_id), POLL_INTERVAL);
    } catch (err: any) {
      setError(err?.message || "Failed to start import");
      setIsImporting(false);
    }
  }, [projectId, poll]);

  const resumePolling = useCallback((jobId: string) => {
    setActiveJobId(jobId);
    setIsImporting(true);
    setError(null);
    poll(jobId);
    timerRef.current = setInterval(() => poll(jobId), POLL_INTERVAL);
  }, [poll]);

  // Cleanup on unmount
  useEffect(() => {
    return () => stopPolling();
  }, [stopPolling]);

  const items: LegacyImportItem[] = activeJob?.items ?? [];

  return {
    startImport,
    resumePolling,
    activeJobId,
    activeJob,
    items,
    isImporting,
    error,
    stopPolling,
  };
}
