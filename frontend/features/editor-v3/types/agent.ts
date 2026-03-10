// SSE event types from /api/domains/{id}/agent

export type AgentEventSessionStart = {
  type: "session_start";
  session_id: string;
  snapshot_id: string;
  is_new: boolean;
};

export type AgentEventText = {
  type: "text";
  delta: string;
};

export type AgentEventToolStart = {
  type: "tool_start";
  id: string;
  tool: string;
  input: Record<string, unknown>;
};

export type AgentEventToolDone = {
  type: "tool_done";
  id: string;
  tool: string;
  preview: string;
  error: boolean;
};

export type AgentEventFileChanged = {
  type: "file_changed";
  path: string;
  action: "created" | "updated" | "deleted";
};

export type AgentEventDone = {
  type: "done";
  summary: string;
  files_changed: string[];
};

export type AgentEventError = {
  type: "error";
  message: string;
  rollback_available: boolean;
};

export type AgentEventStopped = {
  type: "stopped";
  message: string;
};

export type AgentEvent =
  | AgentEventSessionStart
  | AgentEventText
  | AgentEventToolStart
  | AgentEventToolDone
  | AgentEventFileChanged
  | AgentEventDone
  | AgentEventError
  | AgentEventStopped;

// A chat message as rendered in the UI
export type AgentChatMessage = {
  id: string;
  role: "user" | "assistant";
  /** Accumulated text for assistant messages */
  text?: string;
  /** List of tool calls attached to this assistant turn */
  toolCalls?: AgentToolCall[];
  /** Final done/error status for assistant turn */
  status?: "done" | "error" | "stopped" | "running";
  filesChanged?: string[];
  error?: string;
};

export type AgentToolCall = {
  id: string;
  tool: string;
  input: Record<string, unknown>;
  preview?: string;
  done: boolean;
  isError: boolean;
};

export type AgentSessionStatus = "idle" | "running" | "done" | "error" | "stopped";

export type AgentContextHint = {
  current_file?: string;
  include_current_file?: boolean;
};
