import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("AI: создать новую страницу"), "missing AI create-page panel");
assert.ok(page.includes("Generate files"), "missing generate files action");
assert.ok(page.includes("Apply all"), "missing apply-all action");
assert.ok(page.includes("Применить"), "missing confirmation flow for apply-all");
assert.ok(page.includes("Select all"), "missing select-all action");
assert.ok(page.includes("Clear"), "missing clear action");
assert.ok(page.includes("Preview:"), "missing preview section for generated files");
assert.ok(page.includes("Выбрано:"), "missing selected-files counter");

console.log("OK");
