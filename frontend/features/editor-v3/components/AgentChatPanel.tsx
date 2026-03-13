"use client";

import { useCallback, useEffect, useRef, useState } from "react";

let msgCounter = 0;
import { FiClock, FiSend, FiSquare, FiX } from "react-icons/fi";
import type { AgentChatMessage, AgentContextHint } from "../types/agent";
import type { UseAgentSessionResult } from "../hooks/useAgentSession";
import { authFetch } from "@/lib/http";
import { AgentMessage } from "./AgentMessage";
import { AgentSuggestedPrompts } from "./AgentSuggestedPrompts";
import { AgentSessionRollback } from "./AgentSessionRollback";
import { AgentSessionHistory } from "./AgentSessionHistory";

type Tab = "chat" | "history";

type Props = {
  domainId: string;
  currentFilePath?: string;
  session: UseAgentSessionResult;
  onClose: () => void;
  onFilesRefresh?: (changedPath?: string) => Promise<void> | void;
};

export function AgentChatPanel({ domainId, currentFilePath, session, onClose, onFilesRefresh }: Props) {
  const { status, messages, sessionId, snapshotId, sendMessage, stop, clearMessages, reconnect, loadHistory, getSavedSession } = session;

  const [input, setInput] = useState("");
  const [includeFile, setIncludeFile] = useState(false);
  const [tab, setTab] = useState<Tab>("chat");
  const [rollbackNotice, setRollbackNotice] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const isRunning = status === "running";

  // Auto-reconnect to active session on mount
  useEffect(() => {
    const saved = getSavedSession?.();
    if (!saved || saved.domainId !== domainId || saved.status !== "running") return;
    if (status !== "idle" || sessionId) return;
    reconnect(saved.sessionId, domainId);
  }, []); // only on mount

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (tab === "chat") {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [messages, tab]);

  // Keyboard shortcut: Escape closes panel
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !isRunning) {
        onClose();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isRunning, onClose]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || isRunning) return;
    setInput("");
    setTab("chat");

    const context: AgentContextHint | undefined =
      includeFile && currentFilePath
        ? { current_file: currentFilePath, include_current_file: true }
        : undefined;

    await sendMessage(text, domainId, context);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleSuggestSelect = (prompt: string) => {
    setInput(prompt);
    textareaRef.current?.focus();
  };

  const handleRolledBack = (restoredCount: number, deletedCount: number) => {
    void onFilesRefresh?.();
    clearMessages();
    const parts: string[] = [];
    if (restoredCount > 0) parts.push(`восстановлено ${restoredCount} файл(ов)`);
    if (deletedCount > 0) parts.push(`удалено ${deletedCount} файл(ов) агента`);
    setRollbackNotice(parts.length > 0 ? `Откат выполнен: ${parts.join(", ")}.` : "Откат выполнен.");
    setTab("chat");
  };

  const handleLoadSession = useCallback(async (loadSessionId: string) => {
    const data = await authFetch<{ chat_log: any[]; status: string; snapshot_tag?: string; is_active?: boolean }>(
      `/api/domains/${domainId}/agent/${loadSessionId}`
    );
    if (!data) return;

    // If the session is still running, reconnect to it instead of loading static history
    if (data.is_active) {
      setTab("chat");
      await reconnect(loadSessionId, domainId);
      return;
    }

    if (data.chat_log) {
      const msgs: AgentChatMessage[] = data.chat_log.map((entry: any) => ({
        id: entry.id || String(++msgCounter),
        role: entry.role,
        text: entry.text || "",
        toolCalls: (entry.tool_calls || []).map((tc: any) => ({
          id: tc.id,
          tool: tc.tool,
          input: tc.input,
          preview: tc.preview,
          done: tc.done,
          isError: tc.is_error,
        })),
        status: entry.status,
        filesChanged: entry.files_changed,
        error: entry.error,
      }));
      loadHistory(msgs, loadSessionId, data.snapshot_tag || null);
      setTab("chat");
    }
  }, [domainId, loadHistory, reconnect]);

  const showRollback = Boolean(snapshotId && sessionId && !isRunning && status !== "idle");

  return (
    <div className="flex flex-col rounded-xl border border-slate-200 bg-white shadow dark:border-slate-800 dark:bg-slate-900/60 h-[600px]">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3 dark:border-slate-800">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-slate-900 dark:text-white">✦ AI Агент</span>
          {isRunning && (
            <span className="inline-flex items-center gap-1 rounded-full bg-indigo-50 px-2 py-0.5 text-[11px] font-medium text-indigo-600 dark:bg-indigo-950/40 dark:text-indigo-300">
              <span className="inline-flex h-1.5 w-1.5 animate-pulse rounded-full bg-indigo-500" />
              Работает
            </span>
          )}
          {showRollback && sessionId && (
            <AgentSessionRollback
              domainId={domainId}
              sessionId={sessionId}
              onRolledBack={handleRolledBack}
            />
          )}
        </div>
        <div className="flex items-center gap-1">
          {/* Tab toggle */}
          <button
            type="button"
            onClick={() => setTab(tab === "chat" ? "history" : "chat")}
            className={`rounded-md p-1.5 text-xs transition-colors ${
              tab === "history"
                ? "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
                : "text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200"
            }`}
            title="История сессий"
          >
            <FiClock className="h-3.5 w-3.5" />
          </button>
          {messages.length > 0 && !isRunning && tab === "chat" && (
            <button
              type="button"
              onClick={clearMessages}
              className="rounded-md px-2 py-1 text-xs text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200"
            >
              Очистить
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200"
            title="Закрыть (Escape)"
          >
            <FiX className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Body */}
      {tab === "history" ? (
        <div className="flex-1 overflow-y-auto p-4">
          <AgentSessionHistory
            domainId={domainId}
            onLoadSession={handleLoadSession}
            onRolledBack={handleRolledBack}
          />
        </div>
      ) : (
        <>
          {/* Messages */}
          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            {rollbackNotice && (
              <div className="flex items-start justify-between gap-2 rounded-lg border border-green-200 bg-green-50 px-3 py-2 text-xs text-green-700 dark:border-green-800 dark:bg-green-950/30 dark:text-green-300">
                <span>{rollbackNotice}</span>
                <button type="button" onClick={() => setRollbackNotice(null)} className="shrink-0 text-green-500 hover:text-green-700 dark:text-green-400 dark:hover:text-green-200">✕</button>
              </div>
            )}
            {messages.length === 0 ? (
              <AgentSuggestedPrompts onSelect={handleSuggestSelect} />
            ) : (
              messages.map((msg) => (
                <AgentMessage key={msg.id} message={msg} domainId={domainId} />
              ))
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Input area */}
          <div className="border-t border-slate-200 p-3 dark:border-slate-800">
            {currentFilePath && (
              <label className="mb-2 flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400 cursor-pointer select-none">
                <input
                  type="checkbox"
                  className="rounded"
                  checked={includeFile}
                  onChange={(e) => setIncludeFile(e.target.checked)}
                />
                Передать текущий файл ({currentFilePath})
              </label>
            )}
            <div className="flex items-end gap-2">
              <textarea
                ref={textareaRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Что нужно сделать? (Enter — отправить, Shift+Enter — новая строка)"
                disabled={isRunning}
                rows={3}
                className="flex-1 resize-none rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-white dark:placeholder:text-slate-500"
              />
              {isRunning ? (
                <button
                  type="button"
                  onClick={() => stop(domainId)}
                  className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-red-50 text-red-600 hover:bg-red-100 dark:bg-red-950/30 dark:text-red-400 dark:hover:bg-red-950/50"
                  title="Остановить"
                >
                  <FiSquare className="h-4 w-4" />
                </button>
              ) : (
                <button
                  type="button"
                  onClick={handleSend}
                  disabled={!input.trim()}
                  className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-indigo-600 text-white hover:bg-indigo-500 disabled:opacity-40 dark:bg-indigo-700 dark:hover:bg-indigo-600"
                  title="Отправить (Enter)"
                >
                  <FiSend className="h-4 w-4" />
                </button>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
