#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const openApiPath = path.resolve(__dirname, "..", "openapi.yaml");
const serverPath = path.resolve(__dirname, "..", "..", "pbn-generator", "internal", "httpserver", "server.go");

function fail(message) {
  console.error(`OPENAPI_ROUTE_COVERAGE: ${message}`);
  process.exit(1);
}

function readText(filePath) {
  try {
    return fs.readFileSync(filePath, "utf8");
  } catch (error) {
    fail(`cannot read file: ${filePath}\n${String(error)}`);
  }
}

function parseOpenApiPaths(content) {
  const lines = content.split(/\r?\n/);
  const result = new Set();
  let inPaths = false;

  for (const line of lines) {
    if (!inPaths && /^paths:\s*$/.test(line)) {
      inPaths = true;
      continue;
    }
    if (!inPaths) {
      continue;
    }
    if (/^components:\s*$/.test(line) || /^[A-Za-z0-9_-]+:\s*$/.test(line)) {
      break;
    }
    const match = line.match(/^  (\/[^:#\s][^:]*):\s*$/);
    if (match) {
      result.add(match[1]);
    }
  }
  return result;
}

function parseBackendRegisteredPaths(content) {
  const paths = new Set();
  const matcher = /mux\.Handle\(\s*"([^"]+)"\s*,/g;
  let match = matcher.exec(content);
  while (match) {
    const route = match[1].trim();
    if (route.startsWith("/api/")) {
      paths.add(route);
    }
    match = matcher.exec(content);
  }
  return paths;
}

function hasCoverageInOpenApi(route, openApiPaths) {
  if (route.endsWith("/")) {
    for (const p of openApiPaths) {
      if (p.startsWith(route)) {
        return true;
      }
    }
    return false;
  }
  return openApiPaths.has(route);
}

function hasCoverageInBackend(openApiPath, backendRoutes) {
  if (backendRoutes.has(openApiPath)) {
    return true;
  }
  for (const route of backendRoutes) {
    if (route.endsWith("/") && openApiPath.startsWith(route)) {
      return true;
    }
  }
  return false;
}

const openApiContent = readText(openApiPath);
const serverContent = readText(serverPath);

const openApiPaths = parseOpenApiPaths(openApiContent);
if (openApiPaths.size === 0) {
  fail("no paths parsed from openapi.yaml");
}
const backendRoutes = parseBackendRegisteredPaths(serverContent);
if (backendRoutes.size === 0) {
  fail("no `/api/*` routes parsed from server.go");
}

const backendMissingInOpenApi = [];
for (const route of backendRoutes) {
  if (!hasCoverageInOpenApi(route, openApiPaths)) {
    backendMissingInOpenApi.push(route);
  }
}

const openApiMissingInBackend = [];
for (const apiPath of openApiPaths) {
  if (!apiPath.startsWith("/api/")) {
    continue;
  }
  if (!hasCoverageInBackend(apiPath, backendRoutes)) {
    openApiMissingInBackend.push(apiPath);
  }
}

if (backendMissingInOpenApi.length > 0 || openApiMissingInBackend.length > 0) {
  if (backendMissingInOpenApi.length > 0) {
    console.error("OPENAPI_ROUTE_COVERAGE: backend routes missing in openapi:");
    for (const item of backendMissingInOpenApi) {
      console.error(`  - ${item}`);
    }
  }
  if (openApiMissingInBackend.length > 0) {
    console.error("OPENAPI_ROUTE_COVERAGE: openapi paths missing in backend handlers:");
    for (const item of openApiMissingInBackend) {
      console.error(`  - ${item}`);
    }
  }
  process.exit(1);
}

console.log(
  `OPENAPI_ROUTE_COVERAGE: ok (${backendRoutes.size} backend roots, ${openApiPaths.size} openapi paths)`
);
