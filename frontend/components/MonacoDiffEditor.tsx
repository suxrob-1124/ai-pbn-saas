"use client";

import { useEffect, useId, useMemo, useRef } from "react";
import { DiffEditor } from "@monaco-editor/react";
import type { editor as MonacoEditorNs } from "monaco-editor";

type MonacoDiffEditorProps = {
  original: string;
  modified: string;
  language: string;
};

export function MonacoDiffEditor({ original, modified, language }: MonacoDiffEditorProps) {
  const instanceId = useId().replace(/[:]/g, "_");
  const monacoRef = useRef<any>(null);
  const originalModelPath = `inmemory://diff/${instanceId}/original.${language || "txt"}`;
  const modifiedModelPath = `inmemory://diff/${instanceId}/modified.${language || "txt"}`;
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

  useEffect(() => {
    return () => {
      const monaco = monacoRef.current;
      if (!monaco?.editor?.getModel || !monaco?.Uri?.parse) return;
      try {
        const originalModel = monaco.editor.getModel(monaco.Uri.parse(originalModelPath));
        originalModel?.dispose();
      } catch {
        // noop
      }
      try {
        const modifiedModel = monaco.editor.getModel(monaco.Uri.parse(modifiedModelPath));
        modifiedModel?.dispose();
      } catch {
        // noop
      }
    };
  }, [modifiedModelPath, originalModelPath]);

  return (
    <div className="h-[46vh] w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800">
      <DiffEditor
        height="100%"
        language={language}
        originalModelPath={originalModelPath}
        modifiedModelPath={modifiedModelPath}
        keepCurrentOriginalModel
        keepCurrentModifiedModel
        original={original}
        modified={modified}
        options={options}
        onMount={(_editor, monaco) => {
          monacoRef.current = monaco;
        }}
      />
    </div>
  );
}
