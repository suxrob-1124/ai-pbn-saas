import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("AI Studio: создать страницу"), "missing AI Studio create tab");
assert.ok(page.includes("Сгенерировать пакет файлов"), "missing generate files action");
assert.ok(page.includes("Применить план"), "missing apply-plan action");
assert.ok(page.includes("create"), "missing per-file create action");
assert.ok(page.includes("overwrite"), "missing per-file overwrite action");
assert.ok(page.includes("skip"), "missing per-file skip action");
assert.ok(page.includes("Контекст запроса"), "missing context debug action");
assert.ok(page.includes("Диагностика"), "missing diagnostics block");

console.log("OK");
