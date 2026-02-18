import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("Проверьте план применения"), "missing pre-apply summary confirmation");
assert.ok(page.includes("Подтвердите перезапись"), "missing second overwrite confirmation");
assert.ok(page.includes("[OVERWRITE]"), "missing overwrite details in summary");
assert.ok(page.includes("Часть create-применений пропущена"), "missing explicit warning for skipped create");

console.log("OK");

