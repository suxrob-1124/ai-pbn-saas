import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "ArtifactsViewer.tsx");
const source = readFileSync(filePath, "utf8");

assert.ok(source.includes("function FinalHTMLViewer"), "ArtifactsViewer must provide FinalHTMLViewer component");
assert.ok(source.includes("Preview"), "FinalHTMLViewer must provide Preview tab");
assert.ok(source.includes("Code"), "FinalHTMLViewer must provide Code tab");
assert.ok(source.includes("srcDoc={html}"), "FinalHTMLViewer preview must use iframe srcDoc");
assert.ok(source.includes('sandbox="allow-same-origin"'), "FinalHTMLViewer iframe must be sandboxed");

console.log("OK");

