import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const historyPath = path.join(root, "components", "FileHistory.tsx");
const diffPath = path.join(root, "components", "MonacoDiffEditor.tsx");
const modalPath = path.join(root, "components", "ConflictResolutionModal.tsx");
const pagePath = path.join(root, "app", "(app)", "domains", "[id]", "editor", "page.tsx");
const apiPath = path.join(root, "lib", "fileApi.ts");

assert.ok(existsSync(historyPath), "missing FileHistory component");
assert.ok(existsSync(diffPath), "missing MonacoDiffEditor component");
assert.ok(existsSync(modalPath), "missing ConflictResolutionModal component");

const history = readFileSync(historyPath, "utf8");
assert.ok(history.includes("View diff"), "history should expose View diff action");
assert.ok(history.includes("Revert to this version"), "history should expose revert action");
assert.ok(history.includes("MonacoDiffEditor"), "history should render Monaco diff viewer");

const page = readFileSync(pagePath, "utf8");
assert.ok(page.includes("ConflictResolutionModal"), "editor page should render conflict modal");

const api = readFileSync(apiPath, "utf8");
assert.ok(api.includes("class SaveConflictError"), "file API should expose SaveConflictError");

console.log("OK");

