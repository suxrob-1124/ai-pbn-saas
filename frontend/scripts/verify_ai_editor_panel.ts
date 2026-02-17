import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("AI: редактирование файла"), "missing AI file edit panel");
assert.ok(page.includes("Контекст-файлы через запятую"), "missing context-files input");
assert.ok(page.includes("Версия модели"), "missing model select for AI suggest");
assert.ok(page.includes("Prompt source:"), "missing prompt source output");
assert.ok(page.includes("Token usage:"), "missing token usage output");
assert.ok(page.includes("View diff"), "missing diff preview action");
assert.ok(page.includes("Suggest"), "missing suggest action");
assert.ok(page.includes("Apply to buffer"), "missing apply action");

console.log("OK");
