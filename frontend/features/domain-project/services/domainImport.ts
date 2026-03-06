export type DomainImportPayloadItem = {
  url: string;
  keyword?: string;
  country?: string;
  language?: string;
  server_id?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_placed?: string;
  generation_type?: string;
};

export type DomainImportValidationError = {
  line: number;
  reason: string;
};

const DOMAIN_IMPORT_MAX_COLUMNS = 9;

const parseImportCsvFields = (line: string): string[] | null => {
  const fields: string[] = [];
  let current = "";
  let inQuotes = false;
  for (let idx = 0; idx < line.length; idx += 1) {
    const ch = line[idx];
    if (ch === "\"") {
      if (inQuotes && line[idx + 1] === "\"") {
        current += "\"";
        idx += 1;
      } else {
        inQuotes = !inQuotes;
      }
      continue;
    }
    if (ch === "," && !inQuotes) {
      fields.push(current.trim());
      current = "";
      continue;
    }
    current += ch;
  }
  if (inQuotes) {
    return null;
  }
  fields.push(current.trim());
  if (fields.length > 0) {
    fields[0] = fields[0].replace(/^\uFEFF/, "");
  }
  return fields;
};

const isDomainImportHeader = (fields: string[]) => {
  const first = (fields[0] || "").trim().toLowerCase();
  return first === "url" || first === "domain" || first === "домен";
};

const isValidDomainHost = (host: string) => {
  if (!host || host.length > 253 || !host.includes(".")) {
    return false;
  }
  const labels = host.split(".");
  return labels.every((label) => {
    if (!label || label.length > 63) {
      return false;
    }
    if (label.startsWith("-") || label.endsWith("-")) {
      return false;
    }
    return /^[a-z0-9-]+$/.test(label);
  });
};

export const normalizeDomainForImport = (raw: string): string | null => {
  const input = raw.trim();
  if (!input) {
    return null;
  }
  let candidate = input;
  if (candidate.startsWith("//")) {
    candidate = `https:${candidate}`;
  } else if (!candidate.includes("://")) {
    candidate = `https://${candidate}`;
  }
  try {
    const parsed = new URL(candidate);
    if (parsed.pathname && parsed.pathname !== "/") {
      return null;
    }
    if (parsed.search || parsed.hash || parsed.port || parsed.username || parsed.password) {
      return null;
    }
    const host = parsed.hostname.trim().toLowerCase().replace(/\.$/, "");
    if (!isValidDomainHost(host)) {
      return null;
    }
    return host;
  } catch {
    return null;
  }
};

export const parseDomainImportText = (
  rawText: string
): { items: DomainImportPayloadItem[]; errors: DomainImportValidationError[] } => {
  const items: DomainImportPayloadItem[] = [];
  const errors: DomainImportValidationError[] = [];
  const lines = rawText.split(/\r?\n/);
  for (let lineIndex = 0; lineIndex < lines.length; lineIndex += 1) {
    const lineNo = lineIndex + 1;
    const line = lines[lineIndex].trim();
    if (!line) {
      continue;
    }
    const fields = parseImportCsvFields(line);
    if (!fields) {
      errors.push({ line: lineNo, reason: "незакрытая кавычка" });
      continue;
    }
    if (items.length === 0 && isDomainImportHeader(fields)) {
      continue;
    }
    if (fields.length === 0 || fields.length > DOMAIN_IMPORT_MAX_COLUMNS) {
      errors.push({ line: lineNo, reason: "ожидается до 9 колонок" });
      continue;
    }
    const normalizedDomain = normalizeDomainForImport(fields[0] || "");
    if (!normalizedDomain) {
      errors.push({ line: lineNo, reason: "некорректный домен" });
      continue;
    }
    const item: DomainImportPayloadItem = { url: normalizedDomain };
    const keyword = (fields[1] || "").trim();
    const country = (fields[2] || "").trim();
    const language = (fields[3] || "").trim();
    const serverId = (fields[4] || "").trim();
    const anchor = (fields[5] || "").trim();
    const acceptor = (fields[6] || "").trim();
    if (keyword) {
      item.keyword = keyword;
    }
    if (country) {
      item.country = country;
    }
    if (language) {
      item.language = language;
    }
    if (serverId) {
      item.server_id = serverId;
    }
    if (anchor) {
      item.link_anchor_text = anchor;
    }
    if (acceptor) {
      item.link_acceptor_url = acceptor;
    }
    const linkPlaced = (fields[7] || "").trim();
    if (linkPlaced) {
      item.link_placed = linkPlaced;
    }
    const generationType = (fields[8] || "").trim();
    if (generationType) {
      item.generation_type = generationType;
    }
    items.push(item);
  }
  return { items, errors };
};
