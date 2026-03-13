import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "(app)", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

const hasAny = (candidates: string[]) => candidates.some((value) => page.includes(value));

assert.ok(hasAny(["AI Studio: создать страницу", "t.tabs.createPage"]), "missing AI Studio create tab");
assert.ok(hasAny(["Сгенерировать файлы", "t.actions.generateFiles"]), "missing generate files action");
assert.ok(hasAny(["Применить выбранное", "t.actions.applySelected"]), "missing apply-plan action");
assert.ok(hasAny(["создать", "t.applyPlan.create"]), "missing per-file create action");
assert.ok(hasAny(["перезаписать", "t.applyPlan.overwrite"]), "missing per-file overwrite action");
assert.ok(hasAny(["пропустить", "t.applyPlan.skip"]), "missing per-file skip action");
assert.ok(hasAny(["Контекст запроса", "t.actions.requestContext"]), "missing context debug action");
assert.ok(page.includes("Диагностика"), "missing diagnostics block");

console.log("OK");
