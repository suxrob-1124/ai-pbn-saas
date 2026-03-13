import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "(app)", "domains", "[id]", "editor", "page.tsx");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("my_role"), "editor page must use my_role from summary");
assert.ok(page.includes("=== \"viewer\""), "editor page must have viewer read-only check");
assert.ok(page.includes("Недостаточно прав для сохранения") || page.includes("Недостаточно прав"), "editor page must show permission error for save");
assert.ok(page.includes("canSave"), "editor page must gate save action");
assert.ok(page.includes("viewer"), "editor page must include viewer role text");

console.log("OK");
