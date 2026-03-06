/**
 * Парсинг и применение CSS custom properties из :root блока.
 *
 * Сгенерированные сайты содержат :root { --pfx-primary: #...; ... }
 * в style.css (отдельный файл) или в inline <style> тегах внутри HTML.
 * Этот сервис поддерживает оба варианта.
 */

export type CSSVariableCategory = "color" | "size" | "font" | "other";

export type CSSVariable = {
  name: string; // "--pfx-primary"
  value: string; // "#3b82f6"
  category: CSSVariableCategory;
};

const COLOR_VALUE_RE = /^#|^rgb|^hsl|^oklch|^lch|^lab|^hwb/i;
const COLOR_NAME_RE = /color|bg|text|border-color|shadow|accent|primary|secondary|muted/i;
const SIZE_VALUE_RE = /px|rem|em|%|vw|vh|dvh|svh/;
const SIZE_NAME_RE = /radius|gap|spacing|size|width|height|padding|margin/i;
const FONT_NAME_RE = /font/i;

function categorize(name: string, value: string): CSSVariableCategory {
  const trimmed = value.trim();
  if (COLOR_VALUE_RE.test(trimmed) || COLOR_NAME_RE.test(name)) return "color";
  if (SIZE_VALUE_RE.test(trimmed) || SIZE_NAME_RE.test(name)) return "size";
  if (FONT_NAME_RE.test(name)) return "font";
  return "other";
}

/**
 * Извлекает CSS custom properties из :root блока.
 */
export function extractCssVariables(css: string): CSSVariable[] {
  if (!css.trim()) return [];

  // Находим :root { ... } блок (может быть несколько — берём первый)
  const rootMatch = css.match(/:root\s*\{([^}]+)\}/);
  if (!rootMatch) return [];

  const body = rootMatch[1];
  const variables: CSSVariable[] = [];
  const varRe = /(--[\w-]+)\s*:\s*([^;]+);/g;
  let m: RegExpExecArray | null;

  while ((m = varRe.exec(body)) !== null) {
    const name = m[1].trim();
    const value = m[2].trim();
    variables.push({ name, value, category: categorize(name, value) });
  }

  return variables;
}

/**
 * Применяет изменённые переменные обратно в CSS строку.
 * Находит :root { ... } блок и заменяет значения переменных.
 */
export function applyCssVariables(
  css: string,
  variables: CSSVariable[],
): string {
  if (!css.trim() || variables.length === 0) return css;

  // Строим map name → new value
  const updates = new Map<string, string>();
  for (const v of variables) {
    updates.set(v.name, v.value);
  }

  // Заменяем значения внутри :root блока
  return css.replace(
    /(:root\s*\{)([^}]+)(\})/,
    (_full, open: string, body: string, close: string) => {
      const updatedBody = body.replace(
        /(--[\w-]+)\s*:\s*([^;]+)(;)/g,
        (_m, name: string, _oldVal: string, semi: string) => {
          const trimName = name.trim();
          if (updates.has(trimName)) {
            return `${name}: ${updates.get(trimName)}${semi}`;
          }
          return _m;
        },
      );
      return `${open}${updatedBody}${close}`;
    },
  );
}

/**
 * Извлекает CSS из inline <style> тегов HTML-документа.
 * Используется как fallback когда нет отдельного style.css.
 */
export function extractCssFromHtml(html: string): string {
  if (!html.trim()) return "";
  const parts: string[] = [];
  const re = /<style[^>]*>([\s\S]*?)<\/style>/gi;
  let m: RegExpExecArray | null;
  while ((m = re.exec(html)) !== null) {
    parts.push(m[1]);
  }
  return parts.join("\n");
}

/**
 * Применяет изменённые CSS-переменные к inline <style> тегам в HTML.
 * Находит <style> содержащий :root { ... } и обновляет значения переменных.
 */
export function applyCssVariablesToHtml(
  html: string,
  variables: CSSVariable[],
): string {
  if (!html.trim() || variables.length === 0) return html;
  return html.replace(
    /(<style[^>]*>)([\s\S]*?)(<\/style>)/gi,
    (_full, open: string, body: string, close: string) => {
      if (/:root\s*\{/.test(body)) {
        return open + applyCssVariables(body, variables) + close;
      }
      return _full;
    },
  );
}
