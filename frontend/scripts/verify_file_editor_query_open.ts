import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "(app)", "domains", "[id]", "editor", "page.tsx");
const page = readFileSync(pagePath, "utf8");
const actionsHookPath = path.join(process.cwd(), "features", "editor-v3", "hooks", "useEditorPageActions.ts");
const actionsHook = readFileSync(actionsHookPath, "utf8");

assert.ok(page.includes('searchParams.get("path")'), "editor page must read path query param");
assert.ok(page.includes('searchParams.get("line")'), "editor page must read line query param");
assert.ok(
  page.includes("query.set(\"path\"") || actionsHook.includes("query.set(\"path\""),
  "editor page must update path query param"
);
assert.ok(page.includes("scrollLine") || page.includes("focusLine"), "editor page must pass line to Monaco editor");

console.log("OK");
