import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "domains", "[id]", "page.tsx");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("Результат"), "domain page must render result block title");
assert.ok(page.includes("Просмотр HTML"), "domain page must include quick action for final_html");
assert.ok(page.includes("Скачать ZIP"), "domain page must include quick action for zip");
assert.ok(page.includes("К артефактам"), "domain page must include anchor action to artifacts");
assert.ok(page.includes("showResultBlock"), "domain page must define result block visibility logic");
assert.ok(page.includes("go run ./cmd/import_legacy --mode apply --source auto"), "domain page must include empty-state import hint");

console.log("OK");
