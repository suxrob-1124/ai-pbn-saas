/**
 * Парсинг и применение мета-данных страницы: favicon, логотип, навигация.
 *
 * Навигация разделена по секциям (header, footer, aside) —
 * каждая извлекается и применяется независимо.
 */

export type NavLink = {
  label: string;
  href: string;
};

export type NavSection = "header" | "footer" | "aside";

export type PageMeta = {
  favicon: string;
  logo: string;
  headerNav: NavLink[];
  footerNav: NavLink[];
  asideNav: NavLink[];
};

export const defaultPageMeta: PageMeta = {
  favicon: "",
  logo: "",
  headerNav: [],
  footerNav: [],
  asideNav: [],
};

/** Извлекает ссылки из <nav> элемента (или из набора <a> внутри контейнера). */
function extractLinksFromNav(nav: Element | null): NavLink[] {
  if (!nav) return [];
  const links: NavLink[] = [];
  const anchors = nav.querySelectorAll("a");
  for (const a of Array.from(anchors)) {
    const label = (a.textContent || "").trim();
    const href = a.getAttribute("href") || "";
    if (label && href) {
      links.push({ label, href });
    }
  }
  return links;
}

/**
 * Применяет массив NavLink к конкретному <nav> элементу в DOM.
 */
function applyLinksToNav(nav: Element, links: NavLink[], doc: Document) {
  const anchors = nav.querySelectorAll("a");
  const existing = Array.from(anchors);

  // Обновляем существующие
  for (let i = 0; i < links.length && i < existing.length; i++) {
    existing[i].textContent = links[i].label;
    existing[i].setAttribute("href", links[i].href);
  }

  // Добавляем новые
  if (links.length > existing.length) {
    const container = existing.length > 0 ? existing[0].parentElement : nav;
    if (container) {
      for (let i = existing.length; i < links.length; i++) {
        const a = doc.createElement("a");
        a.setAttribute("href", links[i].href);
        a.textContent = links[i].label;
        container.appendChild(a);
      }
    }
  }

  // Удаляем лишние
  if (links.length < existing.length) {
    for (let i = links.length; i < existing.length; i++) {
      existing[i].remove();
    }
  }
}

/**
 * Извлекает мета-данные (favicon, logo, nav links) из полного HTML документа.
 *
 * Генератор создаёт HTML со структурой:
 * - `<nav>` внутри `<header>` — основная навигация (headerNav)
 * - `<nav>` внутри `<footer>` — футер-навигация (footerNav)
 * - `<nav>` внутри `<aside>` — боковая навигация (asideNav)
 * - Логотип = `<a class="pfx-logo-link">` с inline SVG (не <img>)
 */
export function extractPageMeta(html: string): PageMeta {
  if (!html.trim()) return { ...defaultPageMeta };

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  // Favicon
  const faviconEl =
    doc.querySelector('link[rel="icon"]') ||
    doc.querySelector('link[rel="shortcut icon"]') ||
    doc.querySelector('link[rel="apple-touch-icon"]');
  const favicon = faviconEl?.getAttribute("href") || "";

  // Logo — ищем в header: сначала <a> с классом "logo", потом первый <img>
  let logo = "";
  const header = doc.querySelector("header");
  if (header) {
    const logoLink = header.querySelector('a[class*="logo"]');
    const logoImg =
      logoLink?.querySelector("img") || header.querySelector("img");
    if (logoImg) {
      logo = logoImg.getAttribute("src") || "";
    }
    if (!logo && logoLink) {
      const svgEl = logoLink.querySelector("svg");
      if (svgEl) {
        logo = "";
      }
    }
  }

  // Header nav — <nav> внутри <header>, fallback на body-level <nav>
  const headerNavEl = header?.querySelector("nav") || doc.querySelector("body > nav");
  let headerNav = extractLinksFromNav(headerNavEl);

  // Fallback: если в nav ничего нет — ищем header-ссылки (без logo)
  if (headerNav.length === 0 && header) {
    const anchors = header.querySelectorAll("a");
    for (const a of Array.from(anchors)) {
      if (a.classList.toString().includes("logo")) continue;
      const label = (a.textContent || "").trim();
      const href = a.getAttribute("href") || "";
      if (label && href) {
        headerNav.push({ label, href });
      }
    }
  }

  // Footer nav — <nav> внутри <footer>
  const footerNavEl = doc.querySelector("footer nav");
  const footerNav = extractLinksFromNav(footerNavEl);

  // Aside nav — <nav> внутри <aside>
  const asideNavEl = doc.querySelector("aside nav");
  const asideNav = extractLinksFromNav(asideNavEl);

  return { favicon, logo, headerNav, footerNav, asideNav };
}

/**
 * Применяет изменённые мета-данные обратно в полный HTML.
 * Обновляет favicon href, logo src, nav links по секциям.
 */
export function applyPageMeta(html: string, meta: PageMeta): string {
  if (!html.trim()) return html;

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  // Обновляем favicon
  if (meta.favicon) {
    let faviconEl =
      doc.querySelector('link[rel="icon"]') ||
      doc.querySelector('link[rel="shortcut icon"]');
    if (faviconEl) {
      faviconEl.setAttribute("href", meta.favicon);
    } else {
      faviconEl = doc.createElement("link");
      faviconEl.setAttribute("rel", "icon");
      faviconEl.setAttribute("href", meta.favicon);
      doc.head.appendChild(faviconEl);
    }
  }

  // Обновляем логотип
  if (meta.logo) {
    const header = doc.querySelector("header");
    const logoLink = header?.querySelector('a[class*="logo"]');
    const logoImg =
      logoLink?.querySelector("img") || header?.querySelector("img");
    if (logoImg) {
      logoImg.setAttribute("src", meta.logo);
    }
  }

  // Обновляем header nav
  const header = doc.querySelector("header");
  const headerNavEl = header?.querySelector("nav") || doc.querySelector("body > nav");
  if (headerNavEl && meta.headerNav.length > 0) {
    applyLinksToNav(headerNavEl, meta.headerNav, doc);
  }

  // Обновляем footer nav
  const footerNavEl = doc.querySelector("footer nav");
  if (footerNavEl && meta.footerNav.length > 0) {
    applyLinksToNav(footerNavEl, meta.footerNav, doc);
  }

  // Обновляем aside nav
  const asideNavEl = doc.querySelector("aside nav");
  if (asideNavEl && meta.asideNav.length > 0) {
    applyLinksToNav(asideNavEl, meta.asideNav, doc);
  }

  return `<!DOCTYPE html>\n${doc.documentElement.outerHTML}`;
}
