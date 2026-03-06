"use client";

import { useState, useCallback, useMemo } from "react";
import { FiX } from "react-icons/fi";
import { createFileOrDir, aiCreatePage, saveFile, getFile, generateEditorAsset } from "@/lib/fileApi";
import { showToast } from "@/lib/toastStore";
import { slugToFilePath } from "../services/pageNameMapping";
import { extractContentAndTemplate, assembleFullHtml } from "../services/htmlTextExtraction";
import { extractPageMeta } from "../services/htmlMetaExtraction";
import type { SEOData } from "../types/content-editor";
import { contentEditorRu } from "../services/i18n-content-ru";

type CreatePageModalProps = {
  domainId: string;
  onClose: () => void;
  onCreated: (path: string) => void;
};

const t = contentEditorRu.createModal;
const tTpl = contentEditorRu.templates;

const TEMPLATES = [
  {
    id: "blog",
    label: tTpl.blogPost,
    icon: "\u{1F4DD}",
    prompt:
      "Создай статью для блога. Структура: заголовок H1, вступительный параграф, 3-4 подраздела с H2-заголовками, каждый с 2-3 параграфами текста. Добавь список ключевых выводов в конце.",
  },
  {
    id: "about",
    label: tTpl.aboutUs,
    icon: "\u{1F3E2}",
    prompt:
      "Создай страницу «О нас». Структура: заголовок H1 «О нашей компании», параграф о миссии, раздел «Наши ценности» с маркированным списком, раздел «Наша команда» с кратким описанием, раздел «Почему мы» с преимуществами.",
  },
  {
    id: "contacts",
    label: tTpl.contacts,
    icon: "\u{1F4DE}",
    prompt:
      "Создай страницу контактов. Структура: заголовок H1 «Контакты», параграф с приветствием, таблица с контактной информацией (телефон, email, адрес, часы работы), раздел «Как нас найти» с описанием расположения.",
  },
  {
    id: "empty",
    label: tTpl.emptyPage,
    icon: "\u{1F4C4}",
    prompt: "",
  },
];

const EMPTY_HTML_TEMPLATE = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Новая страница</title>
  <meta name="description" content="">
</head>
<body>
  <main>
    <h1>Новая страница</h1>
    <p></p>
  </main>
</body>
</html>`;

async function buildPageFromIndex(
  domainId: string,
  pageName: string,
): Promise<string | null> {
  try {
    const indexFile = await getFile(domainId, "index.html");
    const indexHtml = indexFile.content;
    if (!indexHtml.trim()) return null;

    const { template, classMap } = extractContentAndTemplate(indexHtml);
    if (!template) return null;

    const meta = extractPageMeta(indexHtml);
    const seo: SEOData = {
      title: pageName,
      description: "",
      ogTitle: "",
      ogDescription: "",
    };
    const blankContent = `<h1>${pageName}</h1>\n<p></p>`;

    return assembleFullHtml(template, blankContent, seo, meta, classMap);
  } catch {
    return null;
  }
}

export function CreatePageModal({ domainId, onClose, onCreated }: CreatePageModalProps) {
  const [slug, setSlug] = useState("");
  const [prompt, setPrompt] = useState("");
  const [creating, setCreating] = useState(false);
  const [activeTemplate, setActiveTemplate] = useState<string | null>(null);

  const filePath = useMemo(() => (slug.trim() ? slugToFilePath(slug) : ""), [slug]);

  const handleTemplateClick = useCallback((tpl: (typeof TEMPLATES)[number]) => {
    setActiveTemplate(tpl.id);
    setPrompt(tpl.prompt);
  }, []);

  const ensureParentDir = useCallback(
    async (path: string) => {
      const parts = path.split("/");
      if (parts.length <= 1) return;
      // Create each directory level
      for (let i = 1; i < parts.length; i++) {
        const dir = parts.slice(0, i).join("/");
        try {
          await createFileOrDir(domainId, { kind: "dir", path: dir });
        } catch {
          // Directory may already exist — ignore
        }
      }
    },
    [domainId],
  );

  const handleCreate = useCallback(async () => {
    if (!slug.trim()) {
      showToast({ type: "error", title: t.slugRequired });
      return;
    }

    const targetPath = slugToFilePath(slug);
    setCreating(true);

    try {
      // Ensure parent directories exist
      await ensureParentDir(targetPath);

      if (prompt.trim()) {
        const result = await aiCreatePage(domainId, {
          instruction: prompt,
          target_path: targetPath,
          with_assets: true,
          context_mode: "auto",
        });

        const mainFile = result.files.find((f) => f.path === targetPath) || result.files[0];
        if (!mainFile) {
          showToast({ type: "error", title: "AI не вернул файлов" });
          return;
        }

        // If AI returned a different path, ensure its dirs too
        if (mainFile.path !== targetPath) {
          await ensureParentDir(mainFile.path);
        }

        try {
          await createFileOrDir(domainId, {
            kind: "file",
            path: mainFile.path,
            content: mainFile.content,
            mime_type: "text/html",
          });
        } catch (err: any) {
          if (err?.status === 409) {
            await saveFile(domainId, mainFile.path, mainFile.content, "AI create page", {
              source: "ai",
            });
          } else {
            throw err;
          }
        }

        // Generate images from assets manifest (wait before navigating)
        const assets = (Array.isArray(result.assets) ? result.assets : []).filter(
          (a) => a.path && a.prompt,
        );
        if (assets.length > 0) {
          showToast({ type: "success", title: `Страница создана, генерируем ${assets.length} изобр.…` });
          let ok = 0;
          for (const asset of assets) {
            try {
              const res = await generateEditorAsset(domainId, {
                path: asset.path,
                prompt: asset.prompt,
                mime_type: asset.mime_type || "image/webp",
              });
              if (res.status === "ok") ok++;
              else {
                console.warn("[asset-gen] failed:", asset.path, res);
                showToast({ type: "error", title: `Ошибка генерации ${asset.path}`, message: res.error_message || res.status });
              }
            } catch (err: any) {
              console.error("[asset-gen] error:", asset.path, err);
              showToast({ type: "error", title: `Ошибка генерации ${asset.path}`, message: err?.message || String(err) });
            }
          }
          if (ok > 0) {
            showToast({ type: "success", title: `${ok} из ${assets.length} изображений сгенерировано` });
          }
        } else {
          showToast({ type: "success", title: `${targetPath} создана с помощью AI` });
        }
        onCreated(mainFile.path);
      } else {
        // Мгновенное создание: извлекаем шаблон из index.html
        const pageName = slug.charAt(0).toUpperCase() + slug.slice(1);
        const html = await buildPageFromIndex(domainId, pageName);

        const content = html || EMPTY_HTML_TEMPLATE.replace(
          /<title>.*?<\/title>/,
          `<title>${pageName}</title>`,
        );

        await createFileOrDir(domainId, {
          kind: "file",
          path: targetPath,
          content,
          mime_type: "text/html",
        });

        showToast({ type: "success", title: `${targetPath} создана` });
        onCreated(targetPath);
      }
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Ошибка создания страницы",
        message: err?.message || String(err),
      });
    } finally {
      setCreating(false);
    }
  }, [domainId, slug, prompt, onCreated, ensureParentDir]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="w-full max-w-lg rounded-2xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-700 dark:bg-slate-900">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-800 dark:text-slate-100">{t.title}</h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-300"
          >
            <FiX className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">
              {t.slugLabel}
            </label>
            <input
              type="text"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder={t.slugPlaceholder}
              className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
              disabled={creating}
            />
            {filePath && (
              <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">
                /{filePath}
              </p>
            )}
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">
              {t.promptLabel}
            </label>
            <textarea
              value={prompt}
              onChange={(e) => {
                setPrompt(e.target.value);
                setActiveTemplate(null);
              }}
              placeholder={t.promptPlaceholder}
              rows={4}
              className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
              disabled={creating}
            />
          </div>

          <div>
            <p className="mb-2 text-xs font-medium text-slate-500 dark:text-slate-400">
              {t.templatesLabel}
            </p>
            <div className="flex flex-wrap gap-2">
              {TEMPLATES.map((tpl) => (
                <button
                  key={tpl.id}
                  type="button"
                  onClick={() => handleTemplateClick(tpl)}
                  disabled={creating}
                  className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
                    activeTemplate === tpl.id
                      ? "border-indigo-300 bg-indigo-50 text-indigo-700 dark:border-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-400"
                      : "border-slate-200 bg-white text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
                  }`}
                >
                  <span>{tpl.icon}</span>
                  {tpl.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="mt-6 flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            disabled={creating}
            className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
          >
            {t.cancel}
          </button>
          <button
            type="button"
            onClick={handleCreate}
            disabled={creating || !slug.trim()}
            className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {creating ? t.creating : t.create}
          </button>
        </div>
      </div>
    </div>
  );
}
