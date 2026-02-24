import { useCallback, useState } from "react";

export type FlowStatus = "idle" | "validating" | "sending" | "done" | "error";

export type FlowState = {
  status: FlowStatus;
  message?: string;
  startedAt?: string;
  finishedAt?: string;
  error?: string;
};

const nowISO = () => new Date().toISOString();

export function useFlowState() {
  const [flow, setFlow] = useState<FlowState>({ status: "idle" });

  const validating = useCallback((message: string) => {
    setFlow({
      status: "validating",
      message,
      startedAt: nowISO(),
      finishedAt: undefined,
      error: undefined
    });
  }, []);

  const sending = useCallback((message: string) => {
    setFlow((prev) => ({
      status: "sending",
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: undefined,
      error: undefined
    }));
  }, []);

  const done = useCallback((message: string) => {
    setFlow((prev) => ({
      status: "done",
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: nowISO(),
      error: undefined
    }));
  }, []);

  const fail = useCallback((message: string, err?: unknown) => {
    const details = err instanceof Error ? err.message : typeof err === "string" ? err : undefined;
    setFlow((prev) => ({
      status: "error",
      message,
      startedAt: prev.startedAt || nowISO(),
      finishedAt: nowISO(),
      error: details
    }));
  }, []);

  const reset = useCallback(() => {
    setFlow({ status: "idle" });
  }, []);

  return { flow, validating, sending, done, fail, reset };
}

