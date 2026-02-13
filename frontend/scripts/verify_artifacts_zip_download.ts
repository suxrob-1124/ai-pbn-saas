import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "ArtifactsViewer.tsx");
const source = readFileSync(filePath, "utf8");

assert.ok(source.includes('entry.key === "zip_archive"'), "ArtifactsViewer must branch zip download by key");
assert.ok(source.includes("downloadZipBase64"), "ArtifactsViewer must include zip decoder download helper");
assert.ok(source.includes("application/zip"), "zip download must use application/zip mime type");
assert.ok(source.includes('site.zip'), "zip download must fallback to site.zip");

console.log("OK");

