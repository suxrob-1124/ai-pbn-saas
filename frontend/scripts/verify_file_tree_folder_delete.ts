import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const treePath = path.join(root, "components", "FileTree.tsx");
const editorPath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(treePath), "missing FileTree.tsx");
assert.ok(existsSync(editorPath), "missing editor page");

const tree = readFileSync(treePath, "utf8");
const page = readFileSync(editorPath, "utf8");

assert.ok(tree.includes("onDeleteFolder"), "FileTree should support folder delete callback");
assert.ok(tree.includes("FiTrash2"), "FileTree should render delete icon for folders");
assert.ok(page.includes("onDeleteFolder"), "editor should wire folder delete handler");
assert.ok(page.includes("Папка удалена"), "editor should show success feedback on folder delete");

console.log("OK");

