import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");
const filePath = path.join(root, "frontend/components/IndexStats.tsx");
assert.ok(fs.existsSync(filePath), "IndexStats component must exist");

const content = fs.readFileSync(filePath, "utf8");
assert.ok(content.includes("recharts"), "IndexStats must use recharts");
assert.ok(content.includes("LineChart"), "IndexStats must render LineChart");
assert.ok(content.includes("BarChart"), "IndexStats must render BarChart");
assert.ok(content.includes("7d"), "IndexStats must include 7d toggle");
assert.ok(content.includes("30d"), "IndexStats must include 30d toggle");
assert.ok(content.includes("90d"), "IndexStats must include 90d toggle");
assert.ok(content.includes("Процент индексации"), "IndexStats must include indexing percent metric");
assert.ok(content.includes("Среднее"), "IndexStats must include average attempts metric");
assert.ok(content.includes("failed_investigation"), "IndexStats must include failed metric label");

console.log("OK");
