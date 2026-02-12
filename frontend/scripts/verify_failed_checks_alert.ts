import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");
const filePath = path.join(root, "frontend/components/FailedChecksAlert.tsx");
assert.ok(fs.existsSync(filePath), "FailedChecksAlert component must exist");

const content = fs.readFileSync(filePath, "utf8");
assert.ok(content.includes("View Details"), "FailedChecksAlert must include View Details button");
assert.ok(content.includes("failed_investigation"), "FailedChecksAlert must mention failed_investigation");

console.log("OK");
