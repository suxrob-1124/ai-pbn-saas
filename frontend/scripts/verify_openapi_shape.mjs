#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const openApiPath = path.resolve(__dirname, "..", "openapi.yaml");

function fail(message) {
  console.error(`OPENAPI_LINT: ${message}`);
  process.exit(1);
}

function readFileOrFail(filePath) {
  try {
    return fs.readFileSync(filePath, "utf8");
  } catch (error) {
    fail(`cannot read file: ${filePath}\n${String(error)}`);
  }
}

function parsePaths(content) {
  const lines = content.split(/\r?\n/);
  const paths = new Map();
  let inPaths = false;
  let currentPath = null;

  for (const line of lines) {
    if (!inPaths && /^paths:\s*$/.test(line)) {
      inPaths = true;
      currentPath = null;
      continue;
    }
    if (!inPaths) {
      continue;
    }
    if (/^components:\s*$/.test(line)) {
      break;
    }
    // stop at next top-level key
    if (/^[A-Za-z0-9_-]+:\s*$/.test(line)) {
      break;
    }
    const pathMatch = line.match(/^  (\/[^:#\s][^:]*):\s*$/);
    if (pathMatch) {
      currentPath = pathMatch[1];
      if (paths.has(currentPath)) {
        fail(`duplicate path key: ${currentPath}`);
      }
      paths.set(currentPath, []);
      continue;
    }
    const operationMatch = line.match(/^    (get|post|put|patch|delete|options|head):\s*$/);
    if (operationMatch && currentPath) {
      paths.get(currentPath).push(operationMatch[1]);
    }
  }

  return paths;
}

const content = readFileOrFail(openApiPath);

if (!/^openapi:\s*3\.\d+\.\d+\s*$/m.test(content)) {
  fail("missing or invalid `openapi: 3.x.x` header");
}
if (!/^info:\s*$/m.test(content)) {
  fail("missing `info` section");
}
if (!/^paths:\s*$/m.test(content)) {
  fail("missing `paths` section");
}
if (!/^components:\s*$/m.test(content)) {
  fail("missing `components` section");
}

const paths = parsePaths(content);
if (paths.size === 0) {
  fail("no path entries found in `paths` section");
}

const invalidPrefix = [];
const withoutOperations = [];

for (const [apiPath, methods] of paths) {
  if (!apiPath.startsWith("/api/") && apiPath !== "/healthz" && apiPath !== "/metrics") {
    invalidPrefix.push(apiPath);
  }
  if (!methods.length) {
    withoutOperations.push(apiPath);
  }
  if (/\s/.test(apiPath)) {
    fail(`path contains whitespace: ${apiPath}`);
  }
  if (apiPath.includes("//")) {
    fail(`path contains double slash: ${apiPath}`);
  }
}

if (invalidPrefix.length > 0) {
  fail(`paths with unexpected prefix: ${invalidPrefix.join(", ")}`);
}
if (withoutOperations.length > 0) {
  fail(`paths without HTTP operations: ${withoutOperations.join(", ")}`);
}

console.log(`OPENAPI_LINT: ok (${paths.size} paths validated)`);
