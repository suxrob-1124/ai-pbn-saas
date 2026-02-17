import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("AI Studio: изменить текущий файл"), "missing AI Studio edit tab");
assert.ok(page.includes("Режим контекста"), "missing context mode selector");
assert.ok(page.includes("Сгенерировать предложение"), "missing suggest action");
assert.ok(page.includes("Применить в буфер"), "missing apply-to-buffer action");
assert.ok(page.includes("Контекст запроса"), "missing context debug action");
assert.ok(page.includes("Diff"), "missing diff preview action");
assert.ok(page.includes("Диагностика"), "missing diagnostics collapsible");

console.log("OK");
