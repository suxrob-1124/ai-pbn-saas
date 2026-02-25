#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..", "..");
const docsRoot = path.resolve(repoRoot, "frontend", "app", "docs");

function fail(message) {
  console.error(`DOCS_LINK_CHECK: ${message}`);
  process.exit(1);
}

function readText(filePath) {
  try {
    return fs.readFileSync(filePath, "utf8");
  } catch (error) {
    fail(`cannot read file: ${filePath}\n${String(error)}`);
  }
}

function exists(targetPath) {
  try {
    fs.accessSync(targetPath, fs.constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

function normalizeRoute(route) {
  if (!route) return "";
  if (route.endsWith("/") && route !== "/docs/") {
    return route.slice(0, -1);
  }
  if (route === "/docs/") {
    return "/docs";
  }
  return route;
}

function listDocsRoutes(rootDir) {
  const routes = new Set();

  function walk(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(full);
      } else if (entry.isFile() && entry.name === "page.tsx") {
        const relDir = path.relative(rootDir, path.dirname(full));
        const route = relDir === "" ? "/docs" : `/docs/${relDir.split(path.sep).join("/")}`;
        routes.add(normalizeRoute(route));
      }
    }
  }

  walk(rootDir);
  return routes;
}

function isExternalLink(link) {
  return (
    link.startsWith("http://") ||
    link.startsWith("https://") ||
    link.startsWith("mailto:") ||
    link.startsWith("tel:") ||
    link.startsWith("#") ||
    link.startsWith("javascript:")
  );
}

function stripQueryAndHash(link) {
  return link.split("#")[0].split("?")[0];
}

function verifyMarkdownLinks(filePath, routes, problems) {
  const text = readText(filePath);
  const markdownLinkRe = /\[[^\]]*]\(([^)]+)\)/g;
  let match = markdownLinkRe.exec(text);

  while (match) {
    const raw = match[1].trim().replace(/^<|>$/g, "");
    if (!raw || isExternalLink(raw)) {
      match = markdownLinkRe.exec(text);
      continue;
    }
    const clean = stripQueryAndHash(raw);
    if (!clean) {
      match = markdownLinkRe.exec(text);
      continue;
    }
    if (clean.startsWith("/docs")) {
      const route = normalizeRoute(clean);
      if (!routes.has(route)) {
        problems.push(`${path.relative(repoRoot, filePath)} -> missing docs route ${clean}`);
      }
    } else if (!clean.startsWith("/")) {
      const resolved = path.resolve(path.dirname(filePath), clean);
      if (!exists(resolved)) {
        problems.push(`${path.relative(repoRoot, filePath)} -> broken relative link ${clean}`);
      }
    }
    match = markdownLinkRe.exec(text);
  }
}

function verifyDocsRouteStrings(filePath, routes, problems) {
  const text = readText(filePath);
  const docsRouteRe = /["'`](\/docs(?:\/[a-zA-Z0-9-]+)*)["'`]/g;
  let match = docsRouteRe.exec(text);
  while (match) {
    const route = normalizeRoute(match[1]);
    if (!routes.has(route)) {
      problems.push(`${path.relative(repoRoot, filePath)} -> missing docs route ${match[1]}`);
    }
    match = docsRouteRe.exec(text);
  }
}

const docsRoutes = listDocsRoutes(docsRoot);
if (docsRoutes.size === 0) {
  fail("no docs routes found under frontend/app/docs");
}

const filesToCheck = [
  path.resolve(repoRoot, "README.md"),
  path.resolve(repoRoot, "DOCS_D0_GAP_REPORT.md"),
  path.resolve(repoRoot, "frontend", "docs-content", "registry.ts"),
  path.resolve(repoRoot, "frontend", "components", "DocsSidebar.tsx"),
  path.resolve(repoRoot, "frontend", "app", "docs", "page.tsx"),
];

const problems = [];
for (const filePath of filesToCheck) {
  if (!exists(filePath)) {
    problems.push(`missing file to check: ${path.relative(repoRoot, filePath)}`);
    continue;
  }
  verifyMarkdownLinks(filePath, docsRoutes, problems);
  verifyDocsRouteStrings(filePath, docsRoutes, problems);
}

if (problems.length > 0) {
  console.error("DOCS_LINK_CHECK: found issues:");
  for (const problem of problems) {
    console.error(`  - ${problem}`);
  }
  process.exit(1);
}

console.log(
  `DOCS_LINK_CHECK: ok (${filesToCheck.length} files checked, ${docsRoutes.size} docs routes indexed)`
);
