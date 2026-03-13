import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "(app)", "domains", "[id]", "editor", "page.tsx");
const fileTreePath = path.join(root, "components", "FileTree.tsx");
const monacoPath = path.join(root, "components", "MonacoEditor.tsx");
const toolbarPath = path.join(root, "components", "EditorToolbar.tsx");
const historyPath = path.join(root, "components", "FileHistory.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
assert.ok(existsSync(fileTreePath), "missing FileTree component");
assert.ok(existsSync(monacoPath), "missing MonacoEditor component");
assert.ok(existsSync(toolbarPath), "missing EditorToolbar component");
assert.ok(existsSync(historyPath), "missing FileHistory component");

const page = readFileSync(pagePath, "utf8");
assert.ok(page.includes("/api/domains/"), "editor page should call domain summary API");
assert.ok(page.includes("listFiles"), "editor page should load files list");
assert.ok(page.includes("getFile"), "editor page should load file content");
assert.ok(page.includes("saveFile"), "editor page should save file");
assert.ok(page.includes("<FileTree"), "editor page should render FileTree");
assert.ok(page.includes("<MonacoEditor"), "editor page should render MonacoEditor");
assert.ok(page.includes("<EditorToolbar"), "editor page should render EditorToolbar");
assert.ok(page.includes("<FileHistory"), "editor page should render FileHistory");

console.log("OK");
