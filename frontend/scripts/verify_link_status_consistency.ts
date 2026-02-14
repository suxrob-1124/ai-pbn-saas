import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const queuePage = fs.readFileSync(path.join(process.cwd(), "app", "queue", "page.tsx"), "utf8");
const linkTaskList = fs.readFileSync(path.join(process.cwd(), "components", "LinkTaskList.tsx"), "utf8");

assert.match(
  queuePage,
  /from "\.\.\/\.\.\/lib\/linkTaskStatus"/,
  "queue page must import shared linkTaskStatus helpers"
);
assert.match(
  queuePage,
  /getLinkTaskStatusMeta/,
  "queue page must use getLinkTaskStatusMeta from shared helper"
);
assert.doesNotMatch(
  queuePage,
  /found:\s*\{\s*text:\s*"Найдено"/,
  "queue page must not define legacy found badge mapping"
);

assert.match(
  linkTaskList,
  /from "\.\.\/lib\/linkTaskStatus"/,
  "LinkTaskList must import shared linkTaskStatus helpers"
);
assert.match(
  linkTaskList,
  /normalizeLinkTaskStatus/,
  "LinkTaskList must normalize statuses with shared helper"
);
assert.doesNotMatch(
  linkTaskList,
  /const statusStyles:/,
  "LinkTaskList must not have local status styles map"
);
assert.doesNotMatch(
  linkTaskList,
  /const statusLabels:/,
  "LinkTaskList must not have local status labels map"
);

console.log("verify_link_status_consistency: ok");
