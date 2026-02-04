import assert from "node:assert/strict";

import { apiBase } from "../lib/http";
import { getFile, getFileHistory, listFiles, saveFile } from "../lib/fileApi";

type Call = {
  url: string;
  method: string;
  body?: string;
};

async function main() {
  process.env.NEXT_PUBLIC_API_URL = "http://example.test";

  const base = apiBase();
  const calls: Call[] = [];

  const json = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" }
    });

  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const method = init?.method ? init.method.toUpperCase() : "GET";
    const body = typeof init?.body === "string" ? init.body : undefined;
    calls.push({ url, method, body });

    if (url === `${base}/api/refresh`) {
      return json(200, { status: "ok" });
    }

    if (url === `${base}/api/domains/domain-1/files` && method === "GET") {
      return json(200, [
        {
          id: "file-1",
          path: "index.html",
          size: 120,
          mimeType: "text/html",
          updatedAt: new Date().toISOString()
        }
      ]);
    }

    if (url === `${base}/api/domains/domain-1/files/index.html` && method === "GET") {
      return json(200, { content: "<h1>Hello</h1>", mimeType: "text/html" });
    }

    if (
      url === `${base}/api/domains/domain-1/files/assets/logo%20new.svg` &&
      method === "GET"
    ) {
      return json(200, { content: "<svg></svg>", mimeType: "image/svg+xml" });
    }

    if (url === `${base}/api/domains/domain-1/files/index.html` && method === "PUT") {
      const payload = body ? JSON.parse(body) : null;
      if (payload?.content !== "<h1>Updated</h1>") {
        return json(400, { error: "invalid content" });
      }
      if (payload?.description !== "update heading") {
        return json(400, { error: "invalid description" });
      }
      return json(200, { status: "updated" });
    }

    if (url === `${base}/api/domains/domain-1/files/file-1/history` && method === "GET") {
      return json(200, [
        {
          id: "edit-1",
          editedBy: "editor@example.com",
          editType: "manual",
          description: "initial",
          createdAt: new Date().toISOString()
        }
      ]);
    }

    if (url === `${base}/api/domains/missing/files`) {
      return json(404, { error: "domain not found" });
    }

    return json(500, { error: "unexpected request" });
  };

  const files = await listFiles("domain-1");
  assert.equal(files.length, 1);
  assert.equal(files[0].path, "index.html");

  const file = await getFile("domain-1", "index.html");
  assert.equal(file.mimeType, "text/html");

  const encodedFile = await getFile("domain-1", "assets/logo new.svg");
  assert.equal(encodedFile.mimeType, "image/svg+xml");

  const save = await saveFile(
    "domain-1",
    "index.html",
    "<h1>Updated</h1>",
    "update heading"
  );
  assert.equal(save.status, "updated");

  const history = await getFileHistory("file-1", "domain-1");
  assert.equal(history.length, 1);
  assert.equal(history[0].editedBy, "editor@example.com");

  let missingDomainError = false;
  try {
    await listFiles("missing");
  } catch (err) {
    missingDomainError = true;
    assert.match(String(err), /domain not found/);
  }
  assert.ok(missingDomainError, "expected error for missing domain");

  let missingHistoryDomain = false;
  try {
    await getFileHistory("file-1");
  } catch (err) {
    missingHistoryDomain = true;
    assert.match(String(err), /domainId is required/);
  }
  assert.ok(missingHistoryDomain, "expected error for missing domainId in history");

  const encodedCall = calls.find((call) =>
    call.url.includes("/api/domains/domain-1/files/assets/logo%20new.svg")
  );
  assert.ok(encodedCall, "expected encoded file path call");

  console.log("OK");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
