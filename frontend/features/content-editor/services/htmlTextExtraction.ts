import type { SEOData } from "../types/content-editor";
import { applyPageMeta, type PageMeta } from "./htmlMetaExtraction";
import { extractClassMap, applyClassMap, type ClassMap } from "./cssClassMap";

/**
 * «Всеядный» экстрактор текстового контента из HTML-страницы.
 *
 * Стратегия:
 * 1. Ищем <main> или <article> — если найден, берём его контент.
 * 2. Если нет — берём весь <body>, убирая очевидный «мусор»:
 *    <script>, <style>, <nav>, <noscript>, <svg>, <iframe>.
 * 3. НЕ трогаем <header> и <footer> на шаге 2, чтобы не потерять контент
 *    у страниц, где автор положил текст внутри этих тегов.
 * 4. Из найденного фрагмента оставляем только TipTap-совместимые теги.
 *
 * ВАЖНО: Все DOM-элементы создаются в «инертном» документе, чтобы браузер
 * не пытался загрузить <img src="..."> (что вызывает 404 для относительных путей).
 */

const GARBAGE_SELECTORS = "script, style, nav, noscript, svg, iframe, link[rel=stylesheet]";

const ALLOWED_TAGS = new Set([
  "h1", "h2", "h3", "h4", "h5", "h6",
  "p", "br",
  "ul", "ol", "li",
  "table", "thead", "tbody", "tfoot", "tr", "th", "td",
  "blockquote",
  "a",
  "img",
  "strong", "b", "em", "i", "u", "s", "del", "mark", "code",
  "hr",
  "div", "span", "section", "article", "main", "figure", "figcaption",
]);

function stripGarbage(root: Element) {
  for (const el of Array.from(root.querySelectorAll(GARBAGE_SELECTORS))) {
    el.remove();
  }
}

/**
 * Рекурсивно очищает DOM-дерево, оставляя только разрешённые теги.
 * Принимает `inertDoc` — документ созданный через createHTMLDocument,
 * в котором createElement("img") НЕ загружает картинки.
 */
function sanitizeNode(node: Node, inertDoc: Document): Node | null {
  if (node.nodeType === Node.TEXT_NODE) {
    return inertDoc.createTextNode(node.textContent || "");
  }

  if (node.nodeType !== Node.ELEMENT_NODE) return null;

  const el = node as Element;
  const tag = el.tagName.toLowerCase();

  if (!ALLOWED_TAGS.has(tag)) {
    const fragment = inertDoc.createDocumentFragment();
    for (const child of Array.from(el.childNodes)) {
      const clean = sanitizeNode(child, inertDoc);
      if (clean) fragment.appendChild(clean);
    }
    return fragment.childNodes.length > 0 ? fragment : null;
  }

  const clone = inertDoc.createElement(tag);

  if (tag === "a") {
    const href = el.getAttribute("href");
    if (href) clone.setAttribute("href", href);
    const target = el.getAttribute("target");
    if (target) clone.setAttribute("target", target);
  } else if (tag === "img") {
    const src = el.getAttribute("src");
    if (src) clone.setAttribute("src", src);
    const alt = el.getAttribute("alt");
    if (alt) clone.setAttribute("alt", alt);
  }

  for (const child of Array.from(el.childNodes)) {
    const clean = sanitizeNode(child, inertDoc);
    if (clean) clone.appendChild(clean);
  }

  return clone;
}

/**
 * Извлекает текстовый контент из полного HTML-документа
 * и возвращает упрощённый HTML, пригодный для TipTap.
 */
export function extractTextFromHtml(html: string): string {
  if (!html.trim()) return "";

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  let root: Element | null =
    doc.querySelector("main") || doc.querySelector("article");

  if (!root) {
    root = doc.body;
  }

  if (!root) return "";

  const clone = root.cloneNode(true) as Element;
  stripGarbage(clone);

  // Используем инертный документ — createElement("img") тут НЕ загружает картинки
  const inertDoc = document.implementation.createHTMLDocument("");
  const container = inertDoc.createElement("div");
  for (const child of Array.from(clone.childNodes)) {
    const clean = sanitizeNode(child, inertDoc);
    if (clean) container.appendChild(clean);
  }

  const result = container.innerHTML.trim();
  return result || "";
}

/**
 * Перезаписывает относительные src у <img> на полные API-URL.
 * Пропускает абсолютные URL (http/https/data).
 */
export function rewriteEditorImageUrls(html: string, apiBaseUrl: string, domainId: string): string {
  if (!html || !domainId) return html;
  return html.replace(
    /(<img\s[^>]*?\bsrc\s*=\s*["'])([^"']+)(["'])/gi,
    (_full, prefix: string, rawSrc: string, suffix: string) => {
      const src = rawSrc.trim();
      if (!src || /^(data:|https?:|blob:|\/\/)/i.test(src)) return _full;
      const normalized = src.replace(/^\.\//, "").replace(/^\/+/, "");
      if (!normalized) return _full;
      const encoded = normalized
        .split("/")
        .filter(Boolean)
        .map((part) => encodeURIComponent(part))
        .join("/");
      return `${prefix}${apiBaseUrl}/api/domains/${domainId}/files/${encoded}?raw=1${suffix}`;
    },
  );
}

/**
 * Извлекает только plain-text содержимое <body> (для промптов AI).
 */
export function extractPlainText(html: string): string {
  if (!html.trim()) return "";
  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");
  return (doc.body?.textContent || "").replace(/\s+/g, " ").trim();
}

// ---------------------------------------------------------------------------
// Шаблонная сборка: извлечение шаблона + прямая публикация без AI
// ---------------------------------------------------------------------------

const CONTENT_PLACEHOLDER = "<!--CONTENT-->";

/**
 * Извлекает контент для TipTap + шаблон (полный HTML с плейсхолдером).
 *
 * Шаблон = весь документ, но innerHTML `<main>/<article>` заменён на <!--CONTENT-->.
 * При публикации контент вставляется обратно в шаблон — без AI, мгновенно.
 */
export function extractContentAndTemplate(html: string): {
  content: string;
  template: string;
  classMap: ClassMap;
} {
  if (!html.trim()) return { content: "", template: "", classMap: {} };

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  const root =
    doc.querySelector("main") || doc.querySelector("article");

  if (!root) {
    // Нет чёткой зоны контента — шаблон невозможен, берём body как fallback
    return { content: extractTextFromHtml(html), template: "", classMap: {} };
  }

  // Извлекаем контент (та же логика что extractTextFromHtml)
  const clone = root.cloneNode(true) as Element;
  stripGarbage(clone);

  // Извлекаем карту классов ДО sanitize (пока классы ещё на месте)
  const classMap = extractClassMap(clone);

  const inertDoc = document.implementation.createHTMLDocument("");
  const container = inertDoc.createElement("div");
  for (const child of Array.from(clone.childNodes)) {
    const clean = sanitizeNode(child, inertDoc);
    if (clean) container.appendChild(clean);
  }
  const content = container.innerHTML.trim();

  // Создаём шаблон: заменяем содержимое root на плейсхолдер
  root.innerHTML = CONTENT_PLACEHOLDER;
  const template = `<!DOCTYPE html>\n${doc.documentElement.outerHTML}`;

  return { content, template, classMap };
}

/**
 * Обратная операция к `rewriteEditorImageUrls`:
 * конвертирует API URL обратно в относительные пути для сохранения.
 *
 * `http://host/api/domains/{id}/files/images%2Fhero.webp?raw=1` → `images/hero.webp`
 */
export function unrewriteEditorImageUrls(
  html: string,
  apiBaseUrl: string,
  domainId: string,
): string {
  if (!html || !domainId) return html;
  const prefix = `${apiBaseUrl}/api/domains/${domainId}/files/`;
  return html.replace(
    /(<img\s[^>]*?\bsrc\s*=\s*["'])([^"']+)(["'])/gi,
    (_full, pre: string, rawSrc: string, suf: string) => {
      if (!rawSrc.startsWith(prefix)) return _full;
      let relative = rawSrc.slice(prefix.length).replace(/\?raw=1$/, "");
      relative = relative
        .split("/")
        .filter(Boolean)
        .map(decodeURIComponent)
        .join("/");
      return `${pre}${relative}${suf}`;
    },
  );
}

/**
 * Конвертирует TipTap AI image blocks в стандартные <img>.
 * `<div data-type="ai-image-block" src="..." alt="..."></div>` → `<img src="..." alt="...">`
 * Блоки без src (незавершённая генерация) удаляются.
 */
export function convertAiBlocksToImg(html: string): string {
  if (!html.includes("ai-image-block")) return html;
  const doc = new DOMParser().parseFromString(`<body>${html}</body>`, "text/html");
  const blocks = doc.querySelectorAll('[data-type="ai-image-block"]');
  for (const block of Array.from(blocks)) {
    const src = block.getAttribute("src");
    const alt = block.getAttribute("alt") || "";
    if (src) {
      const img = doc.createElement("img");
      img.setAttribute("src", src);
      if (alt) img.setAttribute("alt", alt);
      block.replaceWith(img);
    } else {
      block.remove();
    }
  }
  return doc.body.innerHTML;
}

/**
 * Собирает финальный HTML из шаблона + отредактированного контента + SEO + мета.
 * Мгновенная замена AI-компиляции.
 */
export function assembleFullHtml(
  template: string,
  editedContent: string,
  seo: SEOData,
  meta?: PageMeta,
  classMap?: ClassMap,
): string {
  if (!template) return editedContent;

  // Восстанавливаем CSS-классы из карты перед вставкой в шаблон
  let content = editedContent;
  if (classMap && Object.keys(classMap).length > 0) {
    content = applyClassMap(content, classMap);
  }

  // Вставляем контент в шаблон
  let html = template.replace(CONTENT_PLACEHOLDER, content);

  // Применяем SEO через DOM
  html = applySeoToHtml(html, seo);

  // Применяем мету (favicon, logo, nav)
  if (meta) {
    html = applyPageMeta(html, meta);
  }

  return html;
}

/**
 * Применяет SEO-данные к полному HTML через DOM-манипуляции.
 */
function applySeoToHtml(html: string, seo: SEOData): string {
  if (!html.trim()) return html;

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  // <title>
  if (seo.title) {
    let el = doc.querySelector("title");
    if (!el) {
      el = doc.createElement("title");
      doc.head.appendChild(el);
    }
    el.textContent = seo.title;
  }

  // <meta name="description">
  if (seo.description) {
    let el = doc.querySelector('meta[name="description"]');
    if (!el) {
      el = doc.createElement("meta");
      el.setAttribute("name", "description");
      doc.head.appendChild(el);
    }
    el.setAttribute("content", seo.description);
  }

  // <meta property="og:title">
  if (seo.ogTitle) {
    let el = doc.querySelector('meta[property="og:title"]');
    if (!el) {
      el = doc.createElement("meta");
      el.setAttribute("property", "og:title");
      doc.head.appendChild(el);
    }
    el.setAttribute("content", seo.ogTitle);
  }

  // <meta property="og:description">
  if (seo.ogDescription) {
    let el = doc.querySelector('meta[property="og:description"]');
    if (!el) {
      el = doc.createElement("meta");
      el.setAttribute("property", "og:description");
      doc.head.appendChild(el);
    }
    el.setAttribute("content", seo.ogDescription);
  }

  return `<!DOCTYPE html>\n${doc.documentElement.outerHTML}`;
}
