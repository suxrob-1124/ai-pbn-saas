import { useCallback, useState } from "react";

import type { AIFlowState, AIFlowStatus } from "../types/ai";

const nowISO = () => new Date().toISOString();

const initialFlowState: AIFlowState = {
  status: "idle",
};

export function useAIFlowState(initialStatus: AIFlowStatus = "idle") {
  const [flow, setFlow] = useState<AIFlowState>({ ...initialFlowState, status: initialStatus });

  const setStatus = useCallback((status: AIFlowStatus, message?: string) => {
    setFlow((prev) => ({
      status,
      message,
      startedAt: status === "idle" ? undefined : prev.startedAt,
      finishedAt: status === "done" || status === "error" ? nowISO() : undefined,
      error: status === "error" ? prev.error : undefined,
    }));
  }, []);

  const start = useCallback((message?: string, status: AIFlowStatus = "sending") => {
    setFlow({
      status,
      message,
      startedAt: nowISO(),
      finishedAt: undefined,
      error: undefined,
    });
  }, []);

  const fail = useCallback((error: unknown, message = "Операция завершилась ошибкой") => {
    const text = error instanceof Error ? error.message : String(error || "");
    setFlow((prev) => ({
      status: "error",
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: nowISO(),
      error: text || "unknown error",
    }));
  }, []);

  const finish = useCallback((message = "Готово", status: AIFlowStatus = "done") => {
    setFlow((prev) => ({
      status,
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: nowISO(),
      error: undefined,
    }));
  }, []);

  const reset = useCallback(() => {
    setFlow(initialFlowState);
  }, []);

  return {
    flow,
    setStatus,
    start,
    fail,
    finish,
    reset,
  };
}
