import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");
const apiPath = path.join(root, "lib", "fileApi.ts");

assert.ok(existsSync(pagePath), "missing editor page");
assert.ok(existsSync(apiPath), "missing fileApi");

const page = readFileSync(pagePath, "utf8");
const api = readFileSync(apiPath, "utf8");

assert.ok(api.includes("AIPageSuggestionAsset"), "missing AIPageSuggestionAsset DTO");
assert.ok(page.includes("Манифест ассетов"), "missing assets manifest block");
assert.ok(page.includes("не применяются автоматически как бинарные файлы"), "missing assets apply warning");

console.log("OK");

