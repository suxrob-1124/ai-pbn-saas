import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "(app)", "domains", "[id]", "page.tsx");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("Открыть в редакторе"), "domain page must include editor button label");
assert.ok(page.includes("canOpenEditor"), "domain page must define editor availability condition");
assert.ok(page.includes("/domains/${id}/editor"), "domain page must link to editor route");
assert.ok(page.includes("Редактор доступен после публикации и синхронизации файлов"), "domain page must include disabled hint for editor");

console.log("OK");

