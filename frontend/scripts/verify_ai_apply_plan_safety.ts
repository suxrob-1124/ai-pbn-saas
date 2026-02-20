import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");
const hasAny = (candidates: string[]) => candidates.some((value) => page.includes(value));

assert.ok(hasAny(["Проверьте план применения", "t.applySafety.summaryTitle"]), "missing pre-apply summary confirmation");
assert.ok(hasAny(["Подтверждаю перезапись существующих файлов", "t.applySafety.overwriteConfirmLabel"]), "missing explicit overwrite confirmation guard");
assert.ok(hasAny(["перезапись изменит существующие файлы", "ПЕРЕЗАПИСАТЬ"]), "missing overwrite risk details in summary");
assert.ok(hasAny(["Проблемы ассетов перед применением", "applyBlockedByAssetIssues"]), "missing asset validation safety block");
assert.ok(page.includes("Часть create-применений пропущена"), "missing explicit warning for skipped create");

console.log("OK");
