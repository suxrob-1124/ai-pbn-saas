"use client";

import { useCallback, useRef, useState } from "react";
import { apiBase, refreshTokens } from "@/lib/http";
import type {
  AgentChatMessage,
  AgentContextHint,
  AgentEvent,
  AgentSessionStatus,
  AgentToolCall,
} from "../types/agent";

let msgCounter = 0;
const nextId = () => String(++msgCounter);

export type UseAgentSessionResult = {
  status: AgentSessionStatus;
  messages: AgentChatMessage[];
  sessionId: string | null;
  snapshotId: string | null;
  changedFiles: string[];
  sendMessage: (text: string, domainId: string, context?: AgentContextHint) => Promise<void>;
  stop: (domainId: string) => Promise<void>;
  clearMessages: () => void;
};

export function useAgentSession(onFileChanged?: (path: string, action: string) => void): UseAgentSessionResult {
  const [status, setStatus] = useState<AgentSessionStatus>("idle");
  const [messages, setMessages] = useState<AgentChatMessage[]>([]);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [snapshotId, setSnapshotId] = useState<string | null>(null);
  const [changedFiles, setChangedFiles] = useState<string[]>([]);

  const abortRef = useRef<AbortController | null>(null);
  // Track the current assistant message index for updates
  const assistantMsgIdRef = useRef<string | null>(null);

  const updateAssistantMsg = useCallback((updater: (msg: AgentChatMessage) => AgentChatMessage) => {
    setMessages((prev) => {
      const id = assistantMsgIdRef.current;
      if (!id) return prev;
      return prev.map((m) => (m.id === id ? updater(m) : m));
    });
  }, []);

  const sendMessage = useCallback(
    async (text: string, domainId: string, context?: AgentContextHint) => {
      if (status === "running") return;

      // Add user message
      const userMsg: AgentChatMessage = { id: nextId(), role: "user", text };
      const asstId = nextId();
      assistantMsgIdRef.current = asstId;
      const asstMsg: AgentChatMessage = {
        id: asstId,
        role: "assistant",
        text: "",
        toolCalls: [],
        status: "running",
      };
      setMessages((prev) => [...prev, userMsg, asstMsg]);
      setStatus("running");

      const body = JSON.stringify({
        message: text,
        session_id: sessionId || undefined,
        context: context || undefined,
      });

      const doFetch = async (): Promise<Response> => {
        abortRef.current = new AbortController();
        return fetch(`${apiBase()}/api/domains/${domainId}/agent`, {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body,
          signal: abortRef.current.signal,
        });
      };

      let res = await doFetch();
      if (res.status === 401) {
        await refreshTokens();
        res = await doFetch();
      }

      if (!res.ok || !res.body) {
        let errMsg = `${res.status} ${res.statusText}`;
        try {
          const d = await res.json();
          if (d?.error) errMsg = d.error;
        } catch { /* ignore */ }
        updateAssistantMsg((m) => ({ ...m, status: "error", error: errMsg }));
        setStatus("error");
        return;
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";

      const processLine = (line: string) => {
        if (!line.startsWith("data: ")) return;
        const raw = line.slice(6).trim();
        if (!raw) return;

        let event: AgentEvent;
        try {
          event = JSON.parse(raw) as AgentEvent;
        } catch {
          return;
        }

        switch (event.type) {
          case "session_start":
            setSessionId(event.session_id);
            setSnapshotId(event.snapshot_id || null);
            break;

          case "text":
            updateAssistantMsg((m) => ({ ...m, text: (m.text || "") + event.delta }));
            break;

          case "tool_start": {
            const tc: AgentToolCall = {
              id: event.id,
              tool: event.tool,
              input: event.input,
              done: false,
              isError: false,
            };
            updateAssistantMsg((m) => ({ ...m, toolCalls: [...(m.toolCalls || []), tc] }));
            break;
          }

          case "tool_done":
            updateAssistantMsg((m) => ({
              ...m,
              toolCalls: (m.toolCalls || []).map((tc) =>
                tc.id === event.id
                  ? { ...tc, preview: event.preview, done: true, isError: event.error }
                  : tc
              ),
            }));
            break;

          case "file_changed":
            setChangedFiles((prev) => {
              if (prev.includes(event.path)) return prev;
              return [...prev, event.path];
            });
            onFileChanged?.(event.path, event.action);
            break;

          case "done":
            updateAssistantMsg((m) => ({
              ...m,
              status: "done",
              filesChanged: event.files_changed || [],
            }));
            setStatus("done");
            break;

          case "error":
            updateAssistantMsg((m) => ({ ...m, status: "error", error: event.message }));
            setStatus("error");
            break;

          case "stopped":
            updateAssistantMsg((m) => ({ ...m, status: "stopped" }));
            setStatus("stopped");
            break;
        }
      };

      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() ?? "";
          for (const line of lines) {
            processLine(line);
          }
        }
        // Flush remaining
        if (buf) processLine(buf);
      } catch (err) {
        if ((err as Error)?.name !== "AbortError") {
          updateAssistantMsg((m) => ({
            ...m,
            status: "error",
            error: (err as Error)?.message || "Connection error",
          }));
          setStatus("error");
        }
      } finally {
        abortRef.current = null;
      }
    },
    [status, sessionId, onFileChanged, updateAssistantMsg]
  );

  const stop = useCallback(
    async (domainId: string) => {
      abortRef.current?.abort();
      if (sessionId) {
        try {
          await fetch(`${apiBase()}/api/domains/${domainId}/agent/${sessionId}/stop`, {
            method: "POST",
            credentials: "include",
          });
        } catch { /* ignore */ }
      }
      setStatus("stopped");
    },
    [sessionId]
  );

  const clearMessages = useCallback(() => {
    setMessages([]);
    setSessionId(null);
    setSnapshotId(null);
    setChangedFiles([]);
    setStatus("idle");
  }, []);

  return { status, messages, sessionId, snapshotId, changedFiles, sendMessage, stop, clearMessages };
}
