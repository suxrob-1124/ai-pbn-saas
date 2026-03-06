"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { FiX, FiUpload, FiLoader, FiSearch, FiImage, FiCheck } from "react-icons/fi";
import { LuSparkles } from "react-icons/lu";
import { listFiles, uploadFile, generateEditorAsset, type FileListItem } from "@/lib/fileApi";
import { apiBase } from "@/lib/http";
import { encodePath } from "@/features/editor-v3/services/editorPreviewUtils";
import { showToast } from "@/lib/toastStore";
import { contentEditorRu } from "../services/i18n-content-ru";

type ImageLibraryModalProps = {
  open: boolean;
  onClose: () => void;
  domainId: string;
  onSelect: (src: string, alt: string) => void;
};

type Tab = "library" | "generate";

const t = contentEditorRu.imageLibrary;

function buildImageUrl(domainId: string, path: string): string {
  const encoded = path
    .split("/")
    .filter(Boolean)
    .map(encodeURIComponent)
    .join("/");
  return `${apiBase()}/api/domains/${domainId}/files/${encoded}?raw=1`;
}

function fileNameFromPath(path: string): string {
  return path.split("/").pop() || path;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function ImageLibraryModal({ open, onClose, domainId, onSelect }: ImageLibraryModalProps) {
  const [images, setImages] = useState<FileListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [search, setSearch] = useState("");
  const [tab, setTab] = useState<Tab>("library");
  const [dragOver, setDragOver] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // AI generation state
  const [aiPrompt, setAiPrompt] = useState("");
  const [generating, setGenerating] = useState(false);
  const [generatedSrc, setGeneratedSrc] = useState<string | null>(null);
  const [generatedAlt, setGeneratedAlt] = useState("");

  const loadImages = useCallback(async () => {
    if (!domainId) return;
    setLoading(true);
    try {
      const files = await listFiles(domainId);
      setImages(
        files
          .filter((f) => f.mimeType.startsWith("image/") && !f.deletedAt)
          .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()),
      );
    } catch {
      setImages([]);
    } finally {
      setLoading(false);
    }
  }, [domainId]);

  useEffect(() => {
    if (open) {
      void loadImages();
      setSearch("");
      setTab("library");
      setSelected(null);
      setAiPrompt("");
      setGeneratedSrc(null);
      setGeneratedAlt("");
    }
  }, [open, loadImages]);

  const filteredImages = useMemo(() => {
    if (!search.trim()) return images;
    const q = search.toLowerCase();
    return images.filter((img) => fileNameFromPath(img.path).toLowerCase().includes(q));
  }, [images, search]);

  const doUpload = useCallback(
    async (file: File) => {
      if (!domainId) return;
      setUploading(true);
      try {
        await uploadFile(domainId, file);
        await loadImages();
        showToast({ type: "success", title: t.uploadSuccess });
      } catch (err: any) {
        showToast({ type: "error", title: t.uploadError, message: err?.message || String(err) });
      } finally {
        setUploading(false);
        if (fileInputRef.current) fileInputRef.current.value = "";
      }
    },
    [domainId, loadImages],
  );

  const handleFileInput = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) void doUpload(file);
    },
    [doUpload],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const file = e.dataTransfer.files[0];
      if (file && file.type.startsWith("image/")) {
        void doUpload(file);
      }
    },
    [doUpload],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
  }, []);

  const handleInsertSelected = useCallback(() => {
    if (!selected) return;
    const img = images.find((i) => i.id === selected);
    if (!img) return;
    const src = buildImageUrl(domainId, img.path);
    const alt = fileNameFromPath(img.path).replace(/\.\w+$/, "");
    onSelect(src, alt);
  }, [selected, images, domainId, onSelect]);

  const handleDoubleClick = useCallback(
    (img: FileListItem) => {
      const src = buildImageUrl(domainId, img.path);
      const alt = fileNameFromPath(img.path).replace(/\.\w+$/, "");
      onSelect(src, alt);
    },
    [domainId, onSelect],
  );

  const handleGenerate = useCallback(async () => {
    if (!aiPrompt.trim() || !domainId) return;
    setGenerating(true);
    setGeneratedSrc(null);
    try {
      const path = `assets/content-img-${Date.now()}.webp`;
      const result = await generateEditorAsset(domainId, {
        path,
        prompt: aiPrompt,
        mime_type: "image/webp",
      });
      if (result.status === "ok") {
        const imageUrl = `${apiBase()}/api/domains/${domainId}/files/${encodePath(path)}?raw=1`;
        setGeneratedSrc(imageUrl);
        setGeneratedAlt(aiPrompt.slice(0, 80));
        void loadImages();
      } else {
        showToast({ type: "error", title: result.error_message || t.genError });
      }
    } catch (err: any) {
      showToast({ type: "error", title: t.genError, message: err?.message || String(err) });
    } finally {
      setGenerating(false);
    }
  }, [aiPrompt, domainId, loadImages]);

  const handleInsertGenerated = useCallback(() => {
    if (generatedSrc) {
      onSelect(generatedSrc, generatedAlt);
    }
  }, [generatedSrc, generatedAlt, onSelect]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  if (!open) return null;

  const selectedImg = selected ? images.find((i) => i.id === selected) : null;

  return createPortal(
    <div className="fixed inset-0 z-[9999] flex items-center justify-center p-6">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />

      {/* Modal — fixed size, never overflows viewport */}
      <div className="relative z-10 flex h-[min(600px,80vh)] w-full max-w-3xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-slate-900 shadow-2xl shadow-black/40">
        {/* ── Header ── */}
        <div className="flex shrink-0 items-center justify-between border-b border-slate-700/80 px-4 py-2.5">
          <div className="flex items-center gap-0.5 rounded-lg bg-slate-800 p-0.5">
            {(["library", "generate"] as const).map((tabKey) => (
              <button
                key={tabKey}
                type="button"
                onClick={() => setTab(tabKey)}
                className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-all ${
                  tab === tabKey
                    ? "bg-slate-700 text-white shadow-sm"
                    : "text-slate-400 hover:text-slate-200"
                }`}
              >
                {tabKey === "library" ? (
                  <>
                    <FiImage className="h-3 w-3" />
                    {t.tabLibrary}
                  </>
                ) : (
                  <>
                    <LuSparkles className="h-3 w-3" />
                    {t.tabGenerate}
                  </>
                )}
              </button>
            ))}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-800 hover:text-slate-300"
          >
            <FiX className="h-4 w-4" />
          </button>
        </div>

        {/* ── Library tab ── */}
        {tab === "library" && (
          <>
            {/* Search + upload strip */}
            <div className="flex shrink-0 items-center gap-2 border-b border-slate-800 px-4 py-2">
              <div className="relative flex-1">
                <FiSearch className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-500" />
                <input
                  type="text"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={t.searchPlaceholder}
                  className="w-full rounded-lg border border-slate-700 bg-slate-800 py-1.5 pl-8 pr-3 text-xs text-slate-200 outline-none placeholder:text-slate-500 transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30"
                />
              </div>
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading}
                className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-slate-600 bg-slate-800 px-3 py-1.5 text-xs font-medium text-slate-300 transition-colors hover:bg-slate-700 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
              >
                {uploading ? (
                  <FiLoader className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <FiUpload className="h-3.5 w-3.5" />
                )}
                {uploading ? t.uploading : t.upload}
              </button>
            </div>

            {/* Content area — two-panel: grid left, preview right */}
            <div className="flex min-h-0 flex-1">
              {/* Image grid */}
              <div
                className={`flex-1 overflow-y-auto p-3 ${dragOver ? "bg-indigo-500/5 ring-2 ring-inset ring-indigo-500/40" : ""}`}
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
              >
                {loading ? (
                  <div className="flex h-full items-center justify-center">
                    <FiLoader className="h-5 w-5 animate-spin text-slate-500" />
                    <span className="ml-2 text-sm text-slate-500">{t.loading}</span>
                  </div>
                ) : filteredImages.length === 0 ? (
                  <div
                    className="flex h-full cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed border-slate-700 transition-colors hover:border-indigo-500/60"
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <FiUpload className="mb-2 h-7 w-7 text-slate-600" />
                    <p className="text-sm text-slate-500">
                      {search.trim() ? t.noResults : t.noImages}
                    </p>
                    <p className="mt-1 text-[11px] text-slate-600">{t.dragHint}</p>
                  </div>
                ) : (
                  <div className="grid grid-cols-3 gap-2">
                    {filteredImages.map((img) => {
                      const isSelected = selected === img.id;
                      return (
                        <button
                          key={img.id}
                          type="button"
                          onClick={() => setSelected(isSelected ? null : img.id)}
                          onDoubleClick={() => handleDoubleClick(img)}
                          className={`group relative aspect-[4/3] overflow-hidden rounded-lg border-2 transition-all ${
                            isSelected
                              ? "border-indigo-500 ring-2 ring-indigo-500/30"
                              : "border-transparent hover:border-slate-600"
                          }`}
                        >
                          <img
                            src={buildImageUrl(domainId, img.path)}
                            alt={fileNameFromPath(img.path)}
                            loading="lazy"
                            className="h-full w-full object-cover"
                          />
                          {/* Hover overlay with filename */}
                          <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/30 to-transparent px-2 pb-1.5 pt-6 opacity-0 transition-opacity group-hover:opacity-100">
                            <p className="truncate text-[10px] font-medium text-white/90">
                              {fileNameFromPath(img.path)}
                            </p>
                          </div>
                          {/* Selection check */}
                          {isSelected && (
                            <div className="absolute right-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-indigo-500 shadow">
                              <FiCheck className="h-3 w-3 text-white" />
                            </div>
                          )}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* Right preview panel — shows when image selected */}
              {selectedImg && (
                <div className="flex w-56 shrink-0 flex-col border-l border-slate-800 bg-slate-900/50">
                  <div className="flex-1 overflow-y-auto p-3">
                    {/* Preview */}
                    <div className="mb-3 overflow-hidden rounded-lg border border-slate-700 bg-slate-800">
                      <img
                        src={buildImageUrl(domainId, selectedImg.path)}
                        alt={fileNameFromPath(selectedImg.path)}
                        className="w-full object-contain"
                      />
                    </div>
                    {/* Info */}
                    <div className="space-y-1.5 text-[11px]">
                      <p className="break-all font-medium text-slate-300">
                        {fileNameFromPath(selectedImg.path)}
                      </p>
                      <div className="flex flex-col gap-0.5 text-slate-500">
                        <span>{formatFileSize(selectedImg.size)}</span>
                        {selectedImg.width && selectedImg.height && (
                          <span>{selectedImg.width} x {selectedImg.height}</span>
                        )}
                        <span>{selectedImg.mimeType}</span>
                      </div>
                    </div>
                  </div>
                  {/* Insert button */}
                  <div className="shrink-0 border-t border-slate-800 p-3">
                    <button
                      type="button"
                      onClick={handleInsertSelected}
                      className="flex w-full items-center justify-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-2 text-xs font-semibold text-white transition-colors hover:bg-indigo-500"
                    >
                      {t.genInsert}
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* Bottom bar */}
            <div className="flex shrink-0 items-center justify-between border-t border-slate-800 px-4 py-1.5">
              <p className="text-[11px] text-slate-500">
                {t.totalImages}: {images.length}
                {search.trim() && filteredImages.length !== images.length && ` · ${t.shown}: ${filteredImages.length}`}
              </p>
              <p className="text-[10px] text-slate-600">{t.doubleClickHint}</p>
            </div>
          </>
        )}

        {/* ── AI Generate tab ── */}
        {tab === "generate" && (
          <div className="flex min-h-0 flex-1 flex-col overflow-y-auto p-5">
            <div className="mx-auto w-full max-w-md space-y-4">
              {/* Prompt */}
              <div>
                <label className="mb-1.5 block text-xs font-medium text-slate-400">
                  {t.genPromptLabel}
                </label>
                <textarea
                  value={aiPrompt}
                  onChange={(e) => setAiPrompt(e.target.value)}
                  placeholder={t.genPromptPlaceholder}
                  rows={3}
                  disabled={generating}
                  className="w-full rounded-xl border border-slate-700 bg-slate-800 px-3 py-2.5 text-sm text-slate-100 outline-none placeholder:text-slate-500 transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30"
                />
              </div>

              {/* Generate button */}
              <button
                type="button"
                onClick={handleGenerate}
                disabled={generating || !aiPrompt.trim()}
                className="inline-flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-indigo-600 to-violet-600 px-4 py-2.5 text-sm font-semibold text-white transition-all hover:from-indigo-500 hover:to-violet-500 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {generating ? (
                  <>
                    <FiLoader className="h-4 w-4 animate-spin" />
                    {t.genGenerating}
                  </>
                ) : (
                  <>
                    <LuSparkles className="h-4 w-4" />
                    {t.genGenerate}
                  </>
                )}
              </button>

              {/* Generated preview */}
              {generatedSrc && (
                <div className="space-y-3 rounded-xl border border-slate-700 bg-slate-800/60 p-3">
                  <img
                    src={generatedSrc}
                    alt={generatedAlt}
                    className="mx-auto max-h-64 rounded-lg object-contain"
                  />
                  <div>
                    <label className="mb-0.5 block text-[11px] text-slate-500">
                      Alt
                    </label>
                    <input
                      type="text"
                      value={generatedAlt}
                      onChange={(e) => setGeneratedAlt(e.target.value)}
                      className="w-full rounded-lg border border-slate-700 bg-slate-800 px-3 py-1.5 text-xs text-slate-200 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30"
                    />
                  </div>
                  <button
                    type="button"
                    onClick={handleInsertGenerated}
                    className="inline-flex w-full items-center justify-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-2 text-xs font-semibold text-white transition-colors hover:bg-indigo-500"
                  >
                    {t.genInsert}
                  </button>
                </div>
              )}
            </div>
          </div>
        )}

        <input
          ref={fileInputRef}
          type="file"
          accept="image/*"
          className="hidden"
          onChange={handleFileInput}
        />
      </div>
    </div>,
    document.body,
  );
}
