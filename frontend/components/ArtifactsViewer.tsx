"use client";

import { useMemo, useState } from "react";
import { FiChevronDown, FiChevronRight } from "react-icons/fi";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

type ArtifactRecord = Record<string, any>;
type ArtifactType = "csv" | "contents" | "json" | "text" | "svg" | "binary";

// Группировка артефактов по этапам (в строгом порядке)
const ARTIFACT_GROUPS = [
  { id: "final", name: "🚀 Итог сайта", order: ["final_html", "zip_archive"] },
  { id: "publish", name: "📦 Публикация и деплой", order: ["published_path", "deployment_mode", "deployment_status", "file_count", "total_size_bytes"] },
  { id: "content", name: "🧠 Контент и структура", order: ["technical_spec", "content_markdown", "design_system", "html_raw", "css_content", "js_content", "404_html"] },
  { id: "media", name: "🖼️ Медиа и файлы", order: ["logo_svg", "image_prompts", "generated_files", "favicon_tag"] },
  { id: "webarchive", name: "🗄️ Вебархив", order: ["wayback_data", "generated_keyword", "wayback_theme_prompt", "wayback_keyword_prompt"] },
  { id: "analysis", name: "📊 Аналитика и источники", order: ["analysis_csv", "contents_txt", "serp_data", "competitor_analysis", "llm_analysis"] },
  { id: "prompts", name: "📝 Промпты и резолв", order: ["llm_analysis_prompt", "technical_spec_prompt", "content_generation_prompt", "design_architecture_prompt", "logo_prompt", "html_generation_prompt", "css_generation_prompt", "js_generation_prompt", "404_page_prompt", "prompt_trace", "legacy_decode_meta"] },
  { id: "audit", name: "🧪 Диагностика", order: ["audit_report", "audit_status", "audit_has_issues"] },
];

const ARTIFACT_ALIASES: Array<{ primary: string; alias: string }> = [
  { primary: "competitor_analysis", alias: "llm_analysis" },
];

const LABELS: Record<string, string> = {
  analysis_csv: "📊 Аналитика SERP",
  contents_txt: "🌐 HTML контент конкурентов",
  serp_data: "📦 Данные SERP",
  llm_analysis_prompt: "📝 Промпт для анализа",
  llm_analysis: "✅ Результат анализа",
  competitor_analysis: "✅ HTML отчёт конкурентов",
  technical_spec_prompt: "📝 Промпт для ТЗ",
  technical_spec: "✅ Техническое задание",
  content_generation_prompt: "📝 Промпт для генерации контента",
  content_markdown: "✅ Сгенерированный контент (Markdown)",
  design_architecture_prompt: "📝 Промпт для дизайн-системы",
  design_system: "✅ Дизайн-система (JSON)",
  llm_requests: "🔍 Запросы к LLM",
  serp_raw: "📦 Сырые данные SERP",
  logo_prompt: "📝 Промпт для логотипа",
  logo_svg: "Превью и исходник SVG логотипа",
  favicon_tag: "Тег фавикона",
  html_generation_prompt: "📝 Промпт для HTML",
  html_raw: "Готовый HTML каркас страницы",
  css_generation_prompt: "📝 Промпт для CSS",
  css_content: "Сгенерированные стили",
  js_generation_prompt: "📝 Промпт для JS",
  js_content: "Скрипты интерактивности",
  generated_files: "Список сгенерированных файлов",
  image_prompts: "Промпты для изображений",
  "404_page_prompt": "📝 Промпт для 404",
  "404_html": "Готовая 404 страница (HTML)",
  audit_report: "Отчет аудита",
  audit_status: "Статус аудита",
  audit_has_issues: "Флаг проблем",
  final_html: "Финальный HTML",
  zip_archive: "ZIP архив сайта",
  published_path: "Путь публикации",
  deployment_mode: "Режим деплоя",
  deployment_status: "Статус деплоя",
  file_count: "Количество файлов",
  total_size_bytes: "Размер сайта (байт)",
  prompt_trace: "Трассировка резолва промптов",
  legacy_decode_meta: "Метаданные legacy decode",
  wayback_data: "🗄️ Данные Wayback Machine",
  generated_keyword: "🔑 Сгенерированное ключевое слово",
  wayback_theme_prompt: "📝 Промпт извлечения темы",
  wayback_keyword_prompt: "📝 Промпт генерации ключевых слов",
};

const DESCRIPTIONS: Record<string, string> = {
  analysis_csv: "Таблица с метриками топ-10 сайтов из поисковой выдачи",
  contents_txt: "HTML-контент страниц конкурентов, извлеченный из SERP",
  serp_data: "Сырые данные SERP после нормализации",
  llm_analysis_prompt: "Промпт с подставленными данными для анализа конкурентов",
  llm_analysis: "Структурированный анализ конкурентов, выполненный LLM",
  competitor_analysis: "HTML отчёт по конкурентам",
  technical_spec_prompt: "Промпт с подставленными данными для разработки ТЗ",
  technical_spec: "Техническое задание для генерации контента на основе анализа",
  content_generation_prompt: "Промпт с подставленными данными для генерации контента (домен, язык, дата, ТЗ)",
  content_markdown: "Готовый контент в формате Markdown с YAML frontmatter, сгенерированный на основе технического задания",
  design_architecture_prompt: "Промпт с подставленными данными для разработки дизайн-системы (ID стиля, цвета, шрифта, макета)",
  design_system: "Полная дизайн-система сайта в формате JSON: стиль, цвета, шрифты, макет, элементы",
  llm_requests: "Детальная информация о всех запросах к LLM (промпты, ответы, токены)",
  serp_raw: "Необработанные данные из поисковой выдачи в формате JSON",
  logo_prompt: "Промпт для генерации SVG-логотипа",
  logo_svg: "Превью и исходник SVG логотипа",
  favicon_tag: "Готовый тег фавикона для вставки в <head>",
  html_generation_prompt: "Промпт для генерации HTML-каркаса",
  html_raw: "Готовый HTML каркас страницы",
  css_generation_prompt: "Промпт для генерации CSS",
  css_content: "Сгенерированные стили",
  js_generation_prompt: "Промпт для генерации JS",
  js_content: "Скрипты интерактивности",
  generated_files: "Итоговые файлы для публикации (zip-содержимое)",
  image_prompts: "JSON промпты для генерации изображений",
  "404_page_prompt": "Промпт с подставленными данными для генерации 404 страницы",
  "404_html": "Готовая HTML-страница 404",
  audit_report: "Отчет аудита с найденными проблемами",
  audit_status: "Итоговый статус аудита",
  audit_has_issues: "Есть ли проблемы в сборке",
  final_html: "Финальный HTML с подключёнными style.css и script.js",
  zip_archive: "Архив сайта в base64",
  published_path: "Технический путь, куда опубликованы файлы домена",
  deployment_mode: "Режим деплоя (local_mock / ssh и т.д.)",
  deployment_status: "Результат последнего publish шага",
  file_count: "Общее количество опубликованных файлов",
  total_size_bytes: "Итоговый размер опубликованных файлов в байтах",
  prompt_trace: "Источник промптов по этапам: domain/project/global",
  legacy_decode_meta: "Служебная информация о декодировании legacy-сайта",
  wayback_data: "Архивные снэпшоты с Wayback Machine: список снэпшотов и извлечённый текст",
  generated_keyword: "Тема сайта, описание, 15 кандидатов и выбранный ключевой запрос для SERP",
  wayback_theme_prompt: "Промпт для извлечения темы из архивного текста сайта",
  wayback_keyword_prompt: "Промпт для генерации ключевых слов по теме",
};

const TYPE_BY_KEY: Record<string, ArtifactType> = {
  analysis_csv: "csv",
  contents_txt: "contents",
  llm_analysis_prompt: "text",
  llm_analysis: "text",
  technical_spec_prompt: "text",
  technical_spec: "text",
  content_generation_prompt: "text",
  content_markdown: "text",
  design_architecture_prompt: "text",
  design_system: "json",
  llm_requests: "json",
  serp_raw: "json",
  serp_data: "json",
  competitor_analysis: "text",
  logo_prompt: "text",
  logo_svg: "svg",
  favicon_tag: "text",
  html_generation_prompt: "text",
  html_raw: "text",
  css_generation_prompt: "text",
  css_content: "text",
  js_generation_prompt: "text",
  js_content: "text",
  generated_files: "json",
  image_prompts: "json",
  "404_page_prompt": "text",
  "404_html": "text",
  audit_report: "json",
  audit_status: "text",
  audit_has_issues: "text",
  final_html: "text",
  zip_archive: "binary",
  published_path: "text",
  deployment_mode: "text",
  deployment_status: "text",
  file_count: "text",
  total_size_bytes: "text",
  prompt_trace: "json",
  legacy_decode_meta: "json",
};

// Определяет, является ли текст markdown
function isMarkdown(text: string): boolean {
  if (!text || typeof text !== "string") return false;
  const markdownPatterns = [
    /^#{1,6}\s+.+$/m,           // Заголовки
    /\*\*.*?\*\*/,              // Жирный текст
    /\*.*?\*/,                  // Курсив
    /\[.*?\]\(.*?\)/,          // Ссылки
    /^\s*[-*+]\s+/m,            // Маркированные списки
    /^\s*\d+\.\s+/m,            // Нумерованные списки
    /```[\s\S]*?```/,          // Блоки кода
    /`[^`]+`/,                  // Инлайн код
    /\|.*\|/,                   // Таблицы
  ];
  return markdownPatterns.some(pattern => pattern.test(text));
}

type ArtifactEntry = {
  key: string;
  label: string;
  type: ArtifactType;
  value: any;
  description?: string;
};

type ArtifactGroup = {
  id: string;
  name: string;
  entries: ArtifactEntry[];
};

export function ArtifactsViewer({ artifacts }: { artifacts?: ArtifactRecord }) {
  const groups = useMemo(() => {
    if (!artifacts || typeof artifacts !== "object") return [];
    
    // Создаем мапу для быстрого доступа (исключаем llm_requests и serp_raw - скрываем из UI)
    const artifactMap = new Map<string, any>();
    Object.entries(artifacts).forEach(([key, value]) => {
      // Скрываем llm_requests и serp_raw из UI
      if (key === "llm_requests" || key === "serp_raw") return;
      if (value !== undefined && value !== null && formatValue(value)) {
        artifactMap.set(key, value);
      }
    });

    // Убираем дубли по известным alias-ключам, если содержимое совпадает
    const normalize = (value: any) => formatValue(value).trim();
    ARTIFACT_ALIASES.forEach(({ primary, alias }) => {
      if (!artifactMap.has(primary) || !artifactMap.has(alias)) return;
      const primaryValue = artifactMap.get(primary);
      const aliasValue = artifactMap.get(alias);
      if (normalize(primaryValue) === normalize(aliasValue)) {
        artifactMap.delete(alias);
      }
    });
    
    // Группируем артефакты по этапам
    const result: ArtifactGroup[] = [];
    const added = new Set<string>();
    
    // Проходим по группам в порядке ARTIFACT_GROUPS
    ARTIFACT_GROUPS.forEach((group) => {
      const entries: ArtifactEntry[] = [];
      group.order.forEach((key) => {
        if (artifactMap.has(key)) {
          entries.push({
            key,
            label: LABELS[key] || key,
            type: TYPE_BY_KEY[key] || "text",
            value: artifactMap.get(key),
            description: DESCRIPTIONS[key],
          });
          added.add(key);
        }
      });
      
      if (entries.length > 0) {
        result.push({
          id: group.id,
          name: group.name,
          entries,
        });
      }
    });
    
    // Добавляем остальные артефакты, которых нет в группах
    const otherEntries: ArtifactEntry[] = [];
    artifactMap.forEach((value, key) => {
      if (!added.has(key)) {
        otherEntries.push({
        key,
        label: LABELS[key] || key,
        type: TYPE_BY_KEY[key] || "text",
        value,
        });
      }
    });
    
    if (otherEntries.length > 0) {
      result.push({
        id: "other",
        name: "📦 Прочие артефакты",
        entries: otherEntries,
      });
    }
    
    return result;
  }, [artifacts]);

  const [open, setOpen] = useState<Record<string, boolean>>({});
  const [parsed, setParsed] = useState<Record<string, any>>({});
  const [groupOpen, setGroupOpen] = useState<Record<string, boolean>>({});

  if (!groups.length) return null;

  const legacyMeta = (() => {
    const raw = artifacts?.legacy_decode_meta;
    if (!raw || typeof raw !== "object") return null;
    const source = typeof (raw as any).source === "string" ? (raw as any).source : "";
    const decodedAt = typeof (raw as any).decoded_at === "string" ? (raw as any).decoded_at : "";
    if (!source && !decodedAt) return null;
    return { source, decodedAt };
  })();

  const handleToggle = (entry: ArtifactEntry) => {
    setOpen((prev) => {
      const next = !prev[entry.key];
      if (next && !parsed[entry.key]) {
        setParsed((cache) => ({ ...cache, [entry.key]: parseArtifact(entry.type, entry.value) }));
      }
      return { ...prev, [entry.key]: next };
    });
  };

  const handleGroupToggle = (groupName: string) => {
    setGroupOpen((prev) => ({ ...prev, [groupName]: !prev[groupName] }));
  };

  return (
    <div className="space-y-4">
      {legacyMeta && (
        <div className="inline-flex items-center gap-2 rounded-full border border-sky-200 bg-sky-50 px-3 py-1 text-xs font-semibold text-sky-700 dark:border-sky-900 dark:bg-sky-950/30 dark:text-sky-300">
          Legacy decoded {legacyMeta.decodedAt ? `· ${new Date(legacyMeta.decodedAt).toLocaleString()}` : ""}
        </div>
      )}
      {groups.map((group) => {
        const isGroupOpen = groupOpen[group.id] ?? (group.id === "final");
        return (
          <div key={group.id} className="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/40">
            <button
              onClick={() => handleGroupToggle(group.id)}
              className="w-full flex items-center justify-between px-4 py-3 text-left"
            >
              <h3 className="text-base font-semibold text-slate-700 dark:text-slate-200">
                {isGroupOpen ? <FiChevronDown className="inline mr-2" /> : <FiChevronRight className="inline mr-2" />}
                {group.name}
              </h3>
            </button>
            {isGroupOpen && (
              <div className="px-4 pb-4 space-y-2">
                {group.entries.map((entry) => {
        const textValue = formatValue(entry.value);
        const isOpen = open[entry.key] ?? false;
        const content = parsed[entry.key];
        return (
                    <div key={entry.key} className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white/60 dark:bg-slate-800/40">
                      <div className="px-3 py-2">
              <button
                onClick={() => handleToggle(entry)}
                          className="flex items-center gap-2 text-left text-sm font-medium text-slate-700 dark:text-slate-200"
              >
                {isOpen ? <FiChevronDown /> : <FiChevronRight />} {entry.label}
              </button>
            </div>
            {isOpen && (
                        <div className="px-3 pb-3">
                {renderArtifactContent(entry.type, content ?? parseArtifact(entry.type, entry.value), textValue, entry.key, entry.value)}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function renderArtifactContent(type: ArtifactType, parsed: any, fallbackText: string, entryKey?: string, entryValue?: any) {
  // Спец-рендер списка файлов
  if (entryKey === "generated_files") {
    return <GeneratedFilesViewer value={parsed ?? entryValue} />;
  }
  if (entryKey === "final_html") {
    return <FinalHTMLViewer html={fallbackText} />;
  }

  switch (type) {
    case "csv":
      if (!parsed || !parsed.headers) return <pre className="text-xs overflow-auto">{fallbackText}</pre>;
      return <CSVTable headers={parsed.headers} rows={parsed.rows} meta={parsed.meta} />;
    case "contents":
      if (!parsed || !Array.isArray(parsed) || parsed.length === 0) return <pre className="text-xs overflow-auto">{fallbackText}</pre>;
      return <ContentsTabs sections={parsed} />;
    case "json": {
      let jsonContent: any;
      try {
        if (parsed) {
          jsonContent = parsed;
        } else if (typeof fallbackText === "string") {
          jsonContent = JSON.parse(fallbackText);
        } else {
          jsonContent = fallbackText;
        }
      } catch {
        jsonContent = fallbackText;
      }
      return (
        <pre className="text-xs bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-3 overflow-auto leading-relaxed font-mono">
          {JSON.stringify(jsonContent, null, 2)}
        </pre>
      );
    }
    case "svg":
      return <SvgViewer svg={parsed || fallbackText} raw={fallbackText} />;
    case "binary": {
      const size = fallbackText ? Math.round((fallbackText.length * 3) / 4 / 1024) : 0;
      return <div className="text-xs text-slate-600 dark:text-slate-300">Бинарные данные · ~{size} KB</div>;
    }
    default:
      // Для текстовых артефактов проверяем, является ли это markdown
      if (isMarkdown(fallbackText)) {
        return (
          <div className="bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-4 overflow-auto">
            <div className="markdown-content text-xs leading-relaxed space-y-2
              [&_h1]:text-base [&_h1]:font-semibold [&_h1]:mt-4 [&_h1]:mb-2 [&_h1]:text-slate-900 dark:[&_h1]:text-slate-100
              [&_h2]:text-sm [&_h2]:font-semibold [&_h2]:mt-3 [&_h2]:mb-2 [&_h2]:text-slate-900 dark:[&_h2]:text-slate-100
              [&_h3]:text-xs [&_h3]:font-semibold [&_h3]:mt-2 [&_h3]:mb-1.5 [&_h3]:text-slate-800 dark:[&_h3]:text-slate-200
              [&_h4]:text-xs [&_h4]:font-semibold [&_h4]:mt-2 [&_h4]:mb-1.5 [&_h4]:text-slate-800 dark:[&_h4]:text-slate-200
              [&_h5]:text-xs [&_h5]:font-semibold [&_h5]:mt-2 [&_h5]:mb-1.5 [&_h5]:text-slate-800 dark:[&_h5]:text-slate-200
              [&_h6]:text-xs [&_h6]:font-semibold [&_h6]:mt-2 [&_h6]:mb-1.5 [&_h6]:text-slate-800 dark:[&_h6]:text-slate-200
              [&_p]:my-2 [&_p]:text-xs [&_p]:leading-relaxed [&_p]:text-slate-700 dark:[&_p]:text-slate-300
              [&_ul]:my-2 [&_ul]:pl-4 [&_ul]:list-disc [&_ul]:space-y-1
              [&_ol]:my-2 [&_ol]:pl-4 [&_ol]:list-decimal [&_ol]:space-y-1
              [&_li]:text-xs [&_li]:my-1 [&_li]:leading-relaxed [&_li]:text-slate-700 dark:[&_li]:text-slate-300
              [&_strong]:text-xs [&_strong]:font-semibold [&_strong]:text-slate-900 dark:[&_strong]:text-slate-100
              [&_em]:text-xs [&_em]:italic [&_em]:text-slate-700 dark:[&_em]:text-slate-300
              [&_code]:text-xs [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:bg-slate-200 [&_code]:dark:bg-slate-800 [&_code]:rounded [&_code]:font-mono [&_code]:text-slate-900 dark:[&_code]:text-slate-100
              [&_pre]:text-xs [&_pre]:p-3 [&_pre]:my-2 [&_pre]:bg-slate-200 [&_pre]:dark:bg-slate-800 [&_pre]:rounded-lg [&_pre]:overflow-x-auto [&_pre]:font-mono
              [&_pre_code]:bg-transparent [&_pre_code]:p-0
              [&_blockquote]:my-2 [&_blockquote]:pl-3 [&_blockquote]:border-l-4 [&_blockquote]:border-slate-300 [&_blockquote]:dark:border-slate-600 [&_blockquote]:italic [&_blockquote]:text-slate-600 dark:[&_blockquote]:text-slate-400
              [&_table]:text-xs [&_table]:w-full [&_table]:my-2 [&_table]:border-collapse
              [&_th]:p-2 [&_th]:border [&_th]:border-slate-300 [&_th]:dark:border-slate-600 [&_th]:bg-slate-200 [&_th]:dark:bg-slate-800 [&_th]:text-left [&_th]:font-semibold
              [&_td]:p-2 [&_td]:border [&_td]:border-slate-300 [&_td]:dark:border-slate-600
              [&_a]:text-xs [&_a]:text-blue-600 [&_a]:dark:text-blue-400 [&_a]:underline
              [&_hr]:my-3 [&_hr]:border-slate-300 [&_hr]:dark:border-slate-600">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{fallbackText}</ReactMarkdown>
            </div>
          </div>
        );
      }
      return <pre className="text-xs bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-3 overflow-auto whitespace-pre-wrap leading-relaxed">{fallbackText}</pre>;
  }
}

function SvgViewer({ svg, raw }: { svg: string; raw: string }) {
  const [view, setView] = useState<"preview" | "code">("preview");
  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <button
          onClick={() => setView("preview")}
          className={`rounded-lg px-3 py-1 text-xs font-semibold border ${
            view === "preview"
              ? "bg-indigo-600 border-indigo-600 text-white"
              : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-200"
          }`}
        >
          Превью
        </button>
        <button
          onClick={() => setView("code")}
          className={`rounded-lg px-3 py-1 text-xs font-semibold border ${
            view === "code"
              ? "bg-indigo-600 border-indigo-600 text-white"
              : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-200"
          }`}
        >
          Код
        </button>
      </div>
      {view === "preview" ? (
        <div className="rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-900 p-3 max-w-xs w-full h-48 overflow-hidden">
          <div
            className="w-full h-full flex items-center justify-center bg-[linear-gradient(45deg,#e2e8f0_25%,transparent_25%,transparent_75%,#e2e8f0_75%,#e2e8f0),linear-gradient(45deg,#e2e8f0_25%,transparent_25%,transparent_75%,#e2e8f0_75%,#e2e8f0)] bg-[length:20px_20px] bg-[position:0_0,10px_10px] dark:bg-[linear-gradient(45deg,#1e293b_25%,transparent_25%,transparent_75%,#1e293b_75%,#1e293b),linear-gradient(45deg,#1e293b_25%,transparent_25%,transparent_75%,#1e293b_75%,#1e293b)] dark:bg-[length:20px_20px] dark:bg-[position:0_0,10px_10px] [&>svg]:max-h-full [&>svg]:max-w-full [&>svg]:h-full [&>svg]:w-auto"
            dangerouslySetInnerHTML={{ __html: svg }}
          />
        </div>
      ) : (
        <pre className="text-xs bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-3 overflow-auto max-h-72 leading-relaxed font-mono whitespace-pre-wrap">
          {raw}
        </pre>
      )}
    </div>
  );
}

function FinalHTMLViewer({ html }: { html: string }) {
  const [view, setView] = useState<"preview" | "code">("preview");
  const previewStyle = { height: "80vh", minHeight: "680px" } as const;
  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <button
          onClick={() => setView("preview")}
          className={`rounded-lg px-3 py-1 text-xs font-semibold border ${
            view === "preview"
              ? "bg-indigo-600 border-indigo-600 text-white"
              : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-200"
          }`}
        >
          Preview
        </button>
        <button
          onClick={() => setView("code")}
          className={`rounded-lg px-3 py-1 text-xs font-semibold border ${
            view === "code"
              ? "bg-indigo-600 border-indigo-600 text-white"
              : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-200"
          }`}
        >
          Code
        </button>
      </div>
      {view === "preview" ? (
        <iframe
          title="Final HTML Preview"
          sandbox="allow-same-origin"
          srcDoc={html}
          style={previewStyle}
          className="w-full rounded-lg border border-slate-200 dark:border-slate-700 bg-white"
        />
      ) : (
        <pre
          style={previewStyle}
          className="text-xs bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-3 overflow-auto whitespace-pre-wrap leading-relaxed font-mono"
        >
          {html}
        </pre>
      )}
    </div>
  );
}

function CSVTable({ headers, rows, meta }: { headers: string[]; rows: string[][]; meta?: Record<string, string> }) {
  return (
    <div className="space-y-3">
      {meta && (
        <div className="flex flex-wrap gap-3 text-xs text-slate-500 dark:text-slate-400">
          {Object.entries(meta).map(([key, val]) => (
            <span key={key}>
              {key}: <strong className="text-slate-700 dark:text-slate-200">{val}</strong>
            </span>
          ))}
        </div>
      )}
      <div className="overflow-auto border border-slate-200 dark:border-slate-800 rounded-lg">
        <table className="min-w-full text-xs">
          <thead className="bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-200">
            <tr>
              {headers.map((h) => (
                <th key={h} className="px-3 py-2 text-left font-semibold">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
            {rows.map((row, idx) => (
              <tr key={idx} className="text-slate-700 dark:text-slate-100">
                {row.map((cell, cid) => (
                  <td key={`${idx}-${cid}`} className="px-3 py-2 align-top">
                    {cell || "—"}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

type ContentSection = { title: string; url: string; html: string };

type GeneratedFileItem = {
  path: string;
  content?: string;
  content_base64?: string;
};

function ContentsTabs({ sections }: { sections: ContentSection[] }) {
  const [active, setActive] = useState(0);
  const current = sections[Math.min(active, sections.length - 1)];
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
        {sections.map((section, idx) => (
          <button
            key={section.title + idx}
            onClick={() => setActive(idx)}
            className={`rounded-full px-3 py-1 text-xs font-semibold border ${
              idx === active ? "bg-indigo-600 border-indigo-600 text-white" : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-200"
            }`}
          >
            {section.title} ({section.url})
          </button>
        ))}
      </div>
      <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 p-3 max-h-[40rem] overflow-auto">
        <div className="text-xs text-slate-500 dark:text-slate-400 mb-2">{current?.url}</div>
        {current && <div className="prose prose-sm dark:prose-invert max-w-none" dangerouslySetInnerHTML={{ __html: current.html }} />}
      </div>
    </div>
  );
}

function GeneratedFilesViewer({ value }: { value: any }) {
  let files: GeneratedFileItem[] = [];
  if (Array.isArray(value)) {
    files = value as GeneratedFileItem[];
  } else if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      if (Array.isArray(parsed)) files = parsed as GeneratedFileItem[];
    } catch {
      // ignore
    }
  }

  if (!files.length) return <div className="text-xs text-slate-500 dark:text-slate-400">Файлы отсутствуют</div>;

  return (
    <div className="space-y-2">
      {files.map((f, idx) => (
        <FileItem key={`${f.path}-${idx}`} file={f} />
      ))}
    </div>
  );
}

function FileItem({ file }: { file: GeneratedFileItem }) {
  const [open, setOpen] = useState(false);
  const ext = (file.path || "").toLowerCase();
  const isImage = ext.endsWith(".webp") || ext.endsWith(".png") || ext.endsWith(".jpg") || ext.endsWith(".jpeg") || ext.endsWith(".svg");
  const isCode = ext.endsWith(".css") || ext.endsWith(".js") || ext.endsWith(".html") || ext.endsWith(".txt") || ext.endsWith(".xml") || ext.endsWith(".htaccess");

  const src = buildDataUrl(file, isImage);
  const code = open ? decodeText(file) : "";

  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/40 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="text-sm font-semibold text-slate-700 dark:text-slate-200">{file.path || "Без имени"}</div>
        {(isImage || isCode) && (
          <button
            onClick={() => setOpen((v) => !v)}
            className="text-xs rounded-lg border border-slate-200 px-2 py-1 text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          >
            {open ? "Скрыть" : "Показать"}
          </button>
        )}
      </div>
      {open && (
        <div className="mt-2">
          {isImage && src ? (
            <img src={src} alt={file.path} className="max-h-64 object-contain rounded-md border border-slate-200 dark:border-slate-800" />
          ) : isCode ? (
            <pre className="text-xs bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-3 overflow-auto whitespace-pre-wrap leading-relaxed font-mono max-h-80">
              {code || "(пусто)"}
            </pre>
          ) : (
            <div className="text-xs text-slate-500 dark:text-slate-400">Предпросмотр недоступен</div>
          )}
        </div>
      )}
    </div>
  );
}

function buildDataUrl(file: GeneratedFileItem, isImage: boolean) {
  if (!isImage) return "";
  const ext = (file.path || "").toLowerCase();
  const mime = ext.endsWith(".webp")
    ? "image/webp"
    : ext.endsWith(".png")
    ? "image/png"
    : ext.endsWith(".jpg") || ext.endsWith(".jpeg")
    ? "image/jpeg"
    : ext.endsWith(".svg")
    ? "image/svg+xml"
    : "application/octet-stream";
  if (file.content_base64) return `data:${mime};base64,${file.content_base64}`;
  if (file.content && ext.endsWith(".svg")) {
    return `data:${mime};base64,${btoa(unescape(encodeURIComponent(file.content)))}`;
  }
  return "";
}

function decodeText(file: GeneratedFileItem) {
  if (file.content) return file.content;
  if (file.content_base64) {
    try {
      return atob(file.content_base64);
    } catch {
      return "(не удалось декодировать)";
    }
  }
  return "";
}

export function LogsViewer({ logs }: { logs?: any }) {
  // Всегда показываем блок логов, даже если они пустые (для завершенных генераций)
  const [levelFilter, setLevelFilter] = useState<"all" | "error" | "warn" | "info" | "success">("all");
  const [viewMode, setViewMode] = useState<"formatted" | "raw">("formatted");
  const LEVEL_LABELS: Record<string, string> = {
    error: "ОШИБКА",
    warn: "ПРЕДУПР",
    info: "ИНФО",
    success: "УСПЕХ"
  };

  const items = useMemo(() => {
    if (!logs) return [];
    if (Array.isArray(logs)) return logs;
    if (typeof logs === "string") {
      // Если logs - это строка, пытаемся распарсить как JSON массив
      try {
        const parsed = JSON.parse(logs);
        if (Array.isArray(parsed)) return parsed;
        return [parsed];
      } catch {
        // Если не JSON, возвращаем как массив из одной строки
        return [logs];
      }
    }
    return [logs];
  }, [logs]);

  const rawText = useMemo(
    () =>
      items
        .map((entry) => {
          if (typeof entry === "string") return entry;
          try {
            return JSON.stringify(entry, null, 2);
          } catch {
            return String(entry);
          }
        })
        .join("\n"),
    [items]
  );

  const lines = useMemo(() => {
    const normalized: string[] = [];
    items.forEach((entry) => {
      let text = "";
      if (typeof entry === "string") {
        text = entry;
      } else {
        try {
          text = JSON.stringify(entry, null, 2);
        } catch {
          text = String(entry);
        }
      }
      text
        .split("\n")
        .map((line) => line.trimEnd())
        .filter((line) => line.length > 0)
        .forEach((line) => normalized.push(line));
    });
    return normalized;
  }, [items]);

  const parsedLines = useMemo(() => lines.map(parseLogLine), [lines]);
  const filteredLines = useMemo(() => {
    if (levelFilter === "all") return parsedLines;
    return parsedLines.filter((line) => line.level === levelFilter);
  }, [levelFilter, parsedLines]);

  const rawOutput = useMemo(() => {
    if (levelFilter === "all") return rawText;
    return filteredLines.map((line) => line.raw).join("\n");
  }, [filteredLines, levelFilter, rawText]);

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h4 className="text-sm font-semibold text-slate-700 dark:text-slate-200">Логи</h4>
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <div className="flex items-center gap-1">
            <FilterBadge label="Все" active={levelFilter === "all"} onClick={() => setLevelFilter("all")} />
            <FilterBadge label="Ошибка" active={levelFilter === "error"} onClick={() => setLevelFilter("error")} tone="error" />
            <FilterBadge label="Предупреждения" active={levelFilter === "warn"} onClick={() => setLevelFilter("warn")} tone="warn" />
            <FilterBadge label="Инфо" active={levelFilter === "info"} onClick={() => setLevelFilter("info")} tone="info" />
            <FilterBadge label="Успех" active={levelFilter === "success"} onClick={() => setLevelFilter("success")} tone="success" />
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setViewMode("formatted")}
              className={`rounded-full px-2 py-0.5 border text-[11px] font-semibold ${
                viewMode === "formatted"
                  ? "bg-indigo-600 border-indigo-600 text-white"
                  : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-200"
              }`}
            >
              Форматированный
            </button>
            <button
              onClick={() => setViewMode("raw")}
              className={`rounded-full px-2 py-0.5 border text-[11px] font-semibold ${
                viewMode === "raw"
                  ? "bg-indigo-600 border-indigo-600 text-white"
                  : "border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-200"
              }`}
            >
              Сырый
            </button>
          </div>
        </div>
      </div>
      <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/70 dark:bg-slate-900/40 overflow-hidden">
        {viewMode === "raw" ? (
          rawOutput ? (
            <pre className="text-xs p-3 overflow-auto text-slate-700 dark:text-slate-200 max-h-96 font-mono whitespace-pre-wrap break-all">
              {rawOutput}
            </pre>
          ) : (
            <div className="text-xs p-3 text-slate-500 dark:text-slate-400 italic">
              Логи пока недоступны
            </div>
          )
        ) : filteredLines.length > 0 ? (
          <div className="text-xs p-3 overflow-y-auto overflow-x-hidden max-h-96 font-mono space-y-1">
            {filteredLines.map((line, idx) => (
              <div key={`${line.raw}-${idx}`} className="flex flex-wrap items-start gap-x-2 gap-y-1 min-w-0">
                {line.timestamp && (
                  <span className="text-slate-400 dark:text-slate-500 shrink-0">{line.timestamp}</span>
                )}
                {line.level && (
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide shrink-0 ${levelClass(line.level)}`}>
                    {LEVEL_LABELS[line.level] || line.level}
                  </span>
                )}
                {line.step && (
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-200 shrink-0">
                    {line.step}
                  </span>
                )}
                <div className="min-w-0 flex-1 break-words">
                  {renderLogMessage(line.message)}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-xs p-3 text-slate-500 dark:text-slate-400 italic">
            Логи пока недоступны
          </div>
        )}
      </div>
    </div>
  );
}

function FilterBadge({
  label,
  active,
  onClick,
  tone,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  tone?: "error" | "warn" | "info" | "success";
}) {
  const toneClass =
    tone === "error"
      ? "border-red-200 text-red-700 dark:border-red-700 dark:text-red-200"
      : tone === "warn"
      ? "border-amber-200 text-amber-700 dark:border-amber-700 dark:text-amber-200"
      : tone === "info"
      ? "border-blue-200 text-blue-700 dark:border-blue-700 dark:text-blue-200"
      : tone === "success"
      ? "border-emerald-200 text-emerald-700 dark:border-emerald-700 dark:text-emerald-200"
      : "border-slate-200 text-slate-600 dark:border-slate-700 dark:text-slate-200";

  return (
    <button
      onClick={onClick}
      className={`rounded-full px-2 py-0.5 border text-[11px] font-semibold ${
        active ? "bg-indigo-600 border-indigo-600 text-white" : toneClass
      }`}
    >
      {label}
    </button>
  );
}

type ParsedLogLine = {
  raw: string;
  timestamp?: string;
  level?: "error" | "warn" | "info" | "success";
  step?: string;
  message: string;
};

const TIMESTAMP_RE = /^(\d{4}-\d{2}-\d{2}T[\d:.+-Z]+)\s+(.*)$/;
const STEP_RE = /(?:step=|step\s+'|step\s+)([a-z0-9_]+)/i;

function parseLogLine(raw: string): ParsedLogLine {
  let timestamp: string | undefined;
  let message = raw;
  const tsMatch = raw.match(TIMESTAMP_RE);
  if (tsMatch) {
    timestamp = tsMatch[1];
    message = tsMatch[2];
  }

  const stepMatch = message.match(STEP_RE);
  const step = stepMatch ? stepMatch[1] : undefined;

  const lower = message.toLowerCase();
  let level: ParsedLogLine["level"];
  if (/(error|failed|fatal|ошибка|invalid|denied)/i.test(lower)) level = "error";
  else if (/(warn|warning|skip|pause|cancell|отмена|пропуск)/i.test(lower)) level = "warn";
  else if (/(completed successfully|success|готово|успешно|завершен)/i.test(lower)) level = "success";
  else if (/(start|executing|checking|processing|начало|используется|создано|generated)/i.test(lower)) level = "info";

  return { raw, timestamp, level, step, message };
}

function levelClass(level: ParsedLogLine["level"]): string {
  switch (level) {
    case "error":
      return "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200";
    case "warn":
      return "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200";
    case "success":
      return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200";
    default:
      return "bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-200";
  }
}

function renderLogMessage(message: string) {
  const jsonBlock = extractJsonBlock(message);
  if (!jsonBlock) {
    return <span className="text-slate-700 dark:text-slate-200 whitespace-pre-wrap break-words">{message}</span>;
  }
  return (
    <div className="space-y-1 text-slate-700 dark:text-slate-200">
      {jsonBlock.prefix && <span className="whitespace-pre-wrap break-words">{jsonBlock.prefix}</span>}
      <pre className="text-[11px] bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-2 overflow-auto whitespace-pre-wrap">
        {jsonBlock.pretty}
      </pre>
      {jsonBlock.suffix && <span className="whitespace-pre-wrap break-words">{jsonBlock.suffix}</span>}
    </div>
  );
}

function extractJsonBlock(message: string): { prefix: string; pretty: string; suffix: string } | null {
  const starts = [];
  for (let i = 0; i < message.length; i++) {
    const char = message[i];
    if (char === "{" || char === "[") starts.push(i);
  }

  for (const start of starts) {
    const end = findJsonEnd(message, start);
    if (end === -1) continue;
    const candidate = message.slice(start, end + 1);
    try {
      const parsed = JSON.parse(candidate);
      const pretty = JSON.stringify(parsed, null, 2);
      return {
        prefix: message.slice(0, start).trimEnd(),
        pretty,
        suffix: message.slice(end + 1).trimStart(),
      };
    } catch {
      continue;
    }
  }

  return null;
}

function findJsonEnd(text: string, start: number): number {
  const openChar = text[start];
  const closeChar = openChar === "{" ? "}" : "]";
  let depth = 0;
  let inString = false;
  let escaped = false;

  for (let i = start; i < text.length; i++) {
    const char = text[i];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (char === "\\") {
      escaped = true;
      continue;
    }
    if (char === "\"") {
      inString = !inString;
      continue;
    }
    if (inString) continue;
    if (char === openChar) {
      depth++;
    } else if (char === closeChar) {
      depth--;
      if (depth === 0) return i;
    } else if (char === "{" || char === "[") {
      depth++;
    } else if (char === "}" || char === "]") {
      depth--;
      if (depth === 0) return i;
    }
  }
  return -1;
}

function parseArtifact(type: ArtifactType, raw: any) {
  const text = formatValue(raw);
  if (!text) return null;
  switch (type) {
    case "csv":
      return parseCSV(text);
    case "contents":
      return parseContents(text);
    case "json":
      try {
        return typeof raw === "string" ? JSON.parse(raw) : raw;
      } catch {
        try {
          return JSON.parse(text);
        } catch {
          return text;
        }
      }
    default:
      return text;
  }
}

function parseCSV(text: string) {
  const rows: string[][] = [];
  let current: string[] = [];
  let field = "";
  let inQuotes = false;

  const pushField = () => {
    current.push(field);
    field = "";
  };

  const pushRow = () => {
    if (current.length > 0) {
      rows.push(current);
    }
    current = [];
  };

  for (let i = 0; i < text.length; i++) {
    const char = text[i];
    if (inQuotes) {
      if (char === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += char;
      }
    } else {
      if (char === '"') {
        inQuotes = true;
      } else if (char === ",") {
        pushField();
      } else if (char === "\n") {
        pushField();
        pushRow();
      } else if (char === "\r") {
        continue;
      } else {
        field += char;
      }
    }
  }
  if (inQuotes) {
    pushField();
  }
  if (current.length) {
    pushRow();
  }
  if (!rows.length) return null;
  const headers = rows[0];
  const dataRows = rows.slice(1).filter((r) => r.some((cell) => cell.trim() !== ""));

  const meta: Record<string, string> = {};
  for (const row of dataRows) {
    if (row[0] === "AGGREGATES") {
      row.forEach((val, idx) => {
        if (idx === 0 || !val) return;
        meta[headers[idx] || `col${idx}`] = val;
      });
    }
  }
  const filteredRows = dataRows.filter((row) => row[0] !== "AGGREGATES");
  return { headers, rows: filteredRows, meta: Object.keys(meta).length ? meta : undefined };
}

function parseContents(text: string): ContentSection[] {
  const regex = /--- САЙТ (\d+) \(URL: ([^)]+)\) ---\s*([\s\S]*?)(?=(?:--- САЙТ \d+ \(URL: [^)]+\) ---)|$)/g;
  const sections: ContentSection[] = [];
  let match: RegExpExecArray | null;
  while ((match = regex.exec(text)) !== null) {
    sections.push({
      title: `Сайт ${match[1]}`,
      url: match[2].trim(),
      html: match[3].trim() || "[ОСНОВНОЙ HTML-КОНТЕНТ НЕ ИЗВЛЕЧЕН]",
    });
  }
  if (!sections.length && text.trim()) {
    sections.push({ title: "Контент", url: "-", html: text });
  }
  return sections;
}

function formatValue(value: any): string {
  if (value == null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
