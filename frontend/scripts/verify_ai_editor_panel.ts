import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "(app)", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

const hasAny = (candidates: string[]) => candidates.some((value) => page.includes(value));

assert.ok(hasAny(["AI Studio: изменить текущий файл", "t.tabs.editCurrentFile"]), "missing AI Studio edit tab");
assert.ok(page.includes("Режим контекста"), "missing context mode selector");
assert.ok(hasAny(["Сгенерировать предложение", "t.actions.generateSuggestion"]), "missing suggest action");
assert.ok(hasAny(["Применить в редактор", "t.actions.applyToEditor"]), "missing apply-to-buffer action");
assert.ok(hasAny(["Контекст запроса", "t.actions.requestContext"]), "missing context debug action");
assert.ok(hasAny(["Сравнение", "t.actions.compare"]), "missing diff preview action");
assert.ok(page.includes("Диагностика"), "missing diagnostics collapsible");

console.log("OK");
