import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const editorPath = path.join(root, "app", "(app)", "domains", "[id]", "editor", "page.tsx");
const panelPath = path.join(
  root,
  "features",
  "editor-v3",
  "components",
  "EditorAIStudioPanel.tsx"
);

assert.ok(existsSync(editorPath), "missing /domains/[id]/editor page");
assert.ok(existsSync(panelPath), "missing EditorAIStudioPanel");
const page = readFileSync(editorPath, "utf8");
const panel = readFileSync(panelPath, "utf8");

assert.ok(page.includes("aiOutputSourcePath"), "missing AI source-path state");
assert.ok(page.includes("aiApplyPathMismatch"), "missing AI source-path guard");
assert.ok(panel.includes("Предложение относится к файлу"), "missing mismatch warning text");
assert.ok(panel.includes("Открыть исходный файл"), "missing action to reopen source file");

console.log("OK");
