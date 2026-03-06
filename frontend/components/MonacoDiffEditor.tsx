"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";
import { DiffEditor } from "@monaco-editor/react";
import type { editor as MonacoEditorNs } from "monaco-editor";

type MonacoDiffEditorProps = {
  original: string;
  modified: string;
  language: string;
};

export function MonacoDiffEditor({ original, modified, language }: MonacoDiffEditorProps) {
  const editorRef = useRef<MonacoEditorNs.IStandaloneDiffEditor | null>(null);

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

  const handleMount = useCallback((editor: MonacoEditorNs.IStandaloneDiffEditor) => {
    editorRef.current = editor;
  }, []);

  // Dispose the editor instance before React unmounts the DOM node,
  // preventing "TextModel got disposed before DiffEditorWidget model got reset".
  useEffect(() => {
    return () => {
      if (editorRef.current) {
        try {
          editorRef.current.dispose();
        } catch {
          // already disposed — safe to ignore
        }
        editorRef.current = null;
      }
    };
  }, []);

  return (
    <div className="h-[46vh] w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800">
      <DiffEditor
        height="100%"
        language={language}
        original={original}
        modified={modified}
        options={options}
        onMount={handleMount}
      />
    </div>
  );
}
