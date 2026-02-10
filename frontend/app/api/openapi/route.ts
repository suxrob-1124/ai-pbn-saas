import { NextRequest, NextResponse } from "next/server";
import fs from "node:fs/promises";
import path from "node:path";

const apiBase =
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "http://localhost:8080";

export async function GET(request: NextRequest) {
  const cookie = request.headers.get("cookie") || "";
  const res = await fetch(`${apiBase}/api/me`, {
    headers: { cookie },
    credentials: "include"
  });

  if (!res.ok) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const filePath = path.join(process.cwd(), "openapi.yaml");
  const body = await fs.readFile(filePath);
  return new NextResponse(body, {
    headers: {
      "Content-Type": "application/yaml; charset=utf-8",
      "Cache-Control": "no-store"
    }
  });
}
