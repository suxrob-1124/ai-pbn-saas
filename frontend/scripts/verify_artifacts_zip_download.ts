import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "ArtifactsViewer.tsx");
const source = readFileSync(filePath, "utf8");

assert.ok(source.includes('"zip_archive"'), "ArtifactsViewer must include zip artifact in final group");
assert.ok(!source.includes("downloadZipBase64"), "ArtifactsViewer should not expose zip download helper in v5 summary mode");
assert.ok(!source.includes("Копировать"), "ArtifactsViewer should not render copy actions");
assert.ok(!source.includes("Скачать"), "ArtifactsViewer should not render download actions");

console.log("OK");
