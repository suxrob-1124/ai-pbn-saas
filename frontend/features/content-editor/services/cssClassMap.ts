/**
 * Карта CSS-классов: tag → className.
 *
 * Сгенерированные сайты используют CSS-классы с префиксом (x9a3b-heading, x9a3b-tbl и т.д.).
 * TipTap и sanitizeNode() при редактировании эти классы теряют.
 *
 * Стратегия:
 * 1. extractClassMap() — до sanitize сканирует оригинальный контент, строит карту tag → className
 * 2. applyClassMap() — при сборке финального HTML навешивает классы обратно на голые теги
 */

export type ClassMap = Record<string, string>;

/** Теги, для которых отслеживаем CSS-классы */
const TRACKED_TAGS = new Set([
  "h1", "h2", "h3", "h4",
  "p",
  "table", "thead", "tbody", "tfoot", "tr", "th", "td",
  "ul", "ol", "li",
  "blockquote",
  "img",
  "a",
  "div", "section", "figure", "figcaption",
]);

/**
 * Извлекает карту CSS-классов из DOM-элемента контентной зоны.
 * Для каждого отслеживаемого тега выбирает самую частую комбинацию className.
 */
export function extractClassMap(contentElement: Element): ClassMap {
  const freq: Record<string, Map<string, number>> = {};

  const walk = (el: Element) => {
    const tag = el.tagName.toLowerCase();
    if (TRACKED_TAGS.has(tag)) {
      const cls = el.getAttribute("class")?.trim();
      if (cls) {
        if (!freq[tag]) freq[tag] = new Map();
        freq[tag].set(cls, (freq[tag].get(cls) || 0) + 1);
      }
    }
    for (const child of Array.from(el.children)) {
      walk(child);
    }
  };

  walk(contentElement);

  const result: ClassMap = {};
  for (const [tag, counts] of Object.entries(freq)) {
    let best = "";
    let bestCount = 0;
    for (const [cls, count] of counts) {
      if (count > bestCount) {
        best = cls;
        bestCount = count;
      }
    }
    if (best) result[tag] = best;
  }

  return result;
}

/**
 * Применяет карту классов к HTML-строке.
 * Для каждого элемента: если тег есть в карте и у элемента нет class — добавляет class.
 * Элементы с существующим class не трогает.
 */
export function applyClassMap(html: string, classMap: ClassMap): string {
  if (!html.trim()) return html;

  const doc = new DOMParser().parseFromString(`<body>${html}</body>`, "text/html");
  const body = doc.body;
  if (!body) return html;

  const walk = (el: Element) => {
    const tag = el.tagName.toLowerCase();
    if (tag in classMap && !el.getAttribute("class")) {
      el.setAttribute("class", classMap[tag]);
    }
    for (const child of Array.from(el.children)) {
      walk(child);
    }
  };

  walk(body);
  return body.innerHTML;
}
