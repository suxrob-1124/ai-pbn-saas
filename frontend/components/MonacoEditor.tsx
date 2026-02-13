"use client";

import { useMemo } from "react";
import Editor, { type OnMount } from "@monaco-editor/react";
import type { editor as MonacoEditorNs } from "monaco-editor";

type MonacoEditorProps = {
  content: string;
  language: string;
  readOnly: boolean;
  scrollLine?: number;
  onChange: (value: string) => void;
};

export function MonacoEditor({ content, language, readOnly, scrollLine, onChange }: MonacoEditorProps) {
  const options = useMemo<MonacoEditorNs.IStandaloneEditorConstructionOptions>(
    () => ({
      readOnly,
      minimap: { enabled: true },
      lineNumbers: "on",
      automaticLayout: true,
      tabSize: 2,
      smoothScrolling: true,
      fontSize: 13,
      wordWrap: "on",
      scrollBeyondLastLine: false
    }),
    [readOnly]
  );

  const handleMount: OnMount = (editor) => {
    if (!scrollLine || Number.isNaN(scrollLine) || scrollLine < 1) {
      return;
    }
    editor.revealLineInCenter(scrollLine);
    editor.setPosition({ lineNumber: scrollLine, column: 1 });
    editor.focus();
  };

  return (
    <div className="h-[62vh] w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800">
      <Editor
        height="100%"
        language={language}
        value={content}
        onMount={handleMount}
        onChange={(value) => onChange(value ?? "")}
        options={options}
      />
    </div>
  );
}
