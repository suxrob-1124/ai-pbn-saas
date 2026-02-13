import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "ArtifactsViewer.tsx");
const source = readFileSync(filePath, "utf8");

assert.ok(source.includes('group.id === "final"'), "ArtifactsViewer must default-open final group");
assert.ok(source.includes("FinalHTMLViewer"), "ArtifactsViewer must use dedicated final_html renderer");
assert.ok(source.includes("Legacy decoded"), "ArtifactsViewer must show legacy decoded badge");

console.log("OK");

