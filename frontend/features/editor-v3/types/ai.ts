export type AIContextMode = "auto" | "manual" | "hybrid";

export type AIFlowStatus = "idle" | "validating" | "sending" | "parsing" | "ready" | "applying" | "done" | "error";

export type AIFlowState = {
  status: AIFlowStatus;
  message?: string;
  startedAt?: string;
  finishedAt?: string;
  error?: string;
};

export type EditorModelOption = {
  value: string;
  label: string;
};

export type AIContextModeOption = {
  value: AIContextMode;
  label: string;
};
