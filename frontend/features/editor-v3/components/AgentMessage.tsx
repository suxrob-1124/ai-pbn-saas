"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { AgentChatMessage } from "../types/agent";
import { AgentToolCallEvent } from "./AgentToolCallEvent";
import { AgentImagePreview, isImagePath } from "./AgentImagePreview";

type Props = {
  message: AgentChatMessage;
  domainId?: string;
};

export function AgentMessage({ message, domainId }: Props) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[85%] rounded-2xl rounded-br-sm bg-indigo-600 px-4 py-2.5 text-sm text-white shadow-sm">
          {message.text}
        </div>
      </div>
    );
  }

  const imageFiles = domainId
    ? (message.filesChanged || []).filter(isImagePath)
    : [];

  // Assistant message
  return (
    <div className="flex flex-col gap-2">
      {/* Tool calls */}
      {(message.toolCalls || []).map((tc) => (
        <AgentToolCallEvent key={tc.id} toolCall={tc} />
      ))}

      {/* Text content */}
      {message.text && (
        <div className="prose prose-sm dark:prose-invert max-w-none rounded-2xl rounded-bl-sm bg-white px-4 py-3 text-sm shadow-sm ring-1 ring-slate-200 dark:bg-slate-800 dark:ring-slate-700">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{message.text}</ReactMarkdown>
        </div>
      )}

      {/* Status indicators */}
      {message.status === "running" && !message.text && (message.toolCalls || []).length === 0 && (
        <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
          <span className="inline-flex h-2 w-2 animate-pulse rounded-full bg-indigo-500" />
          Думаю…
        </div>
      )}
      {message.status === "running" && message.text && (
        <div className="flex items-center gap-1.5 text-xs text-slate-400 dark:text-slate-500">
          <span className="inline-flex h-1.5 w-1.5 animate-pulse rounded-full bg-indigo-400" />
        </div>
      )}
      {message.status === "error" && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-600 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400">
          Ошибка: {message.error}
        </div>
      )}
      {message.status === "stopped" && (
        <div className="text-xs text-slate-400 dark:text-slate-500">— остановлено —</div>
      )}

      {/* Changed files */}
      {message.filesChanged && message.filesChanged.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {message.filesChanged.map((f) => (
            <span
              key={f}
              className="inline-flex items-center rounded-md bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-300 dark:ring-emerald-900"
            >
              {f}
            </span>
          ))}
        </div>
      )}

      {/* Image previews */}
      {imageFiles.length > 0 && domainId && (
        <div className="flex flex-wrap gap-2">
          {imageFiles.map((f) => (
            <AgentImagePreview key={f} domainId={domainId} filePath={f} />
          ))}
        </div>
      )}
    </div>
  );
}
