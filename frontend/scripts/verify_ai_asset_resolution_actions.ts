import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const pagePath = path.join(root, "app", "domains", "[id]", "editor", "page.tsx");

assert.ok(existsSync(pagePath), "missing /domains/[id]/editor page");
const page = readFileSync(pagePath, "utf8");

assert.ok(page.includes("Нерешённые ассеты"), "missing unresolved assets warning block");
assert.ok(page.includes("Ссылки на файлы без манифеста ассетов"), "missing non-manifest assets block");
assert.ok(page.includes("const onAssetUploadPick"), "missing asset upload action handler");
assert.ok(page.includes("const onToggleSkipAsset"), "missing skip/unskip action handler");
assert.ok(page.includes("const onCopyAssetPrompt"), "missing copy prompt action handler");
assert.ok(page.includes("const onRegenerateAsset"), "missing regenerate asset action handler");
assert.ok(page.includes("Регенерировать"), "missing regenerate action button in UI");
assert.ok(page.includes("accept=\"image/png,image/jpeg,image/webp,image/gif,image/svg+xml\""), "missing image-only upload input guard");

console.log("OK");
