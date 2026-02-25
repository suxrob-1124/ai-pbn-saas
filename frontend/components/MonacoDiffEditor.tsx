"use client";

import { useMemo } from "react";
import { DiffEditor } from "@monaco-editor/react";
import type { editor as MonacoEditorNs } from "monaco-editor";

type MonacoDiffEditorProps = {
  original: string;
  modified: string;
  language: string;
};

export function MonacoDiffEditor({ original, modified, language }: MonacoDiffEditorProps) {
  const options = useMemo<MonacoEditorNs.IDiffEditorConstructionOptions>(
    () => ({
      readOnly: true,
      automaticLayout: true,
      renderSideBySide: true,
      minimap: { enabled: false },
      fontSize: 13,
      wordWrap: "on",
      scrollBeyondLastLine: false,
    }),
    []
  );

  return (
    <div className="h-[46vh] w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800">
      <DiffEditor
        height="100%"
        language={language}
        original={original}
        modified={modified}
        options={options}
      />
    </div>
  );
}

