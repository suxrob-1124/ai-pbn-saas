import { useCallback, useState } from "react";
import { toDiagnosticsText, type QueueMonitoringFlowStatus } from "../services/i18n-ru";

export type QueueMonitoringFlowState = {
  status: QueueMonitoringFlowStatus;
  message?: string;
  startedAt?: string;
  finishedAt?: string;
  diagnostics?: string;
};

const nowISO = () => new Date().toISOString();

export function useFlowState() {
  const [flow, setFlow] = useState<QueueMonitoringFlowState>({ status: "idle" });

  const validating = useCallback((message: string) => {
    setFlow({
      status: "validating",
      message,
      startedAt: nowISO(),
      finishedAt: undefined,
      diagnostics: undefined
    });
  }, []);

  const sending = useCallback((message: string) => {
    setFlow((prev) => ({
      status: "sending",
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: undefined,
      diagnostics: undefined
    }));
  }, []);

  const done = useCallback((message: string) => {
    setFlow((prev) => ({
      status: "done",
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: nowISO(),
      diagnostics: undefined
    }));
  }, []);

  const fail = useCallback((message: string, err?: unknown) => {
    setFlow((prev) => ({
      status: "error",
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: nowISO(),
      diagnostics: toDiagnosticsText(err)
    }));
  }, []);

  const reset = useCallback(() => {
    setFlow({ status: "idle" });
  }, []);

  return { flow, validating, sending, done, fail, reset };
}
