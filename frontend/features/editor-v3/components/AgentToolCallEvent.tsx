"use client";

import { useState } from "react";
import { FiChevronDown, FiChevronRight, FiTool } from "react-icons/fi";
import type { AgentToolCall } from "../types/agent";

const TOOL_LABELS: Record<string, string> = {
  list_files: "Список файлов",
  read_file: "Читает файл",
  write_file: "Записывает файл",
  delete_file: "Удаляет файл",
  generate_image: "Генерирует изображение",
  search_in_files: "Поиск по файлам",
};

type Props = {
  toolCall: AgentToolCall;
};

export function AgentToolCallEvent({ toolCall }: Props) {
  const [open, setOpen] = useState(false);
  const label = TOOL_LABELS[toolCall.tool] || toolCall.tool;

  const inputPath =
    (toolCall.input?.path as string) ||
    (toolCall.input?.query as string) ||
    (toolCall.input?.directory as string) ||
    "";

  return (
    <div
      className={`rounded-lg border text-xs ${
        toolCall.isError
          ? "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/20"
          : "border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-800/50"
      }`}
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left"
      >
        <FiTool className={`h-3 w-3 shrink-0 ${toolCall.isError ? "text-red-500" : "text-indigo-500"}`} />
        <span className="font-medium text-slate-700 dark:text-slate-200">{label}</span>
        {inputPath && (
          <span className="truncate text-slate-400 dark:text-slate-500">{inputPath}</span>
        )}
        {!toolCall.done && (
          <span className="ml-auto inline-flex h-1.5 w-1.5 animate-pulse rounded-full bg-indigo-400" />
        )}
        {toolCall.done && (
          <span className="ml-auto">
            {open ? (
              <FiChevronDown className="h-3 w-3 text-slate-400" />
            ) : (
              <FiChevronRight className="h-3 w-3 text-slate-400" />
            )}
          </span>
        )}
      </button>

      {open && toolCall.done && (
        <div className="border-t border-slate-200 px-3 py-2 dark:border-slate-700">
          {toolCall.preview && (
            <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-all font-mono text-[11px] text-slate-600 dark:text-slate-300">
              {toolCall.preview}
            </pre>
          )}
          {!toolCall.preview && (
            <span className="text-slate-400 dark:text-slate-500">Нет вывода</span>
          )}
        </div>
      )}
    </div>
  );
}
